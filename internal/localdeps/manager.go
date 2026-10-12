package localdeps

import (
	"fmt"
	"os"
	"path/filepath"
)

type Manager struct{ Dir string }

func (m Manager) root() (string, error) { return filepath.Abs(m.Dir) }

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
