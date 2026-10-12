package localdeps

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func pgTools(override string) (string, error) {
	candidates := []string{}
	if override != "" {
		candidates = append(candidates, override)
	} else {
		if bin, err := exec.LookPath("pg_ctl"); err == nil {
			candidates = append(candidates, filepath.Dir(bin))
		}
		if raw, err := exec.Command("pg_config", "--bindir").Output(); err == nil {
			candidates = append(candidates, strings.TrimSpace(string(raw)))
		}
		for _, pattern := range []string{"/usr/lib/postgresql/*/bin", "/opt/homebrew/opt/postgresql*/bin", "/usr/local/opt/postgresql*/bin"} {
			paths, _ := filepath.Glob(pattern)
			sort.Sort(sort.Reverse(sort.StringSlice(paths)))
			candidates = append(candidates, paths...)
		}
	}
	for _, dir := range candidates {
		valid := true
		for _, name := range []string{"pg_ctl", "initdb", "postgres"} {
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
				valid = false
				break
			}
		}
		if valid {
			return filepath.Abs(dir)
		}
	}
	return "", fmt.Errorf("PostgreSQL server binaries not found; install PostgreSQL and add its bin directory to PATH, or set PG_BIN_DIR (psql alone is insufficient)")
}

func run(ctx context.Context, command string, args ...string) error {
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", filepath.Base(command), strings.TrimSpace(string(output)))
	}
	return nil
}

func available(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is occupied; change config/server.yml or stop its current owner", port)
	}
	return listener.Close()
}
