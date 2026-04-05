package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
)

type Config struct {
	MasterAPIURL     string
	IngestGatewayURL string
	LoopInterval     time.Duration
	Host             contracts.HostIdentity
	PersistTenant    func(string) error
}

type Agent struct {
	cfg       Config
	logger    *slog.Logger
	client    *http.Client
	agentID   string
	hostUID   string
	bootTime  time.Time
	hbSeq     int64
	metricSeq int64
	sampler   systemSampler
}

type apiError struct {
	StatusCode int
	Status     string
}

func (e apiError) Error() string {
	return fmt.Sprintf("unexpected status: %s", e.Status)
}

func New(cfg Config, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		logger:   logger,
		client:   &http.Client{Timeout: 5 * time.Second},
		agentID:  ids.New("agent"),
		bootTime: time.Now().UTC(),
		sampler:  newSystemSampler(),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	for {
		if err := a.register(ctx); err == nil {
			break
		} else {
			a.logger.Warn("register agent failed, retrying", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	ticker := time.NewTicker(a.cfg.LoopInterval)
	defer ticker.Stop()

	a.sendCycle(ctx)
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("agent stopped")
			return nil
		case <-ticker.C:
			a.sendCycle(ctx)
		}
	}
}

func (a *Agent) register(ctx context.Context) error {
	payload := contracts.RegisterAgentRequest{
		Host: a.cfg.Host,
		Agent: contracts.AgentMetadata{
			AgentID:      a.agentID,
			Version:      "v0.1.0",
			Capabilities: []string{"heartbeat", "metrics"},
			BootTime:     a.bootTime,
		},
	}

	var resp contracts.RegisterAgentResponse
	if err := a.postJSON(ctx, a.cfg.MasterAPIURL+"/api/v1/agents/register", payload, &resp); err != nil {
		return fmt.Errorf("register agent: %w", err)
	}

	a.hostUID = resp.HostUID
	if resp.TenantCode != "" && resp.TenantCode != a.cfg.Host.TenantCode {
		if a.cfg.PersistTenant != nil {
			if err := a.cfg.PersistTenant(resp.TenantCode); err != nil {
				return fmt.Errorf("persist tenant: %w", err)
			}
		}
		a.cfg.Host.TenantCode = resp.TenantCode
	}
	a.logger.Info("agent registered", "host_uid", a.hostUID, "tenant_code", a.cfg.Host.TenantCode)
	return nil
}

func (a *Agent) sendCycle(ctx context.Context) {
	now := time.Now().UTC()
	digest := a.digest(now)

	if err := a.pushMetricsWithDigest(ctx, now, digest); err != nil {
		a.logger.Error("push metrics failed", "error", err)
	}
	if err := a.pushHeartbeat(ctx, now, digest); err != nil {
		a.logger.Error("push heartbeat failed", "error", err)
	}
}

func (a *Agent) pushHeartbeat(ctx context.Context, now time.Time, digest contracts.AgentDigest) error {
	if a.hostUID == "" {
		if err := a.register(ctx); err != nil {
			return err
		}
	}

	a.hbSeq++
	payload := contracts.HeartbeatRequest{
		HostUID: a.hostUID,
		AgentID: a.agentID,
		Seq:     a.hbSeq,
		TS:      now,
		Digest:  digest,
	}

	var resp contracts.HeartbeatResponse
	if err := a.postJSON(ctx, a.cfg.MasterAPIURL+"/api/v1/agents/heartbeat", payload, &resp); err != nil {
		var apiErr apiError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			a.logger.Warn("heartbeat target missing on server, re-registering", "host_uid", a.hostUID)
			a.hostUID = ""
			return a.register(ctx)
		}
		return err
	}
	a.logger.Info("heartbeat sent", "host_uid", a.hostUID, "seq", a.hbSeq)
	return nil
}

func (a *Agent) pushMetricsWithDigest(ctx context.Context, now time.Time, digest contracts.AgentDigest) error {
	a.metricSeq++
	payload := contracts.PushMetricBatchRequest{
		HostUID:     a.hostUID,
		AgentID:     a.agentID,
		BatchSeq:    a.metricSeq,
		CollectedAt: now,
		Points: []contracts.MetricPoint{
			{Name: "runtime_uptime_seconds", Value: time.Since(a.bootTime).Seconds(), TS: now},
			{Name: "agent_heartbeat_seq", Value: float64(a.hbSeq), TS: now},
			{Name: "host_cpu_usage_pct", Value: digest.CPUUsagePct, TS: now},
			{Name: "host_mem_used_pct", Value: digest.MemUsedPct, TS: now},
			{Name: "host_disk_used_pct", Value: digest.DiskUsedPct, TS: now},
			{Name: "host_disk_read_bps", Value: float64(digest.DiskReadBPS), TS: now},
			{Name: "host_disk_write_bps", Value: float64(digest.DiskWriteBPS), TS: now},
			{Name: "host_load1", Value: digest.Load1, TS: now},
			{Name: "host_net_rx_bps", Value: float64(digest.NetRxBPS), TS: now},
			{Name: "host_net_tx_bps", Value: float64(digest.NetTxBPS), TS: now},
		},
	}

	var resp contracts.AckResponse
	if err := a.postJSON(ctx, a.cfg.IngestGatewayURL+"/api/v1/metrics", payload, &resp); err != nil {
		return err
	}
	a.logger.Info("metrics sent", "host_uid", a.hostUID, "batch_seq", a.metricSeq)
	return nil
}

func (a *Agent) digest(now time.Time) contracts.AgentDigest {
	metrics := a.sampler.Sample(now)
	return contracts.AgentDigest{
		CPUUsagePct:        metrics.CPUUsagePct,
		MemUsedPct:         metrics.MemUsedPct,
		DiskUsedPct:        metrics.DiskUsedPct,
		DiskReadBPS:        metrics.DiskReadBPS,
		DiskWriteBPS:       metrics.DiskWriteBPS,
		Load1:              metrics.Load1,
		NetRxBPS:           metrics.NetRxBPS,
		NetTxBPS:           metrics.NetTxBPS,
		QueueDepth:         0,
		LastMetricBatchSeq: a.metricSeq,
	}
}

func (a *Agent) postJSON(ctx context.Context, url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return apiError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
