package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofxq/gaoming/pkg/contracts"
)

type Config struct {
	WorkerID  string
	TargetURL string
	ReportURL string
	Region    string
	Interval  time.Duration
}

type Runner struct {
	cfg    Config
	logger *slog.Logger
	client *http.Client
}

func NewRunner(cfg Config, logger *slog.Logger) *Runner {
	return &Runner{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *Runner) Run(ctx context.Context) error {
	r.logger.Info("probe-worker started", "target", r.cfg.TargetURL, "report_url", r.cfg.ReportURL)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	r.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("probe-worker stopped")
			return nil
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	startedAt := time.Now().UTC()
	result := contracts.ProbeResult{
		JobID:       1,
		TargetID:    1,
		WorkerID:    r.cfg.WorkerID,
		Region:      r.cfg.Region,
		TS:          startedAt,
		Observation: map[string]string{"target": r.cfg.TargetURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.cfg.TargetURL, nil)
	if err != nil {
		result.ErrorCode = "probe_request_build_failed"
		result.ErrorMsg = err.Error()
		r.report(ctx, result)
		return
	}

	resp, err := r.client.Do(req)
	if err != nil {
		result.ErrorCode = "probe_failed"
		result.ErrorMsg = err.Error()
		r.report(ctx, result)
		return
	}
	defer resp.Body.Close()

	result.Success = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	result.StatusCode = resp.StatusCode
	result.LatencyMS = int(time.Since(startedAt).Milliseconds())
	r.report(ctx, result)
}

func (r *Runner) report(ctx context.Context, result contracts.ProbeResult) {
	payload := contracts.ReportProbeResultsRequest{
		WorkerID: r.cfg.WorkerID,
		Results:  []contracts.ProbeResult{result},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		r.logger.Error("failed to marshal probe report", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.ReportURL, bytes.NewReader(body))
	if err != nil {
		r.logger.Error("failed to build probe report request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error("failed to submit probe report", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		r.logger.Error("probe report rejected", "status", resp.Status, "report_url", r.cfg.ReportURL)
		return
	}

	r.logger.Info(
		"probe cycle complete",
		"success", result.Success,
		"status_code", result.StatusCode,
		"latency_ms", result.LatencyMS,
		"report_url", r.cfg.ReportURL,
	)
}
