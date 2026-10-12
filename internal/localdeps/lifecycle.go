package localdeps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type State struct {
	PGBin         string `json:"pg_bin"`
	PGPort        uint16 `json:"pg_port"`
	RedisPort     int    `json:"redis_port"`
	RedisPassword string `json:"redis_password"`
	Fingerprint   string `json:"fingerprint"`
}

type Manager struct {
	Dir      string
	PGBinDir string
}

func (m Manager) paths() (string, string, string, error) {
	root, err := filepath.Abs(m.Dir)
	return root, filepath.Join(root, "postgres"), filepath.Join(root, "redis.sock"), err
}
func readState(root string) (State, error) {
	var s State
	raw, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s)
	return s, err
}
func writeState(root string, s State) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(root, "state-*.json")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
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
	return os.Rename(temporary, filepath.Join(root, "state.json"))
}
func lock(root string) (func(), error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("DEPS_DIR must be private (mode 0700); fix permissions on %s before managing dependencies", root)
	}
	dir := filepath.Join(root, "operation.lock")
	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, fmt.Errorf("dependency operation locked; after confirming no operation is running, remove %s", dir)
	}
	return func() { _ = os.Remove(dir) }, nil
}
func pgRunning(ctx context.Context, bin, data string) (bool, error) {
	cmd := exec.CommandContext(ctx, filepath.Join(bin, "pg_ctl"), "-D", data, "status")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return false, nil
	}
	return false, fmt.Errorf("cannot inspect managed PostgreSQL: %s (%v)", strings.TrimSpace(string(output)), err)
}
func redisClient(socket, password string) *redis.Client {
	return redis.NewClient(&redis.Options{Network: "unix", Addr: socket, Password: password, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
}

func (m Manager) Up(ctx context.Context, cfg Config) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("run deps-up as a regular user, without sudo")
	}
	root, data, socket, err := m.paths()
	if err != nil {
		return err
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := readState(root)
	fresh := os.IsNotExist(err)
	if err != nil && !fresh {
		return fmt.Errorf("read managed dependency state: %w", err)
	}
	if !fresh && state.Fingerprint != cfg.fingerprint {
		return fmt.Errorf("database or credentials differ from initialized dependencies; restore the original credentials or choose a new DEPS_DIR; existing data is preserved")
	}
	lookup := m.PGBinDir
	if !fresh && lookup == "" {
		lookup = state.PGBin
	}
	bin, err := pgTools(lookup)
	if err != nil {
		return err
	}
	redisBin, err := exec.LookPath("redis-server")
	if err != nil {
		return fmt.Errorf("redis-server not found; install Redis before deps-up")
	}
	pgActive := false
	if !fresh {
		pgActive, err = pgRunning(ctx, bin, data)
		if err != nil {
			return err
		}
	}
	client := redisClient(socket, cfg.RedisPassword)
	defer client.Close()
	redisActive := socketExists(socket) && client.Ping(ctx).Err() == nil
	if (pgActive && state.PGPort != cfg.PG.Port) || (redisActive && state.RedisPort != cfg.RedisPort) {
		return fmt.Errorf("managed ports changed; run make deps-down before restarting")
	}
	if !pgActive {
		if err := available(int(cfg.PG.Port)); err != nil {
			return err
		}
	}
	if !redisActive {
		if err := available(cfg.RedisPort); err != nil {
			return err
		}
	}
	if cfg.RedisPort == int(cfg.PG.Port) {
		return fmt.Errorf("PostgreSQL and Redis cannot share the same port")
	}
	if fresh {
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			return fmt.Errorf("unmanaged Redis socket exists; choose a different DEPS_DIR")
		}
		if _, err := os.Stat(data); !os.IsNotExist(err) {
			return fmt.Errorf("unmanaged PostgreSQL data directory exists; choose a different DEPS_DIR")
		}
		pw, err := os.CreateTemp(root, "postgres-password-")
		if err != nil {
			return err
		}
		pwPath := pw.Name()
		defer os.Remove(pwPath)
		if _, err = pw.WriteString(cfg.PG.Password + "\n"); err != nil {
			pw.Close()
			return err
		}
		if err = pw.Close(); err != nil {
			return err
		}
		if err := run(ctx, filepath.Join(bin, "initdb"), "-D", data, "-U", cfg.PG.User, "--auth=scram-sha-256", "--pwfile="+pwPath); err != nil {
			return err
		}
		state = State{PGBin: bin, Fingerprint: cfg.fingerprint, RedisPassword: cfg.RedisPassword}
	}
	state.PGBin = bin
	state.PGPort = cfg.PG.Port
	state.RedisPort = cfg.RedisPort
	if err := writeState(root, state); err != nil {
		return err
	}
	startedPG, startedRedis := false, false
	success := false
	defer func() {
		if !success {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if startedRedis {
				_ = client.Shutdown(cleanupCtx).Err()
			}
			if startedPG {
				_ = run(cleanupCtx, filepath.Join(bin, "pg_ctl"), "-D", data, "-m", "fast", "-w", "stop")
			}
		}
	}()
	if !pgActive {
		conf, err := os.OpenFile(filepath.Join(data, "postgresql.conf"), os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		// PostgreSQL config uses SQL-style quoting; no paths enter a shell command.
		socketDir := strings.ReplaceAll(root, "'", "''")
		_, err = fmt.Fprintf(conf, "\nlisten_addresses = '*'\nport = %d\nunix_socket_directories = '%s'\n", cfg.PG.Port, socketDir)
		closeErr := conf.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		startedPG = true
		if err := run(ctx, filepath.Join(bin, "pg_ctl"), "-D", data, "-l", filepath.Join(root, "postgres.log"), "-w", "start"); err != nil {
			return err
		}
	}
	adminConfig := cfg.PG.Copy()
	adminConfig.Database = "postgres"
	conn, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		return fmt.Errorf("connect managed PostgreSQL failed; check the configured credentials")
	}
	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname=$1)", cfg.PG.Database).Scan(&exists)
	if err == nil && !exists {
		_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{cfg.PG.Database}.Sanitize())
	}
	_ = conn.Close(ctx)
	if err != nil {
		return fmt.Errorf("create development database: %w", err)
	}
	if !redisActive {
		config := fmt.Sprintf("bind 0.0.0.0 -::\nprotected-mode yes\nport %d\nrequirepass %s\ndaemonize yes\npidfile %s\nlogfile %s\ndir %s\nunixsocket %s\nunixsocketperm 700\nsave \"\"\nappendonly no\n", cfg.RedisPort, strconv.Quote(cfg.RedisPassword), strconv.Quote(filepath.Join(root, "redis.pid")), strconv.Quote(filepath.Join(root, "redis.log")), strconv.Quote(root), strconv.Quote(socket))
		configPath := filepath.Join(root, "redis.conf")
		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			return err
		}
		startedRedis = true
		if err := run(ctx, redisBin, configPath); err != nil {
			return fmt.Errorf("Redis failed to start; inspect %s", filepath.Join(root, "redis.log"))
		}
		deadline := time.Now().Add(5 * time.Second)
		for !socketExists(socket) || client.Ping(ctx).Err() != nil {
			if ctx.Err() != nil || time.Now().After(deadline) {
				return fmt.Errorf("managed Redis did not become ready")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	success = true
	fmt.Printf("Native PostgreSQL :%d and Redis :%d ready; data: %s\n", cfg.PG.Port, cfg.RedisPort, root)
	return nil
}

func (m Manager) Down(ctx context.Context) error {
	root, data, socket, err := m.paths()
	if err != nil {
		return err
	}
	state, err := readState(root)
	if os.IsNotExist(err) {
		fmt.Println("No managed local dependencies to stop.")
		return nil
	}
	if err != nil {
		return err
	}
	unlock, err := lock(root)
	if err != nil {
		return err
	}
	defer unlock()
	var failures []error
	// Control Redis only through this project's private Unix socket.
	client := redisClient(socket, state.RedisPassword)
	defer client.Close()
	if socketExists(socket) && client.Ping(ctx).Err() == nil {
		if err := client.Shutdown(ctx).Err(); err != nil && !errors.Is(err, redis.Nil) {
			failures = append(failures, fmt.Errorf("stop managed Redis: %w", err))
		}
	} else if connection, err := net.DialTimeout("unix", socket, time.Second); err == nil {
		connection.Close()
		failures = append(failures, fmt.Errorf("managed Redis is reachable but authentication failed; dependencies may still be running"))
	}
	active, statusErr := pgRunning(ctx, state.PGBin, data)
	if statusErr != nil {
		failures = append(failures, statusErr)
	} else if active {
		if err := run(ctx, filepath.Join(state.PGBin, "pg_ctl"), "-D", data, "-m", "fast", "-w", "stop"); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	fmt.Printf("Managed native dependencies stopped; data retained: %s\n", root)
	return nil
}

func socketExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}
