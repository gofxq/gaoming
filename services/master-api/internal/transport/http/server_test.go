package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/repository"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

type stubHostStore struct {
	allocateTenant func(context.Context) (string, error)
}

func (s stubHostStore) AllocateTenant(ctx context.Context) (string, error) {
	if s.allocateTenant != nil {
		return s.allocateTenant(ctx)
	}
	return "", nil
}

func (s stubHostStore) RegisterAgent(context.Context, contracts.RegisterAgentRequest, time.Time) (state.HostSnapshot, contracts.AgentConfig, string, error) {
	return state.HostSnapshot{}, contracts.AgentConfig{}, "", nil
}

func (s stubHostStore) Heartbeat(context.Context, contracts.HeartbeatRequest, time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	return state.HostSnapshot{}, contracts.AgentConfig{}, nil
}

func (s stubHostStore) ListHosts(context.Context, string) ([]state.HostSnapshot, error) {
	return nil, nil
}

func (s stubHostStore) GetHost(context.Context, string, string) (state.HostSnapshot, bool, error) {
	return state.HostSnapshot{}, false, nil
}

func (s stubHostStore) ReconcileOffline(context.Context, time.Time) ([]state.HostSnapshot, error) {
	return nil, nil
}

type stubMetricStore struct{}

func (stubMetricStore) AppendHeartbeatMetrics(context.Context, string, time.Time, contracts.AgentDigest) error {
	return nil
}

func (stubMetricStore) GetHostMetricHistory(context.Context, string) (map[state.MetricKey][]state.MetricPoint, error) {
	return nil, nil
}

func (stubMetricStore) GetAllHostMetricHistory(context.Context, []string) (map[string]map[state.MetricKey][]state.MetricPoint, error) {
	return nil, nil
}

type stubEventBus struct{}

func (stubEventBus) PublishHostUpsert(context.Context, state.HostSnapshot) error {
	return nil
}

func (stubEventBus) PublishHostDelete(context.Context, string) error {
	return nil
}

func (stubEventBus) SubscribeHostEvents(context.Context) (<-chan repository.HostEvent, error) {
	return nil, nil
}

type stubOpsStore struct{}

func (stubOpsStore) CreateMaintenance(context.Context, contracts.CreateMaintenanceWindowRequest) (repository.MaintenanceWindow, error) {
	return repository.MaintenanceWindow{}, nil
}

func (stubOpsStore) AckAlert(context.Context, string, string, time.Time) error {
	return nil
}

func newTestServer(hostStore repository.HostStateStore) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(hostStore, stubMetricStore{}, stubOpsStore{}, stubEventBus{}, clock.Real{}, logger)
	return NewServer(svc)
}

func TestHandleAllocateInstallTenant(t *testing.T) {
	server := newTestServer(stubHostStore{
		allocateTenant: func(context.Context) (string, error) {
			return "tenant-install-test", nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/master/api/v1/install/tenant", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp contracts.AllocateInstallTenantResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TenantCode != "tenant-install-test" {
		t.Fatalf("expected tenant code tenant-install-test, got %q", resp.TenantCode)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request id to be set")
	}
}

func TestHandleAllocateInstallTenantMethodNotAllowed(t *testing.T) {
	server := newTestServer(stubHostStore{})

	req := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/install/tenant", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
}
