package localdeps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDockerComposeKeepsCredentialsOutOfManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.yml")
	if err := os.WriteFile(path, []byte("postgres_dsn: postgres://gaoming:explicit-pg-password@127.0.0.1:25432/gaoming?sslmode=disable\nredis_addr: 127.0.0.1:26379\nredis_password: explicit-redis-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareDocker(root, cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "compose.deps.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), cfg.PG.Password) || strings.Contains(string(raw), cfg.RedisPassword) {
		t.Fatal("manifest exposes a password")
	}
	var manifest struct {
		Services map[string]struct {
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Services) != 2 || manifest.Services["postgres"].Ports[0] != "0.0.0.0:25432:5432" || manifest.Services["redis"].Ports[0] != "0.0.0.0:26379:6379" {
		t.Fatal("unexpected dependency services or host ports")
	}
	for _, name := range []string{"postgres-password", "redis-password", "redis.conf"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatal("secret file must be private")
		}
	}
}

func TestDockerOwnershipStateContainsNoPassword(t *testing.T) {
	cfg := Config{fingerprint: "fingerprint"}
	state := DockerState{Project: dockerProject("/project/.local-data"), Fingerprint: cfg.fingerprint}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(state.Project, "gaoming-deps-") {
		t.Fatal("project namespace missing")
	}
	if state.Project == dockerProject("/other/.local-data") {
		t.Fatal("projects share namespace")
	}
	if strings.Contains(string(raw), "password") {
		t.Fatal("ownership state should not store a credential")
	}
}

func TestDockerDependenciesRejectIPv6OnlyHost(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server.yml")
	for _, body := range []string{
		"postgres_dsn: postgres://gaoming:explicit-pg-password@[::1]:25432/gaoming?sslmode=disable\nredis_addr: 127.0.0.1:26379\nredis_password: explicit-redis-password\n",
		"postgres_dsn: postgres://gaoming:explicit-pg-password@127.0.0.1:25432/gaoming?sslmode=disable\nredis_addr: '[::1]:26379'\nredis_password: explicit-redis-password\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := prepareDocker(root, cfg); err == nil {
			t.Fatal("IPv6-only host accepted for IPv4 Docker publication")
		}
	}
}
