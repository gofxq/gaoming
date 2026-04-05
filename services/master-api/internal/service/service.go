package service

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/repository/memory"
)

type Service struct {
	store  *memory.Store
	clock  clock.Clock
	logger *slog.Logger
}

func New(store *memory.Store, clk clock.Clock, logger *slog.Logger) *Service {
	return &Service{store: store, clock: clk, logger: logger}
}

func (s *Service) RegisterAgent(req contracts.RegisterAgentRequest) contracts.RegisterAgentResponse {
	snapshot, config := s.store.RegisterAgent(req, s.clock.Now())
	s.logger.Info("agent registered", "host_uid", snapshot.HostUID, "hostname", snapshot.Hostname)

	return contracts.RegisterAgentResponse{
		RequestID: ids.New("req"),
		Message:   "registered",
		HostUID:   snapshot.HostUID,
		Config:    config,
	}
}

func (s *Service) Heartbeat(req contracts.HeartbeatRequest) (contracts.HeartbeatResponse, error) {
	_, config, err := s.store.Heartbeat(req, s.clock.Now())
	if err != nil {
		if errors.Is(err, memory.ErrHostNotFound) {
			return contracts.HeartbeatResponse{}, err
		}
		return contracts.HeartbeatResponse{}, err
	}

	return contracts.HeartbeatResponse{
		RequestID:                ids.New("req"),
		Message:                  "heartbeat accepted",
		NextHeartbeatIntervalSec: config.HeartbeatIntervalSec,
		DesiredConfigVersion:     config.ConfigVersion,
	}, nil
}

func (s *Service) ListHosts() []state.HostSnapshot {
	return s.store.ListHosts()
}

func (s *Service) SubscribeHosts() (string, <-chan []state.HostSnapshot, func()) {
	return s.store.Subscribe()
}

func (s *Service) GetHostMetricHistory(hostUID string) map[state.MetricKey][]state.MetricPoint {
	return s.store.GetMetricHistory(hostUID)
}

func (s *Service) GetAllHostMetricHistory() map[string]map[state.MetricKey][]state.MetricPoint {
	return s.store.GetAllMetricHistory()
}

func (s *Service) GetHost(hostUID string) (state.HostSnapshot, bool) {
	return s.store.GetHost(hostUID)
}

func (s *Service) CreateMaintenance(req contracts.CreateMaintenanceWindowRequest) any {
	return s.store.CreateMaintenance(req)
}

func (s *Service) AckAlert(alertID string, req contracts.AckAlertRequest) contracts.AckResponse {
	s.store.AckAlert(alertID, req.AckedBy)
	return contracts.AckResponse{
		RequestID:  ids.New("req"),
		Code:       0,
		Message:    "acknowledged",
		ServerTime: s.clock.Now(),
	}
}

func (s *Service) Health() map[string]any {
	return map[string]any{
		"status": "ok",
		"time":   s.clock.Now().Format(time.RFC3339),
	}
}

func (s *Service) ReconcileOfflineHosts() int {
	return s.store.ReconcileOffline(s.clock.Now())
}
