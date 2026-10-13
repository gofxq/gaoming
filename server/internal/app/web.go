package app

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
)

// newWebHandler confines all file access (including symlinks) to a built Web directory.
func newWebHandler(dir string) (http.Handler, *os.Root, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("open Web directory: %w", err)
	}
	index, err := root.Stat("index.html")
	if err != nil || !index.Mode().IsRegular() {
		root.Close()
		return nil, nil, fmt.Errorf("Web directory must contain a built index.html")
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		for _, segment := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(segment, ".") {
				http.NotFound(w, r)
				return
			}
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		file, err := root.Open(name)
		if err == nil {
			info, statErr := file.Stat()
			if statErr == nil && info.Mode().IsRegular() {
				defer file.Close()
				if name == "index.html" {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeContent(w, r, name, info.ModTime(), file)
				return
			}
			file.Close()
		}
		// Browser routes receive the SPA shell; missing assets never become HTML.
		if path.Ext(name) != "" || name == "assets" || strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		file, err = root.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", index.ModTime(), file)
	})
	return handler, root, nil
}
