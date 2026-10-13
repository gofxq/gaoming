package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebHandlerRoutesAndBoundaries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<html>gaoming-app</html>", "assets/app.js": "console.log('app')", ".env": "private-token"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("outside-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	handler, root, err := newWebHandler(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, tc := range []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/", 200, "gaoming-app"}, {"GET", "/default/login", 200, "gaoming-app"},
		{"GET", "/default/setup", 200, "gaoming-app"}, {"HEAD", "/default", 200, ""},
		{"GET", "/assets/app.js", 200, "console.log"}, {"GET", "/assets/missing.js", 404, ""},
		{"GET", "/assets/missing", 404, ""}, {"GET", "/.env", 404, ""},
		{"GET", "/../secret.txt", 404, ""}, {"GET", "/escape.txt", 404, ""}, {"POST", "/default", 405, ""},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d", w.Code, tc.status)
			}
			if tc.body != "" && !strings.Contains(w.Body.String(), tc.body) {
				t.Fatal("wrong response body")
			}
			if strings.Contains(w.Body.String(), "outside-secret") || strings.Contains(w.Body.String(), "private-token") {
				t.Fatal("private file exposed")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
		})
	}
	mux := http.NewServeMux()
	mux.Handle("/master/", http.HandlerFunc(http.NotFound))
	mux.Handle("/ingest/", http.HandlerFunc(http.NotFound))
	mux.Handle("/", handler)
	for _, api := range []string{"/master/api/v1/missing", "/ingest/missing"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", api, nil))
		if w.Code != 404 || strings.Contains(w.Body.String(), "gaoming-app") {
			t.Fatal("SPA fallback intercepted API")
		}
	}
}

func TestWebHandlerRequiresBuiltIndex(t *testing.T) {
	_, root, err := newWebHandler(t.TempDir())
	if root != nil {
		root.Close()
	}
	if err == nil {
		t.Fatal("missing index.html accepted")
	}
}
