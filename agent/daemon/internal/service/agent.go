package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
)

type Config struct {
	MasterAPIURL     string
	IngestGatewayURL string
	LoopInterval     time.Duration
	Host             contracts.HostIdentity
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
}

func New(cfg Config, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		logger:   logger,
		client:   &http.Client{Timeout: 5 * time.Second},
		agentID:  ids.New("agent"),
		bootTime: time.Now().UTC(),
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
	a.logger.Info("agent registered", "host_uid", a.hostUID)
	return nil
}

func (a *Agent) sendCycle(ctx context.Context) {
	if err := a.pushMetrics(ctx); err != nil {
		a.logger.Error("push metrics failed", "error", err)
	}
	if err := a.pushHeartbeat(ctx); err != nil {
		a.logger.Error("push heartbeat failed", "error", err)
	}
}

func (a *Agent) pushHeartbeat(ctx context.Context) error {
	a.hbSeq++
	payload := contracts.HeartbeatRequest{
		HostUID: a.hostUID,
		AgentID: a.agentID,
		Seq:     a.hbSeq,
		TS:      time.Now().UTC(),
		Digest:  a.digest(),
	}

	var resp contracts.HeartbeatResponse
	if err := a.postJSON(ctx, a.cfg.MasterAPIURL+"/api/v1/agents/heartbeat", payload, &resp); err != nil {
		return err
	}
	a.logger.Info("heartbeat sent", "host_uid", a.hostUID, "seq", a.hbSeq)
	return nil
}

func (a *Agent) pushMetrics(ctx context.Context) error {
	a.metricSeq++
	now := time.Now().UTC()
	payload := contracts.PushMetricBatchRequest{
		HostUID:     a.hostUID,
		AgentID:     a.agentID,
		BatchSeq:    a.metricSeq,
		CollectedAt: now,
		Points: []contracts.MetricPoint{
			{Name: "runtime_goroutines", Value: float64(runtime.NumGoroutine()), TS: now},
			{Name: "runtime_uptime_seconds", Value: time.Since(a.bootTime).Seconds(), TS: now},
			{Name: "agent_heartbeat_seq", Value: float64(a.hbSeq), TS: now},
		},
	}

	var resp contracts.AckResponse
	if err := a.postJSON(ctx, a.cfg.IngestGatewayURL+"/api/v1/metrics", payload, &resp); err != nil {
		return err
	}
	a.logger.Info("metrics sent", "host_uid", a.hostUID, "batch_seq", a.metricSeq)
	return nil
}

func (a *Agent) digest() contracts.AgentDigest {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return contracts.AgentDigest{
		CPUUsagePct:        float64((runtime.NumGoroutine() % 100)),
		MemUsedPct:         float64((mem.Alloc / 1024 / 1024) % 100),
		DiskUsedPct:        0,
		Load1:              float64(runtime.NumGoroutine()),
		NetRxBPS:           0,
		NetTxBPS:           0,
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
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
