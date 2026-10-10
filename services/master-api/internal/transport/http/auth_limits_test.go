package http

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginRateLimit(t *testing.T) {
	handler := newTestServer(stubHostStore{}).Handler()
	for i := 0; i < 21; i++ {
		r := httptest.NewRequest("POST", "/master/api/v1/auth/login", strings.NewReader(`{"tenant_code":"default","username":"admin","password":"correct-password"}`))
		r.Header.Set("Origin", "http://example.com")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if i < 20 && w.Code == 429 {
			t.Fatal("normal login attempt prematurely limited")
		}
		if i == 20 && w.Code != 429 {
			t.Fatalf("excess login work accepted: %d", w.Code)
		}
	}
}
