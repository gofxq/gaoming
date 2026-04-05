package contracts

import "time"

type AgentConfig struct {
	ConfigVersion        int64             `json:"config_version"`
	HeartbeatIntervalSec int               `json:"heartbeat_interval_sec"`
	MetricIntervalSec    int               `json:"metric_interval_sec"`
	StaticLabels         map[string]string `json:"static_labels,omitempty"`
}

type HostIdentity struct {
	HostUID    string            `json:"host_uid,omitempty"`
	TenantCode string            `json:"tenant_code,omitempty"`
	Hostname   string            `json:"hostname"`
	PrimaryIP  string            `json:"primary_ip"`
	IPs        []string          `json:"ips,omitempty"`
	OSType     string            `json:"os_type"`
	Arch       string            `json:"arch"`
	Region     string            `json:"region"`
	AZ         string            `json:"az,omitempty"`
	Env        string            `json:"env,omitempty"`
	Role       string            `json:"role,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type AgentMetadata struct {
	AgentID      string    `json:"agent_id"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities,omitempty"`
	BootTime     time.Time `json:"boot_time"`
}

type AgentDigest struct {
	CPUUsagePct        float64 `json:"cpu_usage_pct"`
	MemUsedPct         float64 `json:"mem_used_pct"`
	DiskUsedPct        float64 `json:"disk_used_pct"`
	DiskReadBPS        int64   `json:"disk_read_bps"`
	DiskWriteBPS       int64   `json:"disk_write_bps"`
	Load1              float64 `json:"load1"`
	NetRxBPS           int64   `json:"net_rx_bps"`
	NetTxBPS           int64   `json:"net_tx_bps"`
	QueueDepth         int64   `json:"queue_depth"`
	LastMetricBatchSeq int64   `json:"last_metric_batch_seq"`
}

type RegisterAgentRequest struct {
	Host  HostIdentity  `json:"host"`
	Agent AgentMetadata `json:"agent"`
}

type RegisterAgentResponse struct {
	RequestID  string      `json:"request_id"`
	Message    string      `json:"message"`
	HostUID    string      `json:"host_uid"`
	TenantCode string      `json:"tenant_code"`
	Config     AgentConfig `json:"config"`
}

type HeartbeatRequest struct {
	HostUID string      `json:"host_uid"`
	AgentID string      `json:"agent_id"`
	Seq     int64       `json:"seq"`
	TS      time.Time   `json:"ts"`
	Digest  AgentDigest `json:"digest"`
}

type HeartbeatResponse struct {
	RequestID                string `json:"request_id"`
	Message                  string `json:"message"`
	NextHeartbeatIntervalSec int    `json:"next_heartbeat_interval_sec"`
	DesiredConfigVersion     int64  `json:"desired_config_version"`
}

type MetricPoint struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	TS     time.Time         `json:"ts"`
	Labels map[string]string `json:"labels,omitempty"`
}

type EventRecord struct {
	Type     string            `json:"type"`
	Severity string            `json:"severity"`
	Message  string            `json:"message"`
	TS       time.Time         `json:"ts"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type PushMetricBatchRequest struct {
	HostUID     string        `json:"host_uid"`
	AgentID     string        `json:"agent_id"`
	BatchSeq    int64         `json:"batch_seq"`
	CollectedAt time.Time     `json:"collected_at"`
	Points      []MetricPoint `json:"points"`
}

type PushEventBatchRequest struct {
	HostUID  string        `json:"host_uid"`
	AgentID  string        `json:"agent_id"`
	BatchSeq int64         `json:"batch_seq"`
	Events   []EventRecord `json:"events"`
}

type ProbeResult struct {
	JobID       int64             `json:"job_id"`
	TargetID    int64             `json:"target_id"`
	WorkerID    string            `json:"worker_id"`
	Region      string            `json:"region"`
	TS          time.Time         `json:"ts"`
	Success     bool              `json:"success"`
	LatencyMS   int               `json:"latency_ms"`
	StatusCode  int               `json:"status_code"`
	ErrorCode   string            `json:"error_code,omitempty"`
	ErrorMsg    string            `json:"error_msg,omitempty"`
	Observation map[string]string `json:"observation,omitempty"`
}

type ReportProbeResultsRequest struct {
	WorkerID string        `json:"worker_id"`
	Results  []ProbeResult `json:"results"`
}

type AckResponse struct {
	RequestID  string    `json:"request_id"`
	Code       int       `json:"code"`
	Message    string    `json:"message"`
	ServerTime time.Time `json:"server_time"`
}

type CreateMaintenanceWindowRequest struct {
	Title     string    `json:"title"`
	ScopeType string    `json:"scope_type"`
	ScopeRef  string    `json:"scope_ref"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	CreatedBy string    `json:"created_by"`
	Reason    string    `json:"reason,omitempty"`
}

type AckAlertRequest struct {
	AckedBy string `json:"acked_by"`
}
