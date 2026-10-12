package grpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/server/internal/ingest/service"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPushMetricBatch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	collectedAt := time.Now().UTC().Truncate(time.Second)
	var savedSnapshot contracts.MetricSnapshot
	metricStore := stubMetricStore{
		saveMetricSnapshot: func(_ context.Context, hostUID string, snapshot contracts.MetricSnapshot) error {
			if hostUID != "host-1" {
				t.Fatalf("unexpected snapshot host uid: %s", hostUID)
			}
			savedSnapshot = snapshot
			return nil
		},
	}
	svc := service.New(
		logx.NewNop(),
		clock.Real{},
		stubHostStore{},
		metricStore,
		stubEventBus{},
	)
	server := gogrpc.NewServer()
	monitorv1.RegisterMetricsIngestServiceServer(server, NewServer(svc))
	defer server.Stop()

	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := gogrpc.DialContext(
		context.Background(),
		listener.Addr().String(),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := monitorv1.NewMetricsIngestServiceClient(conn)
	resp, err := client.PushMetricBatch(context.Background(), &monitorv1.PushMetricBatchRequest{
		HostUid:     "host-1",
		AgentId:     "agent-1",
		BatchSeq:    1,
		CollectedAt: timestamppb.New(collectedAt),
		Host: &monitorv1.HostIdentity{
			HostUid:    "host-1",
			Hostname:   "node-1",
			PrimaryIp:  "10.0.0.1",
			TenantCode: "default",
		},
		Agent: &monitorv1.AgentMetadata{
			AgentId: "agent-1",
			Version: "v0.1.0",
		},
		Points: []*monitorv1.MetricPoint{
			{Name: "host_cpu_usage_pct", Value: 1, Labels: map[string]string{"cpu": "cpu0"}},
		},
		Collectors: []*monitorv1.CollectorStatus{
			{
				Name:           "cpu",
				State:          monitorv1.CollectorState_COLLECTOR_STATE_AVAILABLE,
				MetricNames:    []string{"host_cpu_usage_pct"},
				DurationMicros: 2500,
			},
		},
	})
	if err != nil {
		t.Fatalf("PushMetricBatch: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("unexpected ack code: %d", resp.Code)
	}
	stats := svc.Stats()
	if stats.MetricBatches != 1 {
		t.Fatalf("unexpected metric batch count: %d", stats.MetricBatches)
	}
	if !savedSnapshot.CollectedAt.Equal(collectedAt) {
		t.Fatalf("unexpected snapshot collection time: %s", savedSnapshot.CollectedAt)
	}
	if len(savedSnapshot.Points) != 1 || savedSnapshot.Points[0].Labels["cpu"] != "cpu0" {
		t.Fatalf("unexpected saved points: %+v", savedSnapshot.Points)
	}
	if len(savedSnapshot.Collectors) != 1 || savedSnapshot.Collectors[0].State != contracts.CollectorStateAvailable {
		t.Fatalf("unexpected saved collectors: %+v", savedSnapshot.Collectors)
	}
}

func TestPushMetricBatchReturnsFailedPreconditionForMissingTenant(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	svc := service.New(
		logx.NewNop(),
		clock.Real{},
		stubHostStore{reportMetricsErr: hostruntime.ErrTenantNotFound},
		stubMetricStore{},
		stubEventBus{},
	)
	server := gogrpc.NewServer()
	monitorv1.RegisterMetricsIngestServiceServer(server, NewServer(svc))
	defer server.Stop()

	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := gogrpc.DialContext(
		context.Background(),
		listener.Addr().String(),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := monitorv1.NewMetricsIngestServiceClient(conn)
	_, err = client.PushMetricBatch(context.Background(), &monitorv1.PushMetricBatchRequest{
		HostUid:  "host-1",
		AgentId:  "agent-1",
		BatchSeq: 1,
		Host: &monitorv1.HostIdentity{
			HostUid:    "host-1",
			TenantCode: "tenant-missing",
		},
		Agent: &monitorv1.AgentMetadata{
			AgentId: "agent-1",
		},
	})
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

type stubHostStore struct {
	reportMetricsErr error
}

func (s stubHostStore) AllocateTenant(context.Context) (string, error) {
	return "", nil
}

func (s stubHostStore) RegisterAgent(context.Context, contracts.RegisterAgentRequest, time.Time) (state.HostSnapshot, contracts.AgentConfig, string, error) {
	return state.HostSnapshot{}, contracts.AgentConfig{}, "", nil
}

func (s stubHostStore) Heartbeat(context.Context, contracts.HeartbeatRequest, time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	return state.HostSnapshot{}, contracts.AgentConfig{}, nil
}

func (s stubHostStore) ReportMetrics(_ context.Context, req contracts.PushMetricBatchRequest, _ contracts.AgentDigest, _ time.Time) (state.HostSnapshot, error) {
	if s.reportMetricsErr != nil {
		return state.HostSnapshot{}, s.reportMetricsErr
	}
	if req.Host.TenantCode == "" {
		return state.HostSnapshot{}, errors.New("expected host identity in metric push")
	}
	return state.HostSnapshot{HostUID: "host-1"}, nil
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

type stubMetricStore struct {
	saveMetricSnapshot func(context.Context, string, contracts.MetricSnapshot) error
}

func (stubMetricStore) AppendHeartbeatMetrics(context.Context, string, time.Time, contracts.AgentDigest) error {
	return nil
}

func (s stubMetricStore) SaveMetricSnapshot(ctx context.Context, hostUID string, snapshot contracts.MetricSnapshot) error {
	if s.saveMetricSnapshot != nil {
		return s.saveMetricSnapshot(ctx, hostUID, snapshot)
	}
	return nil
}

func (stubMetricStore) GetMetricSnapshot(context.Context, string) (contracts.MetricSnapshot, bool, error) {
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

func (stubEventBus) SubscribeHostEvents(context.Context) (<-chan hostruntime.HostEvent, error) {
	return nil, nil
}
