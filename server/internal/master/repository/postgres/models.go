package postgres

import "time"

type tenantModel struct {
	ID         int64     `gorm:"column:id;primaryKey"`
	TenantCode string    `gorm:"column:tenant_code"`
	Name       string    `gorm:"column:name"`
	Status     int       `gorm:"column:status"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (tenantModel) TableName() string {
	return "tenants"
}

type hostModel struct {
	ID             int64                   `gorm:"column:id;primaryKey"`
	TenantID       int64                   `gorm:"column:tenant_id"`
	HostUID        string                  `gorm:"column:host_uid"`
	Hostname       string                  `gorm:"column:hostname"`
	PrimaryIP      string                  `gorm:"column:primary_ip"`
	OSType         string                  `gorm:"column:os_type"`
	Arch           string                  `gorm:"column:arch"`
	Region         string                  `gorm:"column:region"`
	AZ             *string                 `gorm:"column:az"`
	Env            *string                 `gorm:"column:env"`
	Role           *string                 `gorm:"column:role"`
	Status         int                     `gorm:"column:status"`
	RegisteredAt   time.Time               `gorm:"column:registered_at"`
	LastRegisterAt time.Time               `gorm:"column:last_register_at"`
	UpdatedAt      time.Time               `gorm:"column:updated_at"`
	Tenant         tenantModel             `gorm:"foreignKey:TenantID;references:ID"`
	CurrentStatus  *hostStatusCurrentModel `gorm:"foreignKey:HostID;references:ID"`
	Labels         []hostLabelModel        `gorm:"foreignKey:HostID;references:ID"`
}

func (hostModel) TableName() string {
	return "hosts"
}

type hostLabelModel struct {
	HostID     int64  `gorm:"column:host_id;primaryKey"`
	LabelKey   string `gorm:"column:label_key;primaryKey"`
	LabelValue string `gorm:"column:label_value"`
}

func (hostLabelModel) TableName() string {
	return "host_labels"
}

type agentInstanceModel struct {
	ID                   int64      `gorm:"column:id;primaryKey"`
	HostID               int64      `gorm:"column:host_id"`
	AgentID              string     `gorm:"column:agent_id"`
	Version              string     `gorm:"column:version"`
	State                int        `gorm:"column:state"`
	ConfigVersion        int64      `gorm:"column:config_version"`
	HeartbeatIntervalSec int        `gorm:"column:heartbeat_interval_sec"`
	MetricIntervalSec    int        `gorm:"column:metric_interval_sec"`
	LastSeenAt           *time.Time `gorm:"column:last_seen_at"`
	LastSeq              int64      `gorm:"column:last_seq"`
	Capabilities         []byte     `gorm:"column:capabilities"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (agentInstanceModel) TableName() string {
	return "agent_instances"
}

type hostStatusCurrentModel struct {
	HostID            int64      `gorm:"column:host_id;primaryKey"`
	AgentState        int        `gorm:"column:agent_state"`
	ReachabilityState int        `gorm:"column:reachability_state"`
	ServiceState      int        `gorm:"column:service_state"`
	OverallState      int        `gorm:"column:overall_state"`
	CPUUsagePct       float64    `gorm:"column:cpu_usage_pct"`
	MemUsedPct        float64    `gorm:"column:mem_used_pct"`
	MemAvailableBytes int64      `gorm:"column:mem_available_bytes"`
	SwapUsedPct       float64    `gorm:"column:swap_used_pct"`
	DiskUsedPct       float64    `gorm:"column:disk_used_pct"`
	DiskFreeBytes     int64      `gorm:"column:disk_free_bytes"`
	DiskInodesUsedPct float64    `gorm:"column:disk_inodes_used_pct"`
	DiskReadBPS       int64      `gorm:"column:disk_read_bps"`
	DiskWriteBPS      int64      `gorm:"column:disk_write_bps"`
	DiskReadIOPS      int64      `gorm:"column:disk_read_iops"`
	DiskWriteIOPS     int64      `gorm:"column:disk_write_iops"`
	Load1             float64    `gorm:"column:load1"`
	NetRxBPS          int64      `gorm:"column:net_rx_bps"`
	NetTxBPS          int64      `gorm:"column:net_tx_bps"`
	NetRxPacketsPS    int64      `gorm:"column:net_rx_packets_ps"`
	NetTxPacketsPS    int64      `gorm:"column:net_tx_packets_ps"`
	LastAgentSeenAt   *time.Time `gorm:"column:last_agent_seen_at"`
	LastMetricAt      *time.Time `gorm:"column:last_metric_at"`
	LastProbeAt       *time.Time `gorm:"column:last_probe_at"`
	OpenAlertCount    int        `gorm:"column:open_alert_count"`
	Version           int64      `gorm:"column:version"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
	Host              hostModel  `gorm:"foreignKey:HostID;references:ID"`
}

func (hostStatusCurrentModel) TableName() string {
	return "host_status_current"
}

type maintenanceWindowModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	TenantID  int64     `gorm:"column:tenant_id"`
	Title     string    `gorm:"column:title"`
	ScopeType string    `gorm:"column:scope_type"`
	ScopeRef  string    `gorm:"column:scope_ref"`
	StartAt   time.Time `gorm:"column:start_at"`
	EndAt     time.Time `gorm:"column:end_at"`
	CreatedBy string    `gorm:"column:created_by"`
	Reason    *string   `gorm:"column:reason"`
	Enabled   bool      `gorm:"column:enabled"`
}

func (maintenanceWindowModel) TableName() string {
	return "maintenance_windows"
}

type alertEventModel struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	TenantID    int64      `gorm:"column:tenant_id"`
	Fingerprint string     `gorm:"column:fingerprint"`
	AckedBy     string     `gorm:"column:acked_by"`
	AckedAt     *time.Time `gorm:"column:acked_at"`
}

func (alertEventModel) TableName() string {
	return "alert_events"
}
