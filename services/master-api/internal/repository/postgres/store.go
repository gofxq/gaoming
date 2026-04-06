package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/state"
	"github.com/gofxq/gaoming/services/master-api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
	db              *pgxpool.Pool
	defaultTenantID int64
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func NewStore(ctx context.Context, db *pgxpool.Pool, cfg Config) (*Store, error) {
	tenantCode := cfg.TenantCode
	if tenantCode == "" {
		tenantCode = "default"
	}
	tenantName := cfg.TenantName
	if tenantName == "" {
		tenantName = "Default Tenant"
	}

	var tenantID int64
	if err := db.QueryRow(ctx, `
INSERT INTO tenants (tenant_code, name, status)
VALUES ($1, $2, 1)
ON CONFLICT (tenant_code) DO UPDATE SET
	name = EXCLUDED.name,
	updated_at = now()
RETURNING id
`, tenantCode, tenantName).Scan(&tenantID); err != nil {
		return nil, fmt.Errorf("ensure tenant: %w", err)
	}

	return &Store{db: db, defaultTenantID: tenantID}, nil
}

func (s *Store) RegisterAgent(ctx context.Context, req contracts.RegisterAgentRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}
	defer tx.Rollback(ctx)

	hostUID := req.Host.HostUID
	if hostUID == "" {
		hostUID = ids.New("host")
	}
	tenantID, tenantCode, err := s.resolveTenantForRegister(ctx, tx, hostUID, req.Host.TenantCode)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}

	hostID, err := s.upsertHost(ctx, tx, tenantID, hostUID, req, now)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}
	if err := s.replaceLabels(ctx, tx, hostID, req.Host.Labels); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}

	cfg, err := s.upsertAgentInstance(ctx, tx, hostID, req, now)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}
	if err := s.upsertRegisteredStatus(ctx, tx, hostID, now); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}

	snapshot, ok, err := s.getHostByUID(ctx, tx, hostUID)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}
	if !ok {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", repository.ErrHostNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, "", err
	}
	return snapshot, cfg, tenantCode, nil
}

func (s *Store) Heartbeat(ctx context.Context, req contracts.HeartbeatRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}
	defer tx.Rollback(ctx)

	hostID, cfg, err := s.lookupAgentInstance(ctx, tx, req.HostUID, req.AgentID)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE agent_instances
SET state = $1,
	last_seen_at = $2,
	last_seq = $3,
	updated_at = $2
WHERE host_id = $4 AND agent_id = $5
`, int(state.Up), now, req.Seq, hostID, req.AgentID); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}

	if err := s.upsertHeartbeatStatus(ctx, tx, hostID, req, now); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}

	snapshot, ok, err := s.getHostByUID(ctx, tx, req.HostUID)
	if err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}
	if !ok {
		return state.HostSnapshot{}, contracts.AgentConfig{}, repository.ErrHostNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return state.HostSnapshot{}, contracts.AgentConfig{}, err
	}
	return snapshot, cfg, nil
}

func (s *Store) ListHosts(ctx context.Context) ([]state.HostSnapshot, error) {
	return s.listHosts(ctx, s.db, "", nil)
}

func (s *Store) GetHost(ctx context.Context, hostUID string) (state.HostSnapshot, bool, error) {
	return s.getHostByUID(ctx, s.db, hostUID)
}

func (s *Store) ReconcileOffline(ctx context.Context, now time.Time) ([]state.HostSnapshot, error) {
	rows, err := s.db.Query(ctx, `
SELECT h.host_uid
FROM hosts h
JOIN host_status_current hsc ON hsc.host_id = h.id
WHERE hsc.last_agent_seen_at IS NOT NULL
  AND hsc.last_agent_seen_at < $1
  AND NOT (hsc.agent_state = $2 AND hsc.overall_state = $2)
ORDER BY h.host_uid
`, now.Add(-15*time.Second), int(state.Offline))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hostUIDs := make([]string, 0)
	for rows.Next() {
		var hostUID string
		if err := rows.Scan(&hostUID); err != nil {
			return nil, err
		}
		hostUIDs = append(hostUIDs, hostUID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(hostUIDs) == 0 {
		return nil, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	changed := make([]state.HostSnapshot, 0, len(hostUIDs))
	for _, hostUID := range hostUIDs {
		if _, err := tx.Exec(ctx, `
UPDATE host_status_current
SET agent_state = $1,
	reachability_state = $1,
	overall_state = $1,
	version = version + 1,
	updated_at = $2
WHERE host_id = (
	SELECT id FROM hosts WHERE host_uid = $3
)
`, int(state.Offline), now, hostUID); err != nil {
			return nil, err
		}
		snapshot, ok, err := s.getHostByUID(ctx, tx, hostUID)
		if err != nil {
			return nil, err
		}
		if ok {
			changed = append(changed, snapshot)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return changed, nil
}

func (s *Store) CreateMaintenance(ctx context.Context, req contracts.CreateMaintenanceWindowRequest) (repository.MaintenanceWindow, error) {
	var id int64
	if err := s.db.QueryRow(ctx, `
INSERT INTO maintenance_windows (
	tenant_id, title, scope_type, scope_ref, start_at, end_at, created_by, reason, enabled
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), true)
RETURNING id
`, s.defaultTenantID, req.Title, req.ScopeType, req.ScopeRef, req.StartAt, req.EndAt, req.CreatedBy, req.Reason).Scan(&id); err != nil {
		return repository.MaintenanceWindow{}, err
	}

	return repository.MaintenanceWindow{
		ID:        strconv.FormatInt(id, 10),
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
	if id, err := strconv.ParseInt(alertID, 10, 64); err == nil {
		if _, err := s.db.Exec(ctx, `
UPDATE alert_events
SET acked_by = $1,
	acked_at = $2
WHERE id = $3 AND tenant_id = $4
`, ackedBy, now, id, s.defaultTenantID); err != nil {
			return err
		}
		return nil
	}

	_, err := s.db.Exec(ctx, `
UPDATE alert_events
SET acked_by = $1,
	acked_at = $2
WHERE fingerprint = $3 AND tenant_id = $4
`, ackedBy, now, alertID, s.defaultTenantID)
	return err
}

func (s *Store) resolveTenantForRegister(ctx context.Context, q querier, hostUID string, tenantCode string) (int64, string, error) {
	if tenantCode == "" {
		existingTenantCode, err := s.lookupTenantCodeByHostUID(ctx, q, hostUID)
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

	var tenantID int64
	if err := q.QueryRow(ctx, `
INSERT INTO tenants (tenant_code, name, status)
VALUES ($1, $2, 1)
ON CONFLICT (tenant_code) DO UPDATE SET
	updated_at = now()
RETURNING id
`, tenantCode, tenantCode).Scan(&tenantID); err != nil {
		return 0, "", err
	}
	return tenantID, tenantCode, nil
}

func (s *Store) lookupTenantCodeByHostUID(ctx context.Context, q querier, hostUID string) (string, error) {
	var tenantCode string
	err := q.QueryRow(ctx, `
SELECT t.tenant_code
FROM hosts h
JOIN tenants t ON t.id = h.tenant_id
WHERE h.host_uid = $1
`, hostUID).Scan(&tenantCode)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return tenantCode, nil
}

func (s *Store) upsertHost(ctx context.Context, q querier, tenantID int64, hostUID string, req contracts.RegisterAgentRequest, now time.Time) (int64, error) {
	var hostID int64
	err := q.QueryRow(ctx, `
INSERT INTO hosts (
	tenant_id, host_uid, hostname, primary_ip, os_type, arch, region, az, env, role,
	status, registered_at, last_register_at, updated_at
)
VALUES (
	$1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
	$11, $12, $12, $12
)
ON CONFLICT (host_uid) DO UPDATE SET
	tenant_id = EXCLUDED.tenant_id,
	hostname = EXCLUDED.hostname,
	primary_ip = EXCLUDED.primary_ip,
	os_type = EXCLUDED.os_type,
	arch = EXCLUDED.arch,
	region = EXCLUDED.region,
	az = EXCLUDED.az,
	env = EXCLUDED.env,
	role = EXCLUDED.role,
	status = EXCLUDED.status,
	last_register_at = EXCLUDED.last_register_at,
	updated_at = EXCLUDED.updated_at
RETURNING id
`, tenantID, hostUID, req.Host.Hostname, req.Host.PrimaryIP, req.Host.OSType, req.Host.Arch, req.Host.Region, req.Host.AZ, req.Host.Env, req.Host.Role, int(state.Up), now).Scan(&hostID)
	return hostID, err
}

func (s *Store) replaceLabels(ctx context.Context, q querier, hostID int64, labels map[string]string) error {
	if _, err := q.Exec(ctx, `DELETE FROM host_labels WHERE host_id = $1`, hostID); err != nil {
		return err
	}
	for key, value := range labels {
		if _, err := q.Exec(ctx, `
INSERT INTO host_labels (host_id, label_key, label_value)
VALUES ($1, $2, $3)
`, hostID, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) upsertAgentInstance(ctx context.Context, q querier, hostID int64, req contracts.RegisterAgentRequest, now time.Time) (contracts.AgentConfig, error) {
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

	if err := q.QueryRow(ctx, `
INSERT INTO agent_instances (
	host_id, agent_id, version, state, config_version, heartbeat_interval_sec, metric_interval_sec,
	last_seen_at, last_seq, capabilities, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9, $8)
ON CONFLICT (host_id, agent_id) DO UPDATE SET
	version = EXCLUDED.version,
	state = EXCLUDED.state,
	config_version = EXCLUDED.config_version,
	heartbeat_interval_sec = EXCLUDED.heartbeat_interval_sec,
	metric_interval_sec = EXCLUDED.metric_interval_sec,
	last_seen_at = EXCLUDED.last_seen_at,
	capabilities = EXCLUDED.capabilities,
	updated_at = EXCLUDED.updated_at
RETURNING config_version, heartbeat_interval_sec, metric_interval_sec
`, hostID, req.Agent.AgentID, req.Agent.Version, int(state.Up), cfg.ConfigVersion, cfg.HeartbeatIntervalSec, cfg.MetricIntervalSec, now, capabilities).Scan(
		&cfg.ConfigVersion,
		&cfg.HeartbeatIntervalSec,
		&cfg.MetricIntervalSec,
	); err != nil {
		return contracts.AgentConfig{}, err
	}

	return cfg, nil
}

func (s *Store) upsertRegisteredStatus(ctx context.Context, q querier, hostID int64, now time.Time) error {
	_, err := q.Exec(ctx, `
INSERT INTO host_status_current (
	host_id, agent_state, reachability_state, service_state, overall_state,
	last_agent_seen_at, version, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, 1, $6)
ON CONFLICT (host_id) DO UPDATE SET
	agent_state = EXCLUDED.agent_state,
	overall_state = EXCLUDED.overall_state,
	last_agent_seen_at = EXCLUDED.last_agent_seen_at,
	version = host_status_current.version + 1,
	updated_at = EXCLUDED.updated_at
`, hostID, int(state.Up), int(state.Unknown), int(state.Unknown), int(state.Up), now)
	return err
}

func (s *Store) lookupAgentInstance(ctx context.Context, q querier, hostUID string, agentID string) (int64, contracts.AgentConfig, error) {
	cfg := contracts.AgentConfig{}
	var hostID int64
	err := q.QueryRow(ctx, `
SELECT h.id, ai.config_version, ai.heartbeat_interval_sec, ai.metric_interval_sec
FROM hosts h
JOIN agent_instances ai ON ai.host_id = h.id
WHERE h.tenant_id = $1 AND h.host_uid = $2 AND ai.agent_id = $3
`, s.defaultTenantID, hostUID, agentID).Scan(&hostID, &cfg.ConfigVersion, &cfg.HeartbeatIntervalSec, &cfg.MetricIntervalSec)
	if err != nil {
		err = q.QueryRow(ctx, `
SELECT h.id, ai.config_version, ai.heartbeat_interval_sec, ai.metric_interval_sec
FROM hosts h
JOIN agent_instances ai ON ai.host_id = h.id
WHERE h.host_uid = $1 AND ai.agent_id = $2
`, hostUID, agentID).Scan(&hostID, &cfg.ConfigVersion, &cfg.HeartbeatIntervalSec, &cfg.MetricIntervalSec)
		if err != nil {
			if err == pgx.ErrNoRows {
				return 0, contracts.AgentConfig{}, repository.ErrHostNotFound
			}
			return 0, contracts.AgentConfig{}, err
		}
	}
	return hostID, cfg, nil
}

func (s *Store) upsertHeartbeatStatus(ctx context.Context, q querier, hostID int64, req contracts.HeartbeatRequest, now time.Time) error {
	_, err := q.Exec(ctx, `
INSERT INTO host_status_current (
	host_id, agent_state, reachability_state, service_state, overall_state,
	cpu_usage_pct, mem_used_pct, mem_available_bytes, swap_used_pct, disk_used_pct,
	disk_free_bytes, disk_inodes_used_pct, disk_read_bps, disk_write_bps, disk_read_iops,
	disk_write_iops, load1, net_rx_bps, net_tx_bps, net_rx_packets_ps, net_tx_packets_ps,
	last_agent_seen_at, last_metric_at, version, updated_at
)
VALUES (
	$1, $2, $3, $4, $5,
	$6, $7, $8, $9, $10,
	$11, $12, $13, $14, $15,
	$16, $17, $18, $19, $20, $21,
	$22, $22, 1, $22
)
ON CONFLICT (host_id) DO UPDATE SET
	agent_state = EXCLUDED.agent_state,
	overall_state = EXCLUDED.overall_state,
	cpu_usage_pct = EXCLUDED.cpu_usage_pct,
	mem_used_pct = EXCLUDED.mem_used_pct,
	mem_available_bytes = EXCLUDED.mem_available_bytes,
	swap_used_pct = EXCLUDED.swap_used_pct,
	disk_used_pct = EXCLUDED.disk_used_pct,
	disk_free_bytes = EXCLUDED.disk_free_bytes,
	disk_inodes_used_pct = EXCLUDED.disk_inodes_used_pct,
	disk_read_bps = EXCLUDED.disk_read_bps,
	disk_write_bps = EXCLUDED.disk_write_bps,
	disk_read_iops = EXCLUDED.disk_read_iops,
	disk_write_iops = EXCLUDED.disk_write_iops,
	load1 = EXCLUDED.load1,
	net_rx_bps = EXCLUDED.net_rx_bps,
	net_tx_bps = EXCLUDED.net_tx_bps,
	net_rx_packets_ps = EXCLUDED.net_rx_packets_ps,
	net_tx_packets_ps = EXCLUDED.net_tx_packets_ps,
	last_agent_seen_at = EXCLUDED.last_agent_seen_at,
	last_metric_at = EXCLUDED.last_metric_at,
	version = host_status_current.version + 1,
	updated_at = EXCLUDED.updated_at
`, hostID, int(state.Up), int(state.Unknown), int(state.Unknown), int(state.Up),
		req.Digest.CPUUsagePct, req.Digest.MemUsedPct, req.Digest.MemAvailableBytes, req.Digest.SwapUsedPct, req.Digest.DiskUsedPct,
		req.Digest.DiskFreeBytes, req.Digest.DiskInodesUsedPct, req.Digest.DiskReadBPS, req.Digest.DiskWriteBPS, req.Digest.DiskReadIOPS,
		req.Digest.DiskWriteIOPS, req.Digest.Load1, req.Digest.NetRxBPS, req.Digest.NetTxBPS, req.Digest.NetRxPacketsPS, req.Digest.NetTxPacketsPS, now)
	return err
}

func (s *Store) getHostByUID(ctx context.Context, q querier, hostUID string) (state.HostSnapshot, bool, error) {
	items, err := s.listHosts(ctx, q, " WHERE h.host_uid = $1", []any{hostUID})
	if err != nil {
		return state.HostSnapshot{}, false, err
	}
	if len(items) == 0 {
		return state.HostSnapshot{}, false, nil
	}
	return items[0], true, nil
}

func (s *Store) listHosts(ctx context.Context, q querier, whereClause string, args []any) ([]state.HostSnapshot, error) {
	if whereClause == "" {
		whereClause = ""
	}
	query := `
SELECT
	h.host_uid,
	h.hostname,
	host(h.primary_ip)::text,
	COALESCE(hsc.agent_state, 0),
	COALESCE(hsc.reachability_state, 0),
	COALESCE(hsc.service_state, 0),
	COALESCE(hsc.overall_state, 0),
	COALESCE(hsc.cpu_usage_pct::float8, 0),
	COALESCE(hsc.mem_used_pct::float8, 0),
	COALESCE(hsc.mem_available_bytes, 0),
	COALESCE(hsc.swap_used_pct::float8, 0),
	COALESCE(hsc.disk_used_pct::float8, 0),
	COALESCE(hsc.disk_free_bytes, 0),
	COALESCE(hsc.disk_inodes_used_pct::float8, 0),
	COALESCE(hsc.disk_read_bps, 0),
	COALESCE(hsc.disk_write_bps, 0),
	COALESCE(hsc.disk_read_iops, 0),
	COALESCE(hsc.disk_write_iops, 0),
	COALESCE(hsc.load1::float8, 0),
	COALESCE(hsc.net_rx_bps, 0),
	COALESCE(hsc.net_tx_bps, 0),
	COALESCE(hsc.net_rx_packets_ps, 0),
	COALESCE(hsc.net_tx_packets_ps, 0),
	hsc.last_agent_seen_at,
	hsc.last_metric_at,
	hsc.last_probe_at,
	COALESCE(hsc.open_alert_count, 0),
	COALESCE((
		SELECT jsonb_object_agg(hl.label_key, hl.label_value)
		FROM host_labels hl
		WHERE hl.host_id = h.id
	), '{}'::jsonb),
	COALESCE(hsc.version, 0)
FROM hosts h
LEFT JOIN host_status_current hsc ON hsc.host_id = h.id
` + whereClause + `
ORDER BY h.host_uid
`
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]state.HostSnapshot, 0)
	for rows.Next() {
		item, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func scanSnapshot(rows pgx.Rows) (state.HostSnapshot, error) {
	var item state.HostSnapshot
	var agentState int
	var reachabilityState int
	var serviceState int
	var overallState int
	var labelsRaw []byte
	var lastAgentSeenAt sql.NullTime
	var lastMetricAt sql.NullTime
	var lastProbeAt sql.NullTime

	if err := rows.Scan(
		&item.HostUID,
		&item.Hostname,
		&item.PrimaryIP,
		&agentState,
		&reachabilityState,
		&serviceState,
		&overallState,
		&item.CPUUsagePct,
		&item.MemUsedPct,
		&item.MemAvailableBytes,
		&item.SwapUsedPct,
		&item.DiskUsedPct,
		&item.DiskFreeBytes,
		&item.DiskInodesUsedPct,
		&item.DiskReadBPS,
		&item.DiskWriteBPS,
		&item.DiskReadIOPS,
		&item.DiskWriteIOPS,
		&item.Load1,
		&item.NetRxBPS,
		&item.NetTxBPS,
		&item.NetRxPacketsPS,
		&item.NetTxPacketsPS,
		&lastAgentSeenAt,
		&lastMetricAt,
		&lastProbeAt,
		&item.OpenAlertCount,
		&labelsRaw,
		&item.Version,
	); err != nil {
		return state.HostSnapshot{}, err
	}

	item.AgentState = state.Code(agentState)
	item.ReachabilityState = state.Code(reachabilityState)
	item.ServiceState = state.Code(serviceState)
	item.OverallState = state.Code(overallState)
	if lastAgentSeenAt.Valid {
		item.LastAgentSeenAt = lastAgentSeenAt.Time
	}
	if lastMetricAt.Valid {
		item.LastMetricAt = lastMetricAt.Time
	}
	if lastProbeAt.Valid {
		item.LastProbeAt = lastProbeAt.Time
	}
	if len(labelsRaw) > 0 {
		var labels map[string]string
		if err := json.Unmarshal(labelsRaw, &labels); err != nil {
			return state.HostSnapshot{}, err
		}
		if len(labels) > 0 {
			item.Labels = labels
		}
	}
	return item, nil
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
