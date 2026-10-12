package http

import (
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"github.com/gofxq/gaoming/server/internal/master/service"
)

type stubHostStore struct {
	allocateTenant func(context.Context) (string, error)
	listHosts      func(context.Context, string) ([]state.HostSnapshot, error)
	getHost        func(context.Context, string, string) (state.HostSnapshot, bool, error)
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

func (s stubHostStore) ReportMetrics(context.Context, contracts.PushMetricBatchRequest, contracts.AgentDigest, time.Time) (state.HostSnapshot, error) {
	return state.HostSnapshot{}, nil
}

func (s stubHostStore) ListHosts(ctx context.Context, tenantCode string) ([]state.HostSnapshot, error) {
	if s.listHosts != nil {
		return s.listHosts(ctx, tenantCode)
	}
	return nil, nil
}

func (s stubHostStore) GetHost(ctx context.Context, hostUID string, tenantCode string) (state.HostSnapshot, bool, error) {
	if s.getHost != nil {
		return s.getHost(ctx, hostUID, tenantCode)
	}
	return state.HostSnapshot{}, false, nil
}

func (s stubHostStore) ReconcileOffline(context.Context, time.Time) ([]state.HostSnapshot, error) {
	return nil, nil
}

type stubMetricStore struct {
	getMetricSnapshot func(context.Context, string) (contracts.MetricSnapshot, bool, error)
}

func (stubMetricStore) AppendHeartbeatMetrics(context.Context, string, time.Time, contracts.AgentDigest) error {
	return nil
}

func (stubMetricStore) SaveMetricSnapshot(context.Context, string, contracts.MetricSnapshot) error {
	return nil
}

func (s stubMetricStore) GetMetricSnapshot(ctx context.Context, hostUID string) (contracts.MetricSnapshot, bool, error) {
	if s.getMetricSnapshot != nil {
		return s.getMetricSnapshot(ctx, hostUID)
	}
	return contracts.MetricSnapshot{}, false, nil
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

func (stubOpsStore) CreateMaintenance(context.Context, access.Principal, contracts.CreateMaintenanceWindowRequest) (repository.MaintenanceWindow, error) {
	return repository.MaintenanceWindow{}, nil
}

func (stubOpsStore) AckAlert(context.Context, access.Principal, string, time.Time) error {
	return nil
}

func newTestServer(hostStore repository.HostStateStore) *Server {
	return newTestServerWithMetricStore(hostStore, stubMetricStore{})
}

func newTestServerWithMetricStore(hostStore repository.HostStateStore, metricStore repository.MetricWindowStore) *Server {
	_ = io.Discard
	logger := logx.NewNop()
	svc := service.New(hostStore, metricStore, stubOpsStore{}, stubEventBus{}, nil, service.AuthConfig{}, clock.Real{}, logger)
	return NewServer(svc, logger)
}

type testSessionStore struct {
	auth.Store
	tenant string
}

func (s testSessionStore) GetUserBySessionTokenHash(context.Context, string, time.Time) (auth.User, auth.SessionRecord, bool, error) {
	return auth.User{ID: 1, TenantCode: s.tenant, Role: auth.UserRoleAdmin, Status: auth.UserStatusActive}, auth.SessionRecord{}, true, nil
}
func newAuthenticatedTestServer(hostStore repository.HostStateStore, metricStore repository.MetricWindowStore, tenant string) *Server {
	logger := logx.NewNop()
	svc := service.New(hostStore, metricStore, stubOpsStore{}, stubEventBus{}, testSessionStore{tenant: tenant}, service.AuthConfig{}, clock.Real{}, logger)
	return NewServer(svc, logger)
}

func TestHandleGetLatestMetricSnapshotChecksTenantAndReturnsSnapshot(t *testing.T) {
	collectedAt := time.Now().UTC().Truncate(time.Second)
	server := newAuthenticatedTestServer(stubHostStore{
		getHost: func(_ context.Context, hostUID string, tenantCode string) (state.HostSnapshot, bool, error) {
			if hostUID != "host-1" || tenantCode != "tenant-a" {
				t.Fatalf("unexpected host lookup: host=%q tenant=%q", hostUID, tenantCode)
			}
			return state.HostSnapshot{HostUID: hostUID, TenantCode: tenantCode}, true, nil
		},
	}, stubMetricStore{
		getMetricSnapshot: func(_ context.Context, hostUID string) (contracts.MetricSnapshot, bool, error) {
			if hostUID != "host-1" {
				t.Fatalf("unexpected snapshot host uid: %q", hostUID)
			}
			return contracts.MetricSnapshot{
				CollectedAt: collectedAt,
				Points: []contracts.MetricPoint{
					{Name: "host_load5", Value: 1.2},
				},
			}, true, nil
		},
	}, "tenant-a")

	req := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/hosts/host-1/metrics/latest?tenant=tenant-a", nil)
	req.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: "test-session"})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache policy, got %q", rec.Header().Get("Cache-Control"))
	}
	var resp struct {
		Snapshot contracts.MetricSnapshot `json:"snapshot"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Snapshot.CollectedAt.Equal(collectedAt) || len(resp.Snapshot.Points) != 1 {
		t.Fatalf("unexpected metric snapshot: %+v", resp.Snapshot)
	}
}

func TestHandleGetLatestMetricSnapshotRejectsOtherTenant(t *testing.T) {
	metricStoreCalled := false
	server := newAuthenticatedTestServer(stubHostStore{
		getHost: func(_ context.Context, _ string, tenantCode string) (state.HostSnapshot, bool, error) {
			if tenantCode != "tenant-b" {
				t.Fatalf("unexpected tenant lookup: %q", tenantCode)
			}
			return state.HostSnapshot{}, false, nil
		},
	}, stubMetricStore{
		getMetricSnapshot: func(context.Context, string) (contracts.MetricSnapshot, bool, error) {
			metricStoreCalled = true
			return contracts.MetricSnapshot{}, false, nil
		},
	}, "tenant-b")

	req := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/hosts/host-1/metrics/latest?tenant=tenant-b", nil)
	req.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: "test-session"})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if metricStoreCalled {
		t.Fatal("metric store must not be read before tenant ownership is verified")
	}
}

func TestHandleGetLatestMetricSnapshotRequiresTenant(t *testing.T) {
	hostStoreCalled := false
	server := newTestServer(stubHostStore{
		getHost: func(context.Context, string, string) (state.HostSnapshot, bool, error) {
			hostStoreCalled = true
			return state.HostSnapshot{}, false, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/hosts/host-1/metrics/latest", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	if hostStoreCalled {
		t.Fatal("host store must not be read without a tenant")
	}
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

func TestHandleListHostsUsesAuthenticatedTenant(t *testing.T) {
	var gotTenantCode string
	server := newAuthenticatedTestServer(stubHostStore{
		listHosts: func(_ context.Context, tenantCode string) ([]state.HostSnapshot, error) {
			gotTenantCode = tenantCode
			return []state.HostSnapshot{
				{
					HostUID:    "host-1",
					TenantCode: "tenant-a",
					Hostname:   "agent-1",
				},
			}, nil
		},
	}, stubMetricStore{}, "tenant-a")

	req := httptest.NewRequest(nethttp.MethodGet, "/master/api/v1/hosts", nil)
	req.AddCookie(&nethttp.Cookie{Name: "gaoming_session", Value: "test-session"})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if gotTenantCode != "tenant-a" {
		t.Fatalf("expected tenant-a, got %q", gotTenantCode)
	}

	var resp struct {
		Items []state.HostSnapshot `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].HostUID != "host-1" {
		t.Fatalf("unexpected items: %+v", resp.Items)
	}
}
