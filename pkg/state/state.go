package state

import "time"

type Code uint8

const (
	Unknown Code = iota
	Up
	Warning
	Critical
	Offline
	Maintenance
	Disabled
)

func (c Code) String() string {
	switch c {
	case Up:
		return "UP"
	case Warning:
		return "WARNING"
	case Critical:
		return "CRITICAL"
	case Offline:
		return "OFFLINE"
	case Maintenance:
		return "MAINTENANCE"
	case Disabled:
		return "DISABLED"
	default:
		return "UNKNOWN"
	}
}

type HostSnapshot struct {
	HostUID           string            `json:"host_uid"`
	Hostname          string            `json:"hostname"`
	PrimaryIP         string            `json:"primary_ip"`
	GroupID           string            `json:"group_id,omitempty"`
	AgentState        Code              `json:"agent_state"`
	ReachabilityState Code              `json:"reachability_state"`
	ServiceState      Code              `json:"service_state"`
	OverallState      Code              `json:"overall_state"`
	CPUUsagePct       float64           `json:"cpu_usage_pct"`
	MemUsedPct        float64           `json:"mem_used_pct"`
	DiskUsedPct       float64           `json:"disk_used_pct"`
	DiskReadBPS       int64             `json:"disk_read_bps"`
	DiskWriteBPS      int64             `json:"disk_write_bps"`
	Load1             float64           `json:"load1"`
	NetRxBPS          int64             `json:"net_rx_bps"`
	NetTxBPS          int64             `json:"net_tx_bps"`
	LastAgentSeenAt   time.Time         `json:"last_agent_seen_at"`
	LastMetricAt      time.Time         `json:"last_metric_at"`
	LastProbeAt       time.Time         `json:"last_probe_at"`
	OpenAlertCount    int               `json:"open_alert_count"`
	Labels            map[string]string `json:"labels,omitempty"`
	Version           int64             `json:"version"`
}

type MetricKey string

const (
	MetricCPUUsagePct  MetricKey = "cpu_usage_pct"
	MetricMemUsedPct   MetricKey = "mem_used_pct"
	MetricDiskUsedPct  MetricKey = "disk_used_pct"
	MetricDiskReadBPS  MetricKey = "disk_read_bps"
	MetricDiskWriteBPS MetricKey = "disk_write_bps"
	MetricLoad1        MetricKey = "load1"
	MetricNetRxBPS     MetricKey = "net_rx_bps"
	MetricNetTxBPS     MetricKey = "net_tx_bps"
)

type MetricPoint struct {
	TS    time.Time `json:"ts"`
	Value float64   `json:"value"`
}
