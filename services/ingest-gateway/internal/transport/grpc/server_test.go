package grpc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPushMetricBatch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	svc := service.New(slog.New(slog.NewTextHandler(io.Discard, nil)), clock.Real{})
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
		HostUid: "host-1",
		Points: []*monitorv1.MetricPoint{
			{Name: "cpu", Value: 1},
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
}
