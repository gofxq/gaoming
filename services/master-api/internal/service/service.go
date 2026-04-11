package service

import (
	"context"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/pkg/state"
)

type Service struct {
	hostStore   repository.HostStateStore
	metricStore repository.MetricWindowStore
	opsStore    repository.OperationsStore
	eventBus    repository.EventBus
	clock       clock.Clock
	logger      *logx.Logger
}

type HostEvent = repository.HostEvent

const (
	HostEventUpsert = repository.HostEventUpsert
	HostEventDelete = repository.HostEventDelete
)

func New(hostStore repository.HostStateStore, metricStore repository.MetricWindowStore, opsStore repository.OperationsStore, eventBus repository.EventBus, clk clock.Clock, logger *logx.Logger) *Service {
	return &Service{
		hostStore:   hostStore,
		metricStore: metricStore,
		opsStore:    opsStore,
		eventBus:    eventBus,
		clock:       clk,
		logger:      logger,
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

func (s *Service) ListHosts(ctx context.Context, tenantCode string) ([]state.HostSnapshot, error) {
	return s.hostStore.ListHosts(ctx, tenantCode)
}

func (s *Service) SubscribeHostEvents(ctx context.Context) (<-chan HostEvent, error) {
	return s.eventBus.SubscribeHostEvents(ctx)
}

func (s *Service) GetHostMetricHistory(ctx context.Context, hostUID string) (map[state.MetricKey][]state.MetricPoint, error) {
	return s.metricStore.GetHostMetricHistory(ctx, hostUID)
}

func (s *Service) GetAllHostMetricHistory(ctx context.Context, hostUIDs []string) (map[string]map[state.MetricKey][]state.MetricPoint, error) {
	return s.metricStore.GetAllHostMetricHistory(ctx, hostUIDs)
}

func (s *Service) GetHost(ctx context.Context, hostUID string, tenantCode string) (state.HostSnapshot, bool, error) {
	return s.hostStore.GetHost(ctx, hostUID, tenantCode)
}

func (s *Service) CreateMaintenance(ctx context.Context, req contracts.CreateMaintenanceWindowRequest) (repository.MaintenanceWindow, error) {
	return s.opsStore.CreateMaintenance(ctx, req)
}

func (s *Service) AckAlert(ctx context.Context, alertID string, req contracts.AckAlertRequest) (contracts.AckResponse, error) {
	if err := s.opsStore.AckAlert(ctx, alertID, req.AckedBy, s.clock.Now()); err != nil {
		return contracts.AckResponse{}, err
	}
	return contracts.AckResponse{
		RequestID:  ids.New("req"),
		Code:       0,
		Message:    "acknowledged",
		ServerTime: s.clock.Now(),
	}, nil
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
