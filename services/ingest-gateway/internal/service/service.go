package service

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
)

type Counters struct {
	MetricBatches int `json:"metric_batches"`
	EventBatches  int `json:"event_batches"`
	ProbeReports  int `json:"probe_reports"`
}

type Service struct {
	logger *slog.Logger
	clock  clock.Clock
	mu     sync.Mutex
	stats  Counters
}

func New(logger *slog.Logger, clk clock.Clock) *Service {
	return &Service{logger: logger, clock: clk}
}

func (s *Service) PushMetricBatch(req contracts.PushMetricBatchRequest) contracts.AckResponse {
	s.mu.Lock()
	s.stats.MetricBatches++
	s.mu.Unlock()

	s.logger.Info("metric batch accepted", "host_uid", req.HostUID, "points", len(req.Points))
	return s.ack("metrics accepted")
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

func (s *Service) ack(message string) contracts.AckResponse {
	return contracts.AckResponse{
		RequestID:  ids.New("req"),
		Code:       0,
		Message:    message,
		ServerTime: s.clock.Now(),
	}
}
