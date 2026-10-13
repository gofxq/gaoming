package localdeps

import (
	"os"
	"path/filepath"
	"strings"
)

// Clean preserves the private dependency directory, including nested paths.
func (m Manager) Clean() error {
	paths := []string{m.Dir}
	protected := make([]string, 0, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if real, err := filepath.EvalSymlinks(absolute); err == nil {
			absolute = real
		} else if !os.IsNotExist(err) {
			return err
		}
		protected = append(protected, absolute)
	}
	for _, path := range []string{"bin", "dist", "tmp", ".tmp", "coverage", "cover.out", "web/dist"} {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		keep := false
		for _, data := range protected {
			relative, err := filepath.Rel(absolute, data)
			if err != nil {
				return err
			}
			if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
				keep = true
				break
			}
		}
		if keep {
			// Keep the enclosing directory intact so private state is never removed.
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
