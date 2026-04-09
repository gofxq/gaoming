package service

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"google.golang.org/grpc"
)

func TestMasterAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{
			name:     "root base",
			base:     "http://127.0.0.1:8080",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
		{
			name:     "service base",
			base:     "http://127.0.0.1:8080/master",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
		{
			name:     "api base",
			base:     "http://127.0.0.1:8080/master/api/v1",
			endpoint: "agents/register",
			want:     "http://127.0.0.1:8080/master/api/v1/agents/register",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := masterAPIURL(tt.base, tt.endpoint); got != tt.want {
				t.Fatalf("masterAPIURL(%q, %q) = %q, want %q", tt.base, tt.endpoint, got, tt.want)
			}
		})
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
		t.Fatal("expected grpc request to be recorded")
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
	monitorv1.UnimplementedMetricsIngestServiceServer
	lastRequest *monitorv1.PushMetricBatchRequest
}

func (s *recordingIngestServer) PushMetricBatch(_ context.Context, req *monitorv1.PushMetricBatchRequest) (*monitorv1.Ack, error) {
	copied := *req
	copied.Points = append([]*monitorv1.MetricPoint(nil), req.Points...)
	s.lastRequest = &copied
	return &monitorv1.Ack{}, nil
}

func (s *recordingIngestServer) PushEventBatch(context.Context, *monitorv1.PushEventBatchRequest) (*monitorv1.Ack, error) {
	return &monitorv1.Ack{}, nil
}
