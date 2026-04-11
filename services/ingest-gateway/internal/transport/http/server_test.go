package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
)

func TestMetricsEndpointRemoved(t *testing.T) {
	logger := logx.NewNop()
	svc := service.New(logger, clock.Real{}, nil, nil, nil)
	handler := NewServer(svc, logger).Handler()

	req := httptest.NewRequest(nethttp.MethodPost, "/ingest/api/v1/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected metrics endpoint to be removed, got status %d", rec.Code)
	}
}

func TestInstallTenantEndpointRemoved(t *testing.T) {
	logger := logx.NewNop()
	svc := service.New(logger, clock.Real{}, nil, nil, nil)
	handler := NewServer(svc, logger).Handler()

	req := httptest.NewRequest(nethttp.MethodPost, "/ingest/api/v1/install/tenant", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected install tenant endpoint to be removed, got status %d", rec.Code)
	}
}
