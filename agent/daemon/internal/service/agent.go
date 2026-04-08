package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	MasterAPIURL          string
	IngestGatewayURL      string
	IngestGatewayGRPCAddr string
	ReportMode            string
	LoopInterval          time.Duration
	Host                  contracts.HostIdentity
	PersistTenant         func(string) error
}

type Agent struct {
	cfg        Config
	logger     *slog.Logger
	client     *http.Client
	grpcConn   *grpc.ClientConn
	grpcIngest monitorv1.MetricsIngestServiceClient
	agentID    string
	hostUID    string
	bootTime   time.Time
	hbSeq      int64
	metricSeq  int64
	sampler    systemSampler
}

type apiError struct {
	StatusCode int
	Status     string
}

func (e apiError) Error() string {
	return fmt.Sprintf("unexpected status: %s", e.Status)
}

func New(cfg Config, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		logger:   logger,
		client:   &http.Client{Timeout: 5 * time.Second},
		agentID:  ids.New("agent"),
		bootTime: time.Now().UTC(),
		sampler:  newSystemSampler(),
	}
}

func (a *Agent) Close() error {
	if a.grpcConn != nil {
		return a.grpcConn.Close()
	}
	return nil
}

func (a *Agent) Run(ctx context.Context) error {
	for {
		if err := a.register(ctx); err == nil {
			break
		} else {
			a.logger.Warn("register agent failed, retrying", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	ticker := time.NewTicker(a.cfg.LoopInterval)
	defer ticker.Stop()

	a.sendCycle(ctx)
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("agent stopped")
			return nil
		case <-ticker.C:
			a.sendCycle(ctx)
		}
	}
}

func (a *Agent) register(ctx context.Context) error {
	payload := contracts.RegisterAgentRequest{
		Host: a.cfg.Host,
		Agent: contracts.AgentMetadata{
			AgentID:      a.agentID,
			Version:      "v0.1.0",
			Capabilities: []string{"heartbeat", "metrics"},
			BootTime:     a.bootTime,
		},
	}

	var resp contracts.RegisterAgentResponse
	if err := a.postJSON(ctx, masterAPIURL(a.cfg.MasterAPIURL, "agents/register"), payload, &resp); err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	a.hostUID = resp.HostUID
	if resp.TenantCode != "" && resp.TenantCode != a.cfg.Host.TenantCode {
		if a.cfg.PersistTenant != nil {
			if err := a.cfg.PersistTenant(resp.TenantCode); err != nil {
				return fmt.Errorf("persist tenant: %w", err)
			}
		}
		a.cfg.Host.TenantCode = resp.TenantCode
	}
	a.logger.Info("agent registered", "host_uid", a.hostUID, "tenant_code", a.cfg.Host.TenantCode)
	return nil
}

func (a *Agent) sendCycle(ctx context.Context) {
	now := time.Now().UTC()
	digest := a.digest(now)

	if err := a.pushMetricsWithDigest(ctx, now, digest); err != nil {
		a.logger.Error("push metrics failed", "error", err)
	}
	if err := a.pushHeartbeat(ctx, now, digest); err != nil {
		a.logger.Error("push heartbeat failed", "error", err)
	}
}

func (a *Agent) pushHeartbeat(ctx context.Context, now time.Time, digest contracts.AgentDigest) error {
	if a.hostUID == "" {
		if err := a.register(ctx); err != nil {
			return err
		}
	}

	a.hbSeq++
	payload := contracts.HeartbeatRequest{
		HostUID: a.hostUID,
		AgentID: a.agentID,
		Seq:     a.hbSeq,
		TS:      now,
		Digest:  digest,
	}

	var resp contracts.HeartbeatResponse
	if err := a.postJSON(ctx, masterAPIURL(a.cfg.MasterAPIURL, "agents/heartbeat"), payload, &resp); err != nil {
		var apiErr apiError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			a.logger.Warn("heartbeat target missing on server, re-registering", "host_uid", a.hostUID)
			a.hostUID = ""
			return a.register(ctx)
		}
		return err
	}
	a.logger.Info("heartbeat sent", "host_uid", a.hostUID, "seq", a.hbSeq)
	return nil
}

func (a *Agent) pushMetricsWithDigest(ctx context.Context, now time.Time, digest contracts.AgentDigest) error {
	a.metricSeq++
	payload := contracts.PushMetricBatchRequest{
		HostUID:     a.hostUID,
		AgentID:     a.agentID,
		BatchSeq:    a.metricSeq,
		CollectedAt: now,
		Points: []contracts.MetricPoint{
			{Name: "runtime_uptime_seconds", Value: time.Since(a.bootTime).Seconds(), TS: now},
			{Name: "agent_heartbeat_seq", Value: float64(a.hbSeq), TS: now},
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

	switch normalizeReportMode(a.cfg.ReportMode) {
	case reportModeGRPC:
		return a.pushMetricsGRPC(ctx, payload)
	default:
		var resp contracts.AckResponse
		if err := a.postJSON(ctx, ingestAPIURL(a.cfg.IngestGatewayURL, "metrics"), payload, &resp); err != nil {
			return err
		}
	}
	a.logger.Info("metrics sent", "host_uid", a.hostUID, "batch_seq", a.metricSeq)
	return nil
}

func (a *Agent) pushMetricsGRPC(ctx context.Context, payload contracts.PushMetricBatchRequest) error {
	client, err := a.grpcIngestClient(ctx)
	if err != nil {
		return err
	}

	if _, err := client.PushMetricBatch(ctx, toProtoPushMetricBatchRequest(payload)); err != nil {
		return err
	}
	return nil
}

func masterAPIURL(base string, endpoint string) string {
	return serviceAPIURL(base, "/master", "/master/api/v1", endpoint)
}

func ingestAPIURL(base string, endpoint string) string {
	return serviceAPIURL(base, "/ingest", "/ingest/api/v1", endpoint)
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

func (a *Agent) grpcIngestClient(ctx context.Context) (monitorv1.MetricsIngestServiceClient, error) {
	if a.grpcIngest != nil {
		return a.grpcIngest, nil
	}

	conn, err := grpc.DialContext(
		ctx,
		a.cfg.IngestGatewayGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	a.grpcConn = conn
	a.grpcIngest = monitorv1.NewMetricsIngestServiceClient(conn)
	return a.grpcIngest, nil
}

const (
	reportModeHTTP = "http"
	reportModeGRPC = "grpc"
)

func normalizeReportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case reportModeGRPC:
		return reportModeGRPC
	default:
		return reportModeHTTP
	}
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
		Points:      points,
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
