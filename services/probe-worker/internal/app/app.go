package app

import (
	"context"
	"time"

	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/probe-worker/internal/config"
	"github.com/gofxq/gaoming/services/probe-worker/internal/service"
)

type App struct {
	runner *service.Runner
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("probe-worker")

	runner := service.NewRunner(service.Config{
		WorkerID:  cfg.WorkerID,
		TargetURL: cfg.TargetURL,
		ReportURL: cfg.ReportURL,
		Region:    cfg.Region,
		Interval:  time.Duration(cfg.ProbeIntervalSec) * time.Second,
	}, logger)

	return &App{runner: runner}
}

func (a *App) Run(ctx context.Context) error {
	return a.runner.Run(ctx)
}
