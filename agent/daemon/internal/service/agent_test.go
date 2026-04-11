package service

import (
	"context"
	"net"
	"testing"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

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
		Host: contracts.HostIdentity{
			HostUID:    "host-test",
			Hostname:   "node-1",
			PrimaryIP:  "10.0.0.1",
			Region:     "local",
			Env:        "dev",
			Role:       "node",
			TenantCode: "tenant-ok",
		},
	}, logx.NewNop())
	defer agent.Close()

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
	if recorder.lastRequest.GetHost().GetTenantCode() != "tenant-ok" {
		t.Fatalf("unexpected tenant code: %q", recorder.lastRequest.GetHost().GetTenantCode())
	}
	if recorder.lastRequest.GetHost().GetHostname() != "node-1" {
		t.Fatalf("unexpected hostname: %q", recorder.lastRequest.GetHost().GetHostname())
	}
	if recorder.lastRequest.GetAgent().GetAgentId() != "agent-test" {
		t.Fatalf("unexpected nested agent id: %q", recorder.lastRequest.GetAgent().GetAgentId())
	}
	if len(recorder.lastRequest.Points) == 0 {
		t.Fatal("expected metric points to be sent")
	}
}

func TestRunStopsOnInvalidTenant(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := grpc.NewServer()
	defer server.Stop()

	recorder := &recordingIngestServer{streamErr: grpcstatus.Error(codes.FailedPrecondition, "tenant not found")}
	monitorv1.RegisterMetricsIngestServiceServer(server, recorder)

	go func() {
		_ = server.Serve(listener)
	}()

	agent := New(Config{
		IngestGatewayGRPCAddr: listener.Addr().String(),
		LoopInterval:          time.Second,
		Host: contracts.HostIdentity{
			HostUID:    "host-stop",
			Hostname:   "node-stop",
			PrimaryIP:  "10.0.0.2",
			TenantCode: "tenant-missing",
		},
	}, logx.NewNop())
	defer agent.Close()

	err = agent.Run(context.Background())
	if grpcstatus.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
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
	streamErr    error
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
	if s.streamErr != nil {
		return s.streamErr
	}
	return stream.Send(&monitorv1.MetricBatchAck{
		Ack:      &monitorv1.Ack{},
		BatchSeq: req.GetBatchSeq(),
	})
}

func (s *recordingIngestServer) PushEventBatch(context.Context, *monitorv1.PushEventBatchRequest) (*monitorv1.Ack, error) {
	return &monitorv1.Ack{}, nil
}
