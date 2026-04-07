package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/state"
)

var ErrHostNotFound = errors.New("host not found")

type MaintenanceWindow struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	ScopeType string    `json:"scope_type"`
	ScopeRef  string    `json:"scope_ref"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	CreatedBy string    `json:"created_by"`
	Reason    string    `json:"reason,omitempty"`
}

type HostEventType string

const (
	HostEventUpsert HostEventType = "host_upsert"
	HostEventDelete HostEventType = "host_delete"
)

type HostEvent struct {
	Type     HostEventType       `json:"type"`
	HostUID  string              `json:"host_uid"`
	Snapshot *state.HostSnapshot `json:"snapshot,omitempty"`
}

type HostStateStore interface {
	AllocateTenant(ctx context.Context) (string, error)
	RegisterAgent(ctx context.Context, req contracts.RegisterAgentRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, string, error)
	Heartbeat(ctx context.Context, req contracts.HeartbeatRequest, now time.Time) (state.HostSnapshot, contracts.AgentConfig, error)
	ListHosts(ctx context.Context, tenantCode string) ([]state.HostSnapshot, error)
	GetHost(ctx context.Context, hostUID string, tenantCode string) (state.HostSnapshot, bool, error)
	ReconcileOffline(ctx context.Context, now time.Time) ([]state.HostSnapshot, error)
}

type MetricWindowStore interface {
	AppendHeartbeatMetrics(ctx context.Context, hostUID string, now time.Time, digest contracts.AgentDigest) error
	GetHostMetricHistory(ctx context.Context, hostUID string) (map[state.MetricKey][]state.MetricPoint, error)
	GetAllHostMetricHistory(ctx context.Context, hostUIDs []string) (map[string]map[state.MetricKey][]state.MetricPoint, error)
}

type EventBus interface {
	PublishHostUpsert(ctx context.Context, snapshot state.HostSnapshot) error
	PublishHostDelete(ctx context.Context, hostUID string) error
	SubscribeHostEvents(ctx context.Context) (<-chan HostEvent, error)
}

type OperationsStore interface {
	CreateMaintenance(ctx context.Context, req contracts.CreateMaintenanceWindowRequest) (MaintenanceWindow, error)
	AckAlert(ctx context.Context, alertID string, ackedBy string, now time.Time) error
}

func DigestMetricValues(digest contracts.AgentDigest) map[state.MetricKey]float64 {
	return map[state.MetricKey]float64{
		state.MetricCPUUsagePct:       digest.CPUUsagePct,
		state.MetricMemUsedPct:        digest.MemUsedPct,
		state.MetricMemAvailableBytes: float64(digest.MemAvailableBytes),
		state.MetricSwapUsedPct:       digest.SwapUsedPct,
		state.MetricDiskUsedPct:       digest.DiskUsedPct,
		state.MetricDiskFreeBytes:     float64(digest.DiskFreeBytes),
		state.MetricDiskInodesUsedPct: digest.DiskInodesUsedPct,
		state.MetricDiskReadBPS:       float64(digest.DiskReadBPS),
		state.MetricDiskWriteBPS:      float64(digest.DiskWriteBPS),
		state.MetricDiskReadIOPS:      float64(digest.DiskReadIOPS),
		state.MetricDiskWriteIOPS:     float64(digest.DiskWriteIOPS),
		state.MetricLoad1:             digest.Load1,
		state.MetricNetRxBPS:          float64(digest.NetRxBPS),
		state.MetricNetTxBPS:          float64(digest.NetTxBPS),
		state.MetricNetRxPacketsPS:    float64(digest.NetRxPacketsPS),
		state.MetricNetTxPacketsPS:    float64(digest.NetTxPacketsPS),
	}
}

func MetricKeys() []state.MetricKey {
	return []state.MetricKey{
		state.MetricCPUUsagePct,
		state.MetricMemUsedPct,
		state.MetricMemAvailableBytes,
		state.MetricSwapUsedPct,
		state.MetricDiskUsedPct,
		state.MetricDiskFreeBytes,
		state.MetricDiskInodesUsedPct,
		state.MetricDiskReadBPS,
		state.MetricDiskWriteBPS,
		state.MetricDiskReadIOPS,
		state.MetricDiskWriteIOPS,
		state.MetricLoad1,
		state.MetricNetRxBPS,
		state.MetricNetTxBPS,
		state.MetricNetRxPacketsPS,
		state.MetricNetTxPacketsPS,
	}
}
