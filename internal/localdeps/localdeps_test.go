package localdeps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitConfigPreservesExistingAndGeneratesSecrets(t *testing.T) {
	template := filepath.Join(t.TempDir(), "template.yml")
	if err := os.WriteFile(template, []byte("postgres_dsn: postgres://gaoming:gaoming@127.0.0.1:5432/gaoming?sslmode=disable\nredis_addr: 127.0.0.1:6379\nredis_password: ''\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config", "server.yml")
	if err := InitConfig(path, template); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PG.Password) < 24 || len(cfg.RedisPassword) < 24 || cfg.PG.Password == cfg.RedisPassword {
		t.Fatal("credentials were not independently generated")
	}
	if mode, err := os.Stat(path); err != nil || mode.Mode().Perm()&0077 != 0 {
		t.Fatal("credentials file must be private")
	}
	if err := InitConfig(path, "missing-template"); err != nil {
		t.Fatal("existing file should be kept", err)
	}
	after, _ := os.ReadFile(path)
	if string(raw) != string(after) {
		t.Fatal("existing config overwritten")
	}
}

func TestRejectsRemoteAndUnprotectedDependencies(t *testing.T) {
	for _, body := range []string{
		"postgres_dsn: postgres://u:p@remote.example:5432/db\nredis_addr: 127.0.0.1:6379\nredis_password: safe\n",
		"postgres_dsn: postgres://u:p@127.0.0.1:5432/db\nredis_addr: remote.example:6379\nredis_password: safe\n",
		"postgres_dsn: postgres://u:p@127.0.0.1:5432/db\nredis_addr: 127.0.0.1:6379\nredis_password: ''\n",
		"postgres_dsn: postgres://u@127.0.0.1:5432/db\nredis_addr: 127.0.0.1:6379\nredis_password: safe\n",
	} {
		t.Run(strings.Split(body, "\n")[0], func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "server.yml")
			os.WriteFile(p, []byte(body), 0600)
			if _, err := LoadConfig(p); err == nil {
				t.Fatal("unsafe dependency configuration accepted")
			}
		})
	}
}

func TestCleanPreservesDatabaseInsideTemporaryFolder(t *testing.T) {
	t.Chdir(t.TempDir())
	data := filepath.Join(".tmp", "owned-db")
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "retained-data"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll("bin", 0755)
	os.WriteFile("bin/build", []byte("generated"), 0600)
	if err := (Manager{Dir: data}).Clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "retained-data")); err != nil {
		t.Fatal("database data deleted", err)
	}
	if _, err := os.Stat("bin/build"); !os.IsNotExist(err) {
		t.Fatal("build artifact retained")
	}
}

func TestStateUpdateFailurePreservesOwnership(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires regular user permission enforcement")
	}
	root := t.TempDir()
	original := State{PGBin: "/owned/postgres/bin", PGPort: 5432, RedisPort: 6379, Fingerprint: "original"}
	if err := writeState(root, original); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0700) })
	if err := writeState(root, State{Fingerprint: "replacement"}); err == nil {
		t.Fatal("state update should fail when replacement cannot be created")
	}
	after, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("ownership metadata changed after failed update")
	}
}

func TestCleanPreservesRegisteredPostgresTools(t *testing.T) {
	t.Chdir(t.TempDir())
	tools := filepath.Join(".tmp", "postgres-tools", "bin")
	if err := os.MkdirAll(tools, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "pg_ctl"), []byte("native tool"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".local-data", 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeState(".local-data", State{PGBin: tools}); err != nil {
		t.Fatal(err)
	}
	if err := (Manager{Dir: ".local-data"}).Clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tools, "pg_ctl")); err != nil {
		t.Fatal("native stop executable was deleted", err)
	}
}

func TestRejectsSharedDependencyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0777); err != nil {
		t.Fatal(err)
	}
	unlock, err := lock(root)
	if err == nil {
		unlock()
		t.Fatal("shared writable directory accepted for private dependency state")
	}
}

func TestDependencySetupDoesNotReuseAmbientDatabasePassword(t *testing.T) {
	t.Setenv("PGPASSWORD", "unrelated-ambient-secret")
	path := filepath.Join(t.TempDir(), "server.yml")
	if err := os.WriteFile(path, []byte("postgres_dsn: postgres://gaoming@127.0.0.1:5432/gaoming?sslmode=disable\nredis_addr: 127.0.0.1:6379\nredis_password: explicit-redis-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("new native cluster would reuse an ambient credential")
	}
}
