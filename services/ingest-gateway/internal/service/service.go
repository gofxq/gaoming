package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/ids"
)

type Counters struct {
	MetricBatches int `json:"metric_batches"`
	EventBatches  int `json:"event_batches"`
	ProbeReports  int `json:"probe_reports"`
}

type Service struct {
	hostStore   hostruntime.HostStateStore
	metricStore hostruntime.MetricWindowStore
	eventBus    hostruntime.EventBus
	logger      *slog.Logger
	clock       clock.Clock
	mu          sync.Mutex
	stats       Counters
}

func New(logger *slog.Logger, clk clock.Clock, hostStore hostruntime.HostStateStore, metricStore hostruntime.MetricWindowStore, eventBus hostruntime.EventBus) *Service {
	return &Service{
		hostStore:   hostStore,
		metricStore: metricStore,
		eventBus:    eventBus,
		logger:      logger,
		clock:       clk,
	}
}

func (s *Service) RegisterAgent(ctx context.Context, req contracts.RegisterAgentRequest) (contracts.RegisterAgentResponse, error) {
	snapshot, config, tenantCode, err := s.hostStore.RegisterAgent(ctx, req, s.clock.Now())
	if err != nil {
		return contracts.RegisterAgentResponse{}, err
	}
	if err := s.eventBus.PublishHostUpsert(ctx, snapshot); err != nil {
		return contracts.RegisterAgentResponse{}, err
	}
	s.logger.Info("agent registered", "host_uid", snapshot.HostUID, "hostname", snapshot.Hostname)

	return contracts.RegisterAgentResponse{
		RequestID:  ids.New("req"),
		Message:    "registered",
		HostUID:    snapshot.HostUID,
		TenantCode: tenantCode,
		Config:     config,
	}, nil
}

func (s *Service) AllocateInstallTenant(ctx context.Context) (contracts.AllocateInstallTenantResponse, error) {
	tenantCode, err := s.hostStore.AllocateTenant(ctx)
	if err != nil {
		return contracts.AllocateInstallTenantResponse{}, err
	}
	return contracts.AllocateInstallTenantResponse{
		RequestID:  ids.New("req"),
		Message:    "tenant allocated",
		TenantCode: tenantCode,
	}, nil
}

func (s *Service) PushMetricBatch(ctx context.Context, req contracts.PushMetricBatchRequest) (contracts.AckResponse, error) {
	return s.processMetricBatch(ctx, req)
}

func (s *Service) processMetricBatch(ctx context.Context, req contracts.PushMetricBatchRequest) (contracts.AckResponse, error) {
	now := s.clock.Now()
	digest := digestFromMetricBatch(req)

	snapshot, err := s.hostStore.ReportMetrics(ctx, req, digest, now)
	if err != nil {
		return contracts.AckResponse{}, err
	}
	if err := s.metricStore.AppendHeartbeatMetrics(ctx, req.HostUID, now, digest); err != nil {
		return contracts.AckResponse{}, err
	}
	if err := s.eventBus.PublishHostUpsert(ctx, snapshot); err != nil {
		return contracts.AckResponse{}, err
	}

	s.mu.Lock()
	s.stats.MetricBatches++
	s.mu.Unlock()

	s.logger.Info("metric batch accepted", "host_uid", req.HostUID, "points", len(req.Points))
	return s.ack("metrics accepted"), nil
}

func (s *Service) PushEventBatch(req contracts.PushEventBatchRequest) contracts.AckResponse {
	s.mu.Lock()
	s.stats.EventBatches++
	s.mu.Unlock()

	s.logger.Info("event batch accepted", "host_uid", req.HostUID, "events", len(req.Events))
	return s.ack("events accepted")
}

func (s *Service) ReportProbeResults(req contracts.ReportProbeResultsRequest) contracts.AckResponse {
	s.mu.Lock()
	s.stats.ProbeReports++
	s.mu.Unlock()

	s.logger.Info("probe report accepted", "worker_id", req.WorkerID, "results", len(req.Results))
	return s.ack("probe results accepted")
}

func (s *Service) Stats() Counters {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *Service) Health() map[string]any {
	return map[string]any{
		"status": "ok",
		"time":   s.clock.Now().Format(time.RFC3339),
	}
}

func (s *Service) ReconcileOfflineHosts(ctx context.Context) (int, error) {
	changed, err := s.hostStore.ReconcileOffline(ctx, s.clock.Now())
	if err != nil {
		return 0, err
	}
	for _, snapshot := range changed {
		if err := s.eventBus.PublishHostUpsert(ctx, snapshot); err != nil {
			return 0, err
		}
	}
	return len(changed), nil
}

func (s *Service) ack(message string) contracts.AckResponse {
	return contracts.AckResponse{
		RequestID:  ids.New("req"),
		Code:       0,
		Message:    message,
		ServerTime: s.clock.Now(),
	}
}

func digestFromMetricBatch(req contracts.PushMetricBatchRequest) contracts.AgentDigest {
	digest := contracts.AgentDigest{
		QueueDepth:         0,
		LastMetricBatchSeq: req.BatchSeq,
	}
	for _, point := range req.Points {
		switch point.Name {
		case "host_cpu_usage_pct":
			digest.CPUUsagePct = point.Value
		case "host_mem_used_pct":
			digest.MemUsedPct = point.Value
		case "host_mem_available_bytes":
			digest.MemAvailableBytes = int64(point.Value)
		case "host_swap_used_pct":
			digest.SwapUsedPct = point.Value
		case "host_disk_used_pct":
			digest.DiskUsedPct = point.Value
		case "host_disk_free_bytes":
			digest.DiskFreeBytes = int64(point.Value)
		case "host_disk_inodes_used_pct":
			digest.DiskInodesUsedPct = point.Value
		case "host_disk_read_bps":
			digest.DiskReadBPS = int64(point.Value)
		case "host_disk_write_bps":
			digest.DiskWriteBPS = int64(point.Value)
		case "host_disk_read_iops":
			digest.DiskReadIOPS = int64(point.Value)
		case "host_disk_write_iops":
			digest.DiskWriteIOPS = int64(point.Value)
		case "host_load1":
			digest.Load1 = point.Value
		case "host_net_rx_bps":
			digest.NetRxBPS = int64(point.Value)
		case "host_net_tx_bps":
			digest.NetTxBPS = int64(point.Value)
		case "host_net_rx_packets_ps":
			digest.NetRxPacketsPS = int64(point.Value)
		case "host_net_tx_packets_ps":
			digest.NetTxPacketsPS = int64(point.Value)
		}
	}
	return digest
}
