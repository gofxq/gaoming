package localdeps

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type DockerState struct {
	Project     string `json:"project"`
	Fingerprint string `json:"fingerprint"`
}

func dockerProject(root string) string {
	return fmt.Sprintf("gaoming-deps-%x", sha256.Sum256([]byte(root)))[:24]
}

func prepareDocker(root string, cfg Config) error {
	if cfg.PG.Host == "::1" || cfg.RedisHost == "::1" {
		return fmt.Errorf("Docker dependency ports publish on IPv4; use 127.0.0.1 in postgres_dsn and redis_addr")
	}
	if int(cfg.PG.Port) == cfg.RedisPort {
		return fmt.Errorf("PostgreSQL and Redis need different host ports")
	}
	for name, value := range map[string]string{"postgres-password": cfg.PG.Password, "redis-password": cfg.RedisPassword, "redis.conf": "bind 0.0.0.0\nprotected-mode yes\nport 6379\nrequirepass " + strconv.Quote(cfg.RedisPassword) + "\nsave \"\"\nappendonly no\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			return err
		}
	}
	health := func(test []string) map[string]any {
		return map[string]any{"test": test, "interval": "2s", "timeout": "3s", "retries": 30}
	}
	// Secrets are file mounts: passwords never become Compose environment/CLI values.
	manifest := map[string]any{
		"services": map[string]any{
			"postgres": map[string]any{"image": "postgres:16-alpine", "restart": "unless-stopped", "environment": map[string]string{"POSTGRES_USER": cfg.PG.User, "POSTGRES_DB": cfg.PG.Database, "POSTGRES_PASSWORD_FILE": "/run/secrets/postgres_password"}, "secrets": []string{"postgres_password"}, "ports": []string{fmt.Sprintf("0.0.0.0:%d:5432", cfg.PG.Port)}, "volumes": []string{"postgres_data:/var/lib/postgresql/data"}, "healthcheck": health([]string{"CMD", "pg_isready", "-U", cfg.PG.User, "-d", cfg.PG.Database})},
			"redis":    map[string]any{"image": "redis:7-alpine", "restart": "unless-stopped", "secrets": []string{"redis_password", "redis_config"}, "ports": []string{fmt.Sprintf("0.0.0.0:%d:6379", cfg.RedisPort)}, "entrypoint": []string{"sh", "-ec", "cp /run/secrets/redis_config /tmp/gaoming-redis.conf; chown redis:redis /tmp/gaoming-redis.conf; chmod 600 /tmp/gaoming-redis.conf; exec /usr/local/bin/docker-entrypoint.sh redis-server /tmp/gaoming-redis.conf"}, "healthcheck": health([]string{"CMD-SHELL", "REDISCLI_AUTH=\"$$(cat /run/secrets/redis_password)\" redis-cli ping"})},
		},
		"volumes": map[string]any{"postgres_data": map[string]any{}},
		"secrets": map[string]any{"postgres_password": map[string]string{"file": filepath.Join(root, "postgres-password")}, "redis_password": map[string]string{"file": filepath.Join(root, "redis-password")}, "redis_config": map[string]string{"file": filepath.Join(root, "redis.conf")}},
	}
	raw, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "compose.deps.yml"), raw, 0600)
}

func composeCommand(ctx context.Context, command, root, project string, args ...string) *exec.Cmd {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		parts = []string{"docker", "compose"}
	}
	prefix := append(parts[1:], "-p", project, "-f", filepath.Join(root, "compose.deps.yml"))
	return exec.CommandContext(ctx, parts[0], append(prefix, args...)...)
}

func saveDockerState(root string, state DockerState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, "docker-state-*.json")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(root, "docker-state.json"))
}

func (m Manager) DockerUp(ctx context.Context, cfg Config, command string) error {
	root, _, _, err := m.paths()
	if err != nil {
		return err
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	state := DockerState{Project: dockerProject(root), Fingerprint: cfg.fingerprint}
	if raw, err := os.ReadFile(filepath.Join(root, "docker-state.json")); err == nil {
		var old DockerState
		if err := json.Unmarshal(raw, &old); err != nil {
			return fmt.Errorf("invalid Docker dependency ownership state")
		}
		if old.Project != state.Project || old.Fingerprint != state.Fingerprint {
			return fmt.Errorf("dependency credentials differ from initialized Docker data; restore the original configuration or use a new DEPS_DIR; existing volume is preserved")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := prepareDocker(root, cfg); err != nil {
		return err
	}
	// Record ownership before starting containers, so a failed startup can be stopped.
	if err := saveDockerState(root, state); err != nil {
		return err
	}
	cmd := composeCommand(ctx, command, root, state.Project, "up", "-d", "--wait", "--wait-timeout", "90", "postgres", "redis")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker dependencies failed to start; check container logs and configured ports, then retry (data volume preserved)")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(verifyCtx, cfg.PG.Copy())
	if err != nil {
		return fmt.Errorf("PostgreSQL credentials do not match the existing Docker database; restore the original configuration (do not delete the volume)")
	}
	defer conn.Close(context.Background())
	if err := conn.Ping(verifyCtx); err != nil {
		return fmt.Errorf("PostgreSQL did not become reachable from the host")
	}
	fmt.Printf("Docker PostgreSQL :%d and Redis :%d ready; project: %s\n", cfg.PG.Port, cfg.RedisPort, state.Project)
	return nil
}

func (m Manager) DockerDown(ctx context.Context, command string) error {
	root, _, _, err := m.paths()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(root, "docker-state.json"))
	if os.IsNotExist(err) {
		fmt.Println("No managed Docker dependencies to stop.")
		return nil
	}
	if err != nil {
		return err
	}
	var state DockerState
	if err := json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if state.Project != dockerProject(root) {
		return fmt.Errorf("Docker project does not match DEPS_DIR; refusing to stop a different project")
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	cmd := composeCommand(ctx, command, root, state.Project, "down", "--remove-orphans")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop Docker dependencies failed")
	}
	fmt.Println("Docker dependencies stopped; PostgreSQL volume retained.")
	return nil
}
