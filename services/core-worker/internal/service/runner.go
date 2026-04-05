package service

import (
	"context"
	"log/slog"
	"time"
)

type Runner struct {
	logger   *slog.Logger
	interval time.Duration
}

func NewRunner(logger *slog.Logger, interval time.Duration) *Runner {
	return &Runner{logger: logger, interval: interval}
}

func (r *Runner) Run(ctx context.Context) error {
	r.logger.Info("core-worker started", "interval", r.interval.String())

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.tick()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("core-worker stopped")
			return nil
		case <-ticker.C:
			r.tick()
		}
	}
}

func (r *Runner) tick() {
	r.logger.Info("core-worker tick", "pipelines", "status-engine,alert-engine,probe-scheduler")
}
