package http

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/state"
)

func TestPrivateReadsRejectMissingOrForgedTenant(t *testing.T) {
	for _, path := range []string{
		"/master/api/v1/hosts",
		"/master/api/v1/hosts?tenant=tenant-a",
		"/master/api/v1/hosts/host-1?tenant=tenant-a",
		"/master/api/v1/hosts/host-1/metrics/latest?tenant=tenant-a",
		"/master/api/v1/stream/hosts?tenant=tenant-a",
	} {
		t.Run(path, func(t *testing.T) {
			server := newTestServer(stubHostStore{listHosts: func(context.Context, string) ([]state.HostSnapshot, error) {
				return []state.HostSnapshot{{HostUID: "private-host", TenantCode: "tenant-a"}}, nil
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			req := httptest.NewRequest("GET", path, nil).WithContext(ctx)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, req)
			if response.Code != 401 {
				t.Fatalf("anonymous private read accepted: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAuthenticatedReadsRejectForgedTenant(t *testing.T) {
	for _, path := range []string{"/hosts?tenant=tenant-b", "/hosts/host-1?tenant=tenant-b", "/hosts/host-1/metrics/latest?tenant=tenant-b", "/stream/hosts?tenant=tenant-b"} {
		server := newAuthenticatedTestServer(stubHostStore{listHosts: func(context.Context, string) ([]state.HostSnapshot, error) {
			t.Fatal("forged tenant reached storage")
			return nil, nil
		}}, stubMetricStore{}, "tenant-a")
		req := httptest.NewRequest("GET", "/master/api/v1"+path, nil)
		req.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: "test-session"})
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatalf("forged tenant accepted: %s %d", path, w.Code)
		}
	}
}
