package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"google.golang.org/grpc"
)

func TestRegisterGRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := grpc.NewServer()
	defer server.Stop()

	recorder := &recordingIngestServer{}
	monitorv1.RegisterAgentControlServiceServer(server, recorder)
	monitorv1.RegisterMetricsIngestServiceServer(server, recorder)

	go func() {
		_ = server.Serve(listener)
	}()

	masterServer := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost || r.URL.Path != "/master/api/v1/install/tenant" {
			w.WriteHeader(nethttp.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_code": "tenant-master",
		})
	}))
	defer masterServer.Close()

	persistedTenant := ""
	agent := New(Config{
		MasterAPIURL:          masterServer.URL,
		IngestGatewayGRPCAddr: listener.Addr().String(),
		LoopInterval:          time.Second,
		PersistTenant: func(value string) error {
			persistedTenant = value
			return nil
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer agent.Close()

	if err := agent.register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}

	if recorder.lastRegister == nil {
		t.Fatal("expected register request to be recorded")
	}
	if recorder.lastRegister.GetHost().GetTenantCode() != "tenant-master" {
		t.Fatalf("unexpected tenant code sent to ingest: %q", recorder.lastRegister.GetHost().GetTenantCode())
	}
	if agent.hostUID != "host-test" {
		t.Fatalf("unexpected host uid: %q", agent.hostUID)
	}
	if persistedTenant != "tenant-master" {
		t.Fatalf("unexpected persisted tenant: %q", persistedTenant)
	}
}

func TestRegisterGRPCFallsBackToLocalTenantWhenMasterFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := grpc.NewServer()
	defer server.Stop()

	recorder := &recordingIngestServer{}
	monitorv1.RegisterAgentControlServiceServer(server, recorder)
	monitorv1.RegisterMetricsIngestServiceServer(server, recorder)

	go func() {
		_ = server.Serve(listener)
	}()

	masterServer := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		nethttp.Error(w, "boom", nethttp.StatusInternalServerError)
	}))
	defer masterServer.Close()

	persistedTenant := ""
	agent := New(Config{
		MasterAPIURL:          masterServer.URL,
		IngestGatewayGRPCAddr: listener.Addr().String(),
		LoopInterval:          time.Second,
		PersistTenant: func(value string) error {
			persistedTenant = value
			return nil
		},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer agent.Close()

	if err := agent.register(context.Background()); err != nil {
		t.Fatalf("register: %v", err)
	}

	if recorder.lastRegister == nil {
		t.Fatal("expected register request to be recorded")
	}
	tenantCode := recorder.lastRegister.GetHost().GetTenantCode()
	if !strings.HasPrefix(tenantCode, "tenant-") {
		t.Fatalf("expected fallback tenant code, got %q", tenantCode)
	}
	if persistedTenant != tenantCode {
		t.Fatalf("expected persisted tenant %q, got %q", tenantCode, persistedTenant)
	}
}

func TestPushMetricsWithDigestGRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := grpc.NewServer()
	defer server.Stop()

	recorder := &recordingIngestServer{}
	monitorv1.RegisterAgentControlServiceServer(server, recorder)
	monitorv1.RegisterMetricsIngestServiceServer(server, recorder)

	go func() {
		_ = server.Serve(listener)
	}()

	agent := New(Config{
		IngestGatewayGRPCAddr: listener.Addr().String(),
		LoopInterval:          time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer agent.Close()

	agent.hostUID = "host-test"
	agent.agentID = "agent-test"
	agent.bootTime = time.Now().Add(-time.Minute)
	agent.sampler = fixedSampler{}

	now := time.Now().UTC()
	digest := agent.digest(now)

	if err := agent.pushMetricsWithDigest(context.Background(), now, digest); err != nil {
		t.Fatalf("pushMetricsWithDigest: %v", err)
	}

	if recorder.lastRequest == nil {
		t.Fatal("expected stream request to be recorded")
	}
	if recorder.lastRequest.HostUid != "host-test" {
		t.Fatalf("unexpected host uid: %q", recorder.lastRequest.HostUid)
	}
	if recorder.lastRequest.AgentId != "agent-test" {
		t.Fatalf("unexpected agent id: %q", recorder.lastRequest.AgentId)
	}
	if len(recorder.lastRequest.Points) == 0 {
		t.Fatal("expected metric points to be sent")
	}
}

type fixedSampler struct{}

func (fixedSampler) Sample(time.Time) systemMetrics {
	return systemMetrics{}
}

type recordingIngestServer struct {
	monitorv1.UnimplementedAgentControlServiceServer
	monitorv1.UnimplementedMetricsIngestServiceServer
	lastRegister *monitorv1.RegisterAgentRequest
	lastRequest  *monitorv1.PushMetricBatchRequest
	tenantCode   string
}

func (s *recordingIngestServer) RegisterAgent(_ context.Context, req *monitorv1.RegisterAgentRequest) (*monitorv1.RegisterAgentResponse, error) {
	copied := *req
	s.lastRegister = &copied
	return &monitorv1.RegisterAgentResponse{
		Ack:        &monitorv1.Ack{},
		HostUid:    "host-test",
		TenantCode: s.tenantCode,
	}, nil
}

func (s *recordingIngestServer) PushMetricBatch(_ context.Context, req *monitorv1.PushMetricBatchRequest) (*monitorv1.Ack, error) {
	copied := *req
	copied.Points = append([]*monitorv1.MetricPoint(nil), req.Points...)
	s.lastRequest = &copied
	return &monitorv1.Ack{}, nil
}

func (s *recordingIngestServer) StreamMetricBatches(stream monitorv1.MetricsIngestService_StreamMetricBatchesServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	copied := *req
	copied.Points = append([]*monitorv1.MetricPoint(nil), req.Points...)
	s.lastRequest = &copied
	return stream.Send(&monitorv1.MetricBatchAck{
		Ack:      &monitorv1.Ack{},
		BatchSeq: req.GetBatchSeq(),
	})
}

func (s *recordingIngestServer) PushEventBatch(context.Context, *monitorv1.PushEventBatchRequest) (*monitorv1.Ack, error) {
	return &monitorv1.Ack{}, nil
}
