package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	MasterAPIURL          string
	IngestGatewayGRPCAddr string
	LoopInterval          time.Duration
	Host                  contracts.HostIdentity
	PersistTenant         func(string) error
}

type Agent struct {
	cfg        Config
	logger     *slog.Logger
	client     *http.Client
	grpcConn   *grpc.ClientConn
	grpcCtl    monitorv1.AgentControlServiceClient
	grpcIngest monitorv1.MetricsIngestServiceClient
	metricSink monitorv1.MetricsIngestService_StreamMetricBatchesClient
	agentID    string
	hostUID    string
	bootTime   time.Time
	metricSeq  int64
	sampler    systemSampler
}

func New(cfg Config, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		logger:   logger,
		client:   &http.Client{Timeout: 5 * time.Second},
		agentID:  ids.New("agent"),
		hostUID:  cfg.Host.HostUID,
		bootTime: time.Now().UTC(),
		sampler:  newSystemSampler(),
	}
}

func (a *Agent) Close() error {
	if a.metricSink != nil {
		_ = a.metricSink.CloseSend()
	}
	if a.grpcConn != nil {
		return a.grpcConn.Close()
	}
	return nil
}

func (a *Agent) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.cfg.LoopInterval)
	defer ticker.Stop()

	if err := a.sendCycle(ctx); err != nil {
		if isFatalMetricError(err) {
			return err
		}
		a.logger.Error("push metrics failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("agent stopped")
			return nil
		case <-ticker.C:
			if err := a.sendCycle(ctx); err != nil {
				if isFatalMetricError(err) {
					return err
				}
				a.logger.Error("push metrics failed", "error", err)
			}
		}
	}
}

func (a *Agent) sendCycle(ctx context.Context) error {
	now := time.Now().UTC()
	digest := a.digest(now)

	return a.pushMetricsWithDigest(ctx, now, digest)
}

func (a *Agent) pushMetricsWithDigest(ctx context.Context, now time.Time, digest contracts.AgentDigest) error {
	a.metricSeq++
	payload := contracts.PushMetricBatchRequest{
		HostUID:     a.hostUID,
		AgentID:     a.agentID,
		BatchSeq:    a.metricSeq,
		CollectedAt: now,
		Host:        a.cfg.Host,
		Agent: contracts.AgentMetadata{
			AgentID:      a.agentID,
			Version:      "v0.1.0",
			Capabilities: []string{"metrics", "stream_metrics"},
			BootTime:     a.bootTime,
		},
		Points: []contracts.MetricPoint{
			{Name: "runtime_uptime_seconds", Value: time.Since(a.bootTime).Seconds(), TS: now},
			{Name: "host_cpu_usage_pct", Value: digest.CPUUsagePct, TS: now},
			{Name: "host_mem_used_pct", Value: digest.MemUsedPct, TS: now},
			{Name: "host_mem_available_bytes", Value: float64(digest.MemAvailableBytes), TS: now},
			{Name: "host_swap_used_pct", Value: digest.SwapUsedPct, TS: now},
			{Name: "host_disk_used_pct", Value: digest.DiskUsedPct, TS: now},
			{Name: "host_disk_free_bytes", Value: float64(digest.DiskFreeBytes), TS: now},
			{Name: "host_disk_inodes_used_pct", Value: digest.DiskInodesUsedPct, TS: now},
			{Name: "host_disk_read_bps", Value: float64(digest.DiskReadBPS), TS: now},
			{Name: "host_disk_write_bps", Value: float64(digest.DiskWriteBPS), TS: now},
			{Name: "host_disk_read_iops", Value: float64(digest.DiskReadIOPS), TS: now},
			{Name: "host_disk_write_iops", Value: float64(digest.DiskWriteIOPS), TS: now},
			{Name: "host_load1", Value: digest.Load1, TS: now},
			{Name: "host_net_rx_bps", Value: float64(digest.NetRxBPS), TS: now},
			{Name: "host_net_tx_bps", Value: float64(digest.NetTxBPS), TS: now},
			{Name: "host_net_rx_packets_ps", Value: float64(digest.NetRxPacketsPS), TS: now},
			{Name: "host_net_tx_packets_ps", Value: float64(digest.NetTxPacketsPS), TS: now},
		},
	}

	if err := a.pushMetricsGRPC(ctx, payload); err != nil {
		return err
	}
	a.logger.Info("metrics sent", "host_uid", a.hostUID, "batch_seq", a.metricSeq)
	return nil
}

func (a *Agent) pushMetricsGRPC(ctx context.Context, payload contracts.PushMetricBatchRequest) error {
	stream, err := a.metricStreamClient(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(toProtoPushMetricBatchRequest(payload)); err != nil {
		a.resetMetricStream()
		return err
	}

	ack, err := stream.Recv()
	if err != nil {
		a.resetMetricStream()
		return err
	}
	if ack.GetAck() != nil && ack.GetAck().GetCode() != 0 {
		return fmt.Errorf("metric batch rejected: %s", ack.GetAck().GetMessage())
	}
	if batchSeq := ack.GetBatchSeq(); batchSeq != 0 && batchSeq != payload.BatchSeq {
		return fmt.Errorf("metric batch ack seq mismatch: got %d want %d", batchSeq, payload.BatchSeq)
	}
	return nil
}

func isFatalMetricError(err error) bool {
	return grpcstatus.Code(err) == codes.FailedPrecondition
}

func (a *Agent) digest(now time.Time) contracts.AgentDigest {
	metrics := a.sampler.Sample(now)
	return contracts.AgentDigest{
		CPUUsagePct:        metrics.CPUUsagePct,
		MemUsedPct:         metrics.MemUsedPct,
		MemAvailableBytes:  metrics.MemAvailableBytes,
		SwapUsedPct:        metrics.SwapUsedPct,
		DiskUsedPct:        metrics.DiskUsedPct,
		DiskFreeBytes:      metrics.DiskFreeBytes,
		DiskInodesUsedPct:  metrics.DiskInodesUsedPct,
		DiskReadBPS:        metrics.DiskReadBPS,
		DiskWriteBPS:       metrics.DiskWriteBPS,
		DiskReadIOPS:       metrics.DiskReadIOPS,
		DiskWriteIOPS:      metrics.DiskWriteIOPS,
		Load1:              metrics.Load1,
		NetRxBPS:           metrics.NetRxBPS,
		NetTxBPS:           metrics.NetTxBPS,
		NetRxPacketsPS:     metrics.NetRxPacketsPS,
		NetTxPacketsPS:     metrics.NetTxPacketsPS,
		QueueDepth:         0,
		LastMetricBatchSeq: a.metricSeq,
	}
}

type apiError struct {
	StatusCode int
	Status     string
}

func (e apiError) Error() string {
	return fmt.Sprintf("unexpected status: %s", e.Status)
}

func (a *Agent) postJSON(ctx context.Context, url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return apiError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func masterAPIURL(base string, endpoint string) string {
	return serviceAPIURL(base, "/master", "/master/api/v1", endpoint)
}

func serviceAPIURL(base string, servicePrefix string, apiPrefix string, endpoint string) string {
	base = strings.TrimRight(base, "/")
	endpoint = strings.TrimLeft(endpoint, "/")

	switch {
	case strings.HasSuffix(base, apiPrefix):
		return base + "/" + endpoint
	case strings.HasSuffix(base, servicePrefix):
		return base + strings.TrimPrefix(apiPrefix, servicePrefix) + "/" + endpoint
	case strings.HasSuffix(base, "/api/v1"):
		return base + "/" + endpoint
	default:
		return base + apiPrefix + "/" + endpoint
	}
}

func (a *Agent) grpcIngestClient(ctx context.Context) (monitorv1.MetricsIngestServiceClient, error) {
	if a.grpcIngest != nil {
		return a.grpcIngest, nil
	}

	conn, err := a.grpcConnOrDial(ctx)
	if err != nil {
		return nil, err
	}

	a.grpcIngest = monitorv1.NewMetricsIngestServiceClient(conn)
	return a.grpcIngest, nil
}

func (a *Agent) grpcControlClient(ctx context.Context) (monitorv1.AgentControlServiceClient, error) {
	if a.grpcCtl != nil {
		return a.grpcCtl, nil
	}

	conn, err := a.grpcConnOrDial(ctx)
	if err != nil {
		return nil, err
	}

	a.grpcCtl = monitorv1.NewAgentControlServiceClient(conn)
	return a.grpcCtl, nil
}

func (a *Agent) metricStreamClient(ctx context.Context) (monitorv1.MetricsIngestService_StreamMetricBatchesClient, error) {
	if a.metricSink != nil {
		return a.metricSink, nil
	}

	client, err := a.grpcIngestClient(ctx)
	if err != nil {
		return nil, err
	}

	stream, err := client.StreamMetricBatches(ctx)
	if err != nil {
		return nil, err
	}
	a.metricSink = stream
	return a.metricSink, nil
}

func (a *Agent) grpcConnOrDial(ctx context.Context) (*grpc.ClientConn, error) {
	if a.grpcConn != nil {
		return a.grpcConn, nil
	}

	conn, err := grpc.NewClient(
		a.cfg.IngestGatewayGRPCAddr,
		grpc.WithTransportCredentials(grpcTransportCredentials(a.cfg.IngestGatewayGRPCAddr)),
	)
	if err != nil {
		logx.New("agent").Error("failed to create gRPC client", "error", err)
		return nil, err
	}
	a.grpcConn = conn
	return conn, nil
}

func (a *Agent) resetMetricStream() {
	if a.metricSink != nil {
		_ = a.metricSink.CloseSend()
	}
	a.metricSink = nil
}

func grpcTransportCredentials(addr string) credentials.TransportCredentials {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err == nil {
		switch {
		case strings.EqualFold(host, "localhost"):
			return insecure.NewCredentials()
		case isLoopbackIP(host):
			return insecure.NewCredentials()
		}
	}
	return credentials.NewClientTLSFromCert(nil, "")
}

func isLoopbackIP(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func toProtoPushMetricBatchRequest(req contracts.PushMetricBatchRequest) *monitorv1.PushMetricBatchRequest {
	points := make([]*monitorv1.MetricPoint, 0, len(req.Points))
	for _, point := range req.Points {
		points = append(points, &monitorv1.MetricPoint{
			Name:   point.Name,
			Value:  point.Value,
			Ts:     timestamppb.New(point.TS),
			Labels: cloneStringMap(point.Labels),
		})
	}

	return &monitorv1.PushMetricBatchRequest{
		HostUid:     req.HostUID,
		AgentId:     req.AgentID,
		BatchSeq:    req.BatchSeq,
		CollectedAt: timestamppb.New(req.CollectedAt),
		Host: &monitorv1.HostIdentity{
			HostUid:    req.Host.HostUID,
			Hostname:   req.Host.Hostname,
			PrimaryIp:  req.Host.PrimaryIP,
			Ips:        append([]string(nil), req.Host.IPs...),
			OsType:     req.Host.OSType,
			Arch:       req.Host.Arch,
			Region:     req.Host.Region,
			Az:         req.Host.AZ,
			Env:        req.Host.Env,
			Role:       req.Host.Role,
			Labels:     cloneStringMap(req.Host.Labels),
			TenantCode: req.Host.TenantCode,
		},
		Agent: &monitorv1.AgentMetadata{
			AgentId:      req.Agent.AgentID,
			Version:      req.Agent.Version,
			Capabilities: append([]string(nil), req.Agent.Capabilities...),
			BootTime:     timestamppb.New(req.Agent.BootTime),
		},
		Points: points,
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
