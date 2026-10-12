package localdeps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Clean preserves native data and registered server tools, including nested paths.
func (m Manager) Clean() error {
	paths := []string{m.Dir}
	if m.PGBinDir != "" {
		paths = append(paths, m.PGBinDir)
	}
	state, err := readState(m.Dir)
	if err == nil {
		if state.PGBin != "" {
			paths = append(paths, state.PGBin)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read dependency ownership state; cleaning aborted: %w", err)
	}
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
			fmt.Printf("Preserving %s because it contains native dependency data or tools\n", path)
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
