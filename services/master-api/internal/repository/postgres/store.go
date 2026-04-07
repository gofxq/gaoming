package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/repository"
	"gorm.io/gorm"
)

const (
	defaultHeartbeatIntervalSec = 1
	defaultMetricIntervalSec    = 1
	defaultConfigVersion        = 1
)

type Config struct {
	TenantCode string
	TenantName string
}

type Store struct {
	db              *gorm.DB
	defaultTenantID int64
}

func NewStore(ctx context.Context, db *gorm.DB, cfg Config) (*Store, error) {
	tenantCode := cfg.TenantCode
	if tenantCode == "" {
		tenantCode = "default"
	}
	tenantName := cfg.TenantName
	if tenantName == "" {
		tenantName = "Default Tenant"
	}

	tenant, err := ensureTenant(ctx, db, tenantCode, tenantName)
	if err != nil {
		return nil, err
	}

	return &Store{db: db, defaultTenantID: tenant.ID}, nil
}

func (s *Store) AllocateTenant(ctx context.Context) (string, error) {
	tenantCode := ids.New("tenant")
	tenant, err := ensureTenant(ctx, s.db, tenantCode, tenantCode)
	if err != nil {
		return "", err
	}
	return tenant.TenantCode, nil
}

func (s *Store) RegisterAgent(ctx context.Context, req contracts.RegisterAgentRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, string, error) {
	var (
		snapshot   state.HostSnapshot
		cfg        contracts.AgentConfig
		tenantCode string
	)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hostUID := req.Host.HostUID
		if hostUID == "" {
			hostUID = ids.New("host")
		}

		tenantID, resolvedTenantCode, err := s.resolveTenantForRegister(ctx, tx, hostUID, req.Host.TenantCode)
		if err != nil {
			return err
		}
		tenantCode = resolvedTenantCode

		hostID, err := s.upsertHost(ctx, tx, tenantID, hostUID, req, now)
		if err != nil {
			return err
		}
		if err := s.replaceLabels(ctx, tx, hostID, req.Host.Labels); err != nil {
			return err
		}

		cfg, err = s.upsertAgentInstance(ctx, tx, hostID, req, now)
		if err != nil {
			return err
		}
		if err := s.upsertRegisteredStatus(ctx, tx, hostID, now); err != nil {
			return err
		}

		item, ok, err := s.getHostByUID(ctx, tx, hostUID, "")
		if err != nil {
			return err
		}
		if !ok {
			return repository.ErrHostNotFound
		}
		snapshot = item
		return nil
	})
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}

	return snapshot, cfg, tenantCode, nil
}

func (s *Store) Heartbeat(ctx context.Context, req contracts.HeartbeatRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	var (
		snapshot state.HostSnapshot
		cfg      contracts.AgentConfig
	)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		hostID, currentCfg, err := s.lookupAgentInstance(ctx, tx, req.HostUID, req.AgentID)
		if err != nil {
			return err
		}
		cfg = currentCfg

		if err := tx.Model(&agentInstanceModel{}).
			Where(&agentInstanceModel{HostID: hostID, AgentID: req.AgentID}).
			Updates(map[string]any{
				"state":        int(state.Up),
				"last_seen_at": now,
				"last_seq":     req.Seq,
				"updated_at":   now,
			}).Error; err != nil {
			return err
		}

		if err := s.upsertHeartbeatStatus(ctx, tx, hostID, req, now); err != nil {
			return err
		}

		item, ok, err := s.getHostByUID(ctx, tx, req.HostUID, "")
		if err != nil {
			return err
		}
		if !ok {
			return repository.ErrHostNotFound
		}
		snapshot = item
		return nil
	})
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}

	return snapshot, cfg, nil
}

func (s *Store) ListHosts(ctx context.Context, tenantCode string) ([]state.HostSnapshot, error) {
	return s.listHosts(ctx, s.db, "", tenantCode)
}

func (s *Store) GetHost(ctx context.Context, hostUID string, tenantCode string) (state.HostSnapshot, bool, error) {
	return s.getHostByUID(ctx, s.db, hostUID, tenantCode)
}

func (s *Store) ReconcileOffline(ctx context.Context, now time.Time) ([]state.HostSnapshot, error) {
	var statuses []hostStatusCurrentModel
	err := s.db.WithContext(ctx).
		Preload("Host").
		Where("last_agent_seen_at IS NOT NULL").
		Where("last_agent_seen_at < ?", now.Add(-15*time.Second)).
		Not(hostStatusCurrentModel{AgentState: int(state.Offline), OverallState: int(state.Offline)}).
		Order("host_id").
		Find(&statuses).Error
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
	}

	changed := make([]state.HostSnapshot, 0, len(statuses))
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, current := range statuses {
			if err := tx.Model(&hostStatusCurrentModel{}).
				Where(&hostStatusCurrentModel{HostID: current.HostID}).
				Updates(map[string]any{
					"agent_state":        int(state.Offline),
					"reachability_state": int(state.Offline),
					"overall_state":      int(state.Offline),
					"version":            current.Version + 1,
					"updated_at":         now,
				}).Error; err != nil {
				return err
			}

			item, ok, err := s.getHostByUID(ctx, tx, current.Host.HostUID, "")
			if err != nil {
				return err
			}
			if ok {
				changed = append(changed, item)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return changed, nil
}

func (s *Store) CreateMaintenance(ctx context.Context, req contracts.CreateMaintenanceWindowRequest) (repository.MaintenanceWindow, error) {
	model := maintenanceWindowModel{
		TenantID:  s.defaultTenantID,
		Title:     req.Title,
		ScopeType: req.ScopeType,
		ScopeRef:  req.ScopeRef,
		StartAt:   req.StartAt,
		EndAt:     req.EndAt,
		CreatedBy: req.CreatedBy,
		Reason:    emptyStringPtr(req.Reason),
		Enabled:   true,
	}
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return repository.MaintenanceWindow{}, err
	}

	return repository.MaintenanceWindow{
		ID:        strconv.FormatInt(model.ID, 10),
		Title:     req.Title,
		ScopeType: req.ScopeType,
		ScopeRef:  req.ScopeRef,
		StartAt:   req.StartAt,
		EndAt:     req.EndAt,
		CreatedBy: req.CreatedBy,
		Reason:    req.Reason,
	}, nil
}

func (s *Store) AckAlert(ctx context.Context, alertID string, ackedBy string, now time.Time) error {
	query := s.db.WithContext(ctx).Model(&alertEventModel{}).Where(&alertEventModel{TenantID: s.defaultTenantID})
	if id, err := strconv.ParseInt(alertID, 10, 64); err == nil {
		return query.Where(&alertEventModel{ID: id}).Updates(map[string]any{
			"acked_by": ackedBy,
			"acked_at": now,
		}).Error
	}

	return query.Where(&alertEventModel{Fingerprint: alertID}).Updates(map[string]any{
		"acked_by": ackedBy,
		"acked_at": now,
	}).Error
}

func (s *Store) resolveTenantForRegister(ctx context.Context, db *gorm.DB, hostUID string, tenantCode string) (int64, string, error) {
	if tenantCode == "" {
		existingTenantCode, err := s.lookupTenantCodeByHostUID(ctx, db, hostUID)
		if err != nil {
			return 0, "", err
		}
		if existingTenantCode != "" {
			tenantCode = existingTenantCode
		}
	}
	if tenantCode == "" {
		tenantCode = ids.New("tenant")
	}

	tenant, err := ensureTenant(ctx, db, tenantCode, tenantCode)
	if err != nil {
		return 0, "", err
	}
	return tenant.ID, tenantCode, nil
}

func (s *Store) lookupTenantCodeByHostUID(ctx context.Context, db *gorm.DB, hostUID string) (string, error) {
	var host hostModel
	err := firstOrNotFound(
		db.WithContext(ctx).
			Preload("Tenant").
			Where(&hostModel{HostUID: hostUID}).
			Order("id"),
		&host,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return host.Tenant.TenantCode, nil
}

func (s *Store) upsertHost(ctx context.Context, db *gorm.DB, tenantID int64, hostUID string, req contracts.RegisterAgentRequest, now time.Time) (int64, error) {
	var host hostModel
	err := firstOrNotFound(
		db.WithContext(ctx).Where(&hostModel{HostUID: hostUID}).Order("id"),
		&host,
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		host = hostModel{
			TenantID:       tenantID,
			HostUID:        hostUID,
			Hostname:       req.Host.Hostname,
			PrimaryIP:      req.Host.PrimaryIP,
			OSType:         req.Host.OSType,
			Arch:           req.Host.Arch,
			Region:         req.Host.Region,
			AZ:             emptyStringPtr(req.Host.AZ),
			Env:            emptyStringPtr(req.Host.Env),
			Role:           emptyStringPtr(req.Host.Role),
			Status:         int(state.Up),
			RegisteredAt:   now,
			LastRegisterAt: now,
			UpdatedAt:      now,
		}
		if err := db.WithContext(ctx).Create(&host).Error; err != nil {
			return 0, err
		}
		return host.ID, nil
	}

	host.TenantID = tenantID
	host.Hostname = req.Host.Hostname
	host.PrimaryIP = req.Host.PrimaryIP
	host.OSType = req.Host.OSType
	host.Arch = req.Host.Arch
	host.Region = req.Host.Region
	host.AZ = emptyStringPtr(req.Host.AZ)
	host.Env = emptyStringPtr(req.Host.Env)
	host.Role = emptyStringPtr(req.Host.Role)
	host.Status = int(state.Up)
	host.LastRegisterAt = now
	host.UpdatedAt = now
	if err := db.WithContext(ctx).Save(&host).Error; err != nil {
		return 0, err
	}
	return host.ID, nil
}

func (s *Store) replaceLabels(ctx context.Context, db *gorm.DB, hostID int64, labels map[string]string) error {
	if err := db.WithContext(ctx).Where(&hostLabelModel{HostID: hostID}).Delete(&hostLabelModel{}).Error; err != nil {
		return err
	}
	if len(labels) == 0 {
		return nil
	}

	items := make([]hostLabelModel, 0, len(labels))
	for key, value := range labels {
		items = append(items, hostLabelModel{
			HostID:     hostID,
			LabelKey:   key,
			LabelValue: value,
		})
	}
	return db.WithContext(ctx).Create(&items).Error
}

func (s *Store) upsertAgentInstance(ctx context.Context, db *gorm.DB, hostID int64, req contracts.RegisterAgentRequest, now time.Time) (contracts.AgentConfig, error) {
	capabilities, err := json.Marshal(req.Agent.Capabilities)
	if err != nil {
		return contracts.AgentConfig{}, err
	}

	cfg := contracts.AgentConfig{
		ConfigVersion:        defaultConfigVersion,
		HeartbeatIntervalSec: defaultHeartbeatIntervalSec,
		MetricIntervalSec:    defaultMetricIntervalSec,
		StaticLabels:         cloneLabels(req.Host.Labels),
	}

	var agent agentInstanceModel
	err = firstOrNotFound(
		db.WithContext(ctx).Where(&agentInstanceModel{HostID: hostID, AgentID: req.Agent.AgentID}).Order("id"),
		&agent,
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return contracts.AgentConfig{}, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		agent = agentInstanceModel{
			HostID:               hostID,
			AgentID:              req.Agent.AgentID,
			Version:              req.Agent.Version,
			State:                int(state.Up),
			ConfigVersion:        cfg.ConfigVersion,
			HeartbeatIntervalSec: cfg.HeartbeatIntervalSec,
			MetricIntervalSec:    cfg.MetricIntervalSec,
			LastSeenAt:           &now,
			LastSeq:              0,
			Capabilities:         capabilities,
			UpdatedAt:            now,
		}
		if err := db.WithContext(ctx).Create(&agent).Error; err != nil {
			return contracts.AgentConfig{}, err
		}
		return cfg, nil
	}

	agent.Version = req.Agent.Version
	agent.State = int(state.Up)
	agent.ConfigVersion = cfg.ConfigVersion
	agent.HeartbeatIntervalSec = cfg.HeartbeatIntervalSec
	agent.MetricIntervalSec = cfg.MetricIntervalSec
	agent.LastSeenAt = &now
	agent.Capabilities = capabilities
	agent.UpdatedAt = now
	if err := db.WithContext(ctx).Save(&agent).Error; err != nil {
		return contracts.AgentConfig{}, err
	}

	return contracts.AgentConfig{
		ConfigVersion:        agent.ConfigVersion,
		HeartbeatIntervalSec: agent.HeartbeatIntervalSec,
		MetricIntervalSec:    agent.MetricIntervalSec,
		StaticLabels:         cloneLabels(req.Host.Labels),
	}, nil
}

func (s *Store) upsertRegisteredStatus(ctx context.Context, db *gorm.DB, hostID int64, now time.Time) error {
	var statusRow hostStatusCurrentModel
	err := firstOrNotFound(
		db.WithContext(ctx).Where(&hostStatusCurrentModel{HostID: hostID}),
		&statusRow,
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		statusRow = hostStatusCurrentModel{
			HostID:            hostID,
			AgentState:        int(state.Up),
			ReachabilityState: int(state.Unknown),
			ServiceState:      int(state.Unknown),
			OverallState:      int(state.Up),
			LastAgentSeenAt:   &now,
			Version:           1,
			UpdatedAt:         now,
		}
		return db.WithContext(ctx).Create(&statusRow).Error
	}

	statusRow.AgentState = int(state.Up)
	statusRow.OverallState = int(state.Up)
	statusRow.LastAgentSeenAt = &now
	statusRow.Version++
	statusRow.UpdatedAt = now
	return db.WithContext(ctx).Save(&statusRow).Error
}

func (s *Store) lookupAgentInstance(ctx context.Context, db *gorm.DB, hostUID string, agentID string) (int64, contracts.AgentConfig, error) {
	host, err := s.findHostByUID(ctx, db, hostUID, true)
	if err == nil {
		if cfg, ok, err := s.findAgentConfig(ctx, db, host.ID, agentID); err != nil {
			return 0, contracts.AgentConfig{}, err
		} else if ok {
			return host.ID, cfg, nil
		}
	}

	host, err = s.findHostByUID(ctx, db, hostUID, false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, contracts.AgentConfig{}, repository.ErrHostNotFound
		}
		return 0, contracts.AgentConfig{}, err
	}

	cfg, ok, err := s.findAgentConfig(ctx, db, host.ID, agentID)
	if err != nil {
		return 0, contracts.AgentConfig{}, err
	}
	if !ok {
		return 0, contracts.AgentConfig{}, repository.ErrHostNotFound
	}
	return host.ID, cfg, nil
}

func (s *Store) upsertHeartbeatStatus(ctx context.Context, db *gorm.DB, hostID int64, req contracts.HeartbeatRequest, now time.Time) error {
	var statusRow hostStatusCurrentModel
	err := firstOrNotFound(
		db.WithContext(ctx).Where(&hostStatusCurrentModel{HostID: hostID}),
		&statusRow,
	)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		statusRow = hostStatusCurrentModel{
			HostID:            hostID,
			AgentState:        int(state.Up),
			ReachabilityState: int(state.Unknown),
			ServiceState:      int(state.Unknown),
			OverallState:      int(state.Up),
			Version:           1,
		}
	}

	statusRow.AgentState = int(state.Up)
	statusRow.OverallState = int(state.Up)
	statusRow.CPUUsagePct = req.Digest.CPUUsagePct
	statusRow.MemUsedPct = req.Digest.MemUsedPct
	statusRow.MemAvailableBytes = req.Digest.MemAvailableBytes
	statusRow.SwapUsedPct = req.Digest.SwapUsedPct
	statusRow.DiskUsedPct = req.Digest.DiskUsedPct
	statusRow.DiskFreeBytes = req.Digest.DiskFreeBytes
	statusRow.DiskInodesUsedPct = req.Digest.DiskInodesUsedPct
	statusRow.DiskReadBPS = req.Digest.DiskReadBPS
	statusRow.DiskWriteBPS = req.Digest.DiskWriteBPS
	statusRow.DiskReadIOPS = req.Digest.DiskReadIOPS
	statusRow.DiskWriteIOPS = req.Digest.DiskWriteIOPS
	statusRow.Load1 = req.Digest.Load1
	statusRow.NetRxBPS = req.Digest.NetRxBPS
	statusRow.NetTxBPS = req.Digest.NetTxBPS
	statusRow.NetRxPacketsPS = req.Digest.NetRxPacketsPS
	statusRow.NetTxPacketsPS = req.Digest.NetTxPacketsPS
	statusRow.LastAgentSeenAt = &now
	statusRow.LastMetricAt = &now
	statusRow.UpdatedAt = now
	if statusRow.Version == 0 {
		statusRow.Version = 1
		return db.WithContext(ctx).Create(&statusRow).Error
	}
	statusRow.Version++
	return db.WithContext(ctx).Save(&statusRow).Error
}

func (s *Store) getHostByUID(ctx context.Context, db *gorm.DB, hostUID string, tenantCode string) (state.HostSnapshot, bool, error) {
	items, err := s.listHosts(ctx, db, hostUID, tenantCode)
	if err != nil {
		return state.HostSnapshot{}, false, err
	}
	if len(items) == 0 {
		return state.HostSnapshot{}, false, nil
	}
	return items[0], true, nil
}

func (s *Store) listHosts(ctx context.Context, db *gorm.DB, hostUID string, tenantCode string) ([]state.HostSnapshot, error) {
	query := db.WithContext(ctx).
		Preload("Tenant").
		Preload("CurrentStatus").
		Preload("Labels").
		Order("host_uid")
	if hostUID != "" {
		query = query.Where(&hostModel{HostUID: hostUID})
	}
	if tenantCode != "" {
		query = query.Where("tenant_id IN (?)", db.WithContext(ctx).Model(&tenantModel{}).Select("id").Where("tenant_code = ?", tenantCode))
	}

	var hosts []hostModel
	if err := query.Find(&hosts).Error; err != nil {
		return nil, err
	}

	items := make([]state.HostSnapshot, 0, len(hosts))
	for _, host := range hosts {
		items = append(items, hostSnapshotFromModel(host))
	}
	return items, nil
}

func (s *Store) findHostByUID(ctx context.Context, db *gorm.DB, hostUID string, limitToDefaultTenant bool) (hostModel, error) {
	query := db.WithContext(ctx).Where(&hostModel{HostUID: hostUID})
	if limitToDefaultTenant {
		query = query.Where(&hostModel{TenantID: s.defaultTenantID})
	}

	var host hostModel
	if err := firstOrNotFound(query.Order("id"), &host); err != nil {
		return hostModel{}, err
	}
	return host, nil
}

func (s *Store) findAgentConfig(ctx context.Context, db *gorm.DB, hostID int64, agentID string) (contracts.AgentConfig, bool, error) {
	var agent agentInstanceModel
	err := firstOrNotFound(
		db.WithContext(ctx).Where(&agentInstanceModel{HostID: hostID, AgentID: agentID}).Order("id"),
		&agent,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return contracts.AgentConfig{}, false, nil
		}
		return contracts.AgentConfig{}, false, err
	}

	return contracts.AgentConfig{
		ConfigVersion:        agent.ConfigVersion,
		HeartbeatIntervalSec: agent.HeartbeatIntervalSec,
		MetricIntervalSec:    agent.MetricIntervalSec,
	}, true, nil
}

func ensureTenant(ctx context.Context, db *gorm.DB, tenantCode string, tenantName string) (tenantModel, error) {
	tenant := tenantModel{}
	err := db.WithContext(ctx).
		Where(&tenantModel{TenantCode: tenantCode}).
		Assign(tenantModel{Name: tenantName, Status: 1}).
		FirstOrCreate(&tenant).Error
	if err != nil {
		return tenantModel{}, err
	}
	return tenant, nil
}

func firstOrNotFound[T any](query *gorm.DB, dest *T) error {
	result := query.Limit(1).Find(dest)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func hostSnapshotFromModel(host hostModel) state.HostSnapshot {
	item := state.HostSnapshot{
		HostUID:    host.HostUID,
		TenantCode: host.Tenant.TenantCode,
		Hostname:   host.Hostname,
		PrimaryIP:  host.PrimaryIP,
	}

	if host.CurrentStatus != nil {
		statusRow := host.CurrentStatus
		item.AgentState = state.Code(statusRow.AgentState)
		item.ReachabilityState = state.Code(statusRow.ReachabilityState)
		item.ServiceState = state.Code(statusRow.ServiceState)
		item.OverallState = state.Code(statusRow.OverallState)
		item.CPUUsagePct = statusRow.CPUUsagePct
		item.MemUsedPct = statusRow.MemUsedPct
		item.MemAvailableBytes = statusRow.MemAvailableBytes
		item.SwapUsedPct = statusRow.SwapUsedPct
		item.DiskUsedPct = statusRow.DiskUsedPct
		item.DiskFreeBytes = statusRow.DiskFreeBytes
		item.DiskInodesUsedPct = statusRow.DiskInodesUsedPct
		item.DiskReadBPS = statusRow.DiskReadBPS
		item.DiskWriteBPS = statusRow.DiskWriteBPS
		item.DiskReadIOPS = statusRow.DiskReadIOPS
		item.DiskWriteIOPS = statusRow.DiskWriteIOPS
		item.Load1 = statusRow.Load1
		item.NetRxBPS = statusRow.NetRxBPS
		item.NetTxBPS = statusRow.NetTxBPS
		item.NetRxPacketsPS = statusRow.NetRxPacketsPS
		item.NetTxPacketsPS = statusRow.NetTxPacketsPS
		item.OpenAlertCount = statusRow.OpenAlertCount
		item.Version = statusRow.Version
		if statusRow.LastAgentSeenAt != nil {
			item.LastAgentSeenAt = *statusRow.LastAgentSeenAt
		}
		if statusRow.LastMetricAt != nil {
			item.LastMetricAt = *statusRow.LastMetricAt
		}
		if statusRow.LastProbeAt != nil {
			item.LastProbeAt = *statusRow.LastProbeAt
		}
	}

	if len(host.Labels) > 0 {
		item.Labels = make(map[string]string, len(host.Labels))
		for _, label := range host.Labels {
			item.Labels[label.LabelKey] = label.LabelValue
		}
	}

	return item
}

func emptyStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func cloneLabels(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
