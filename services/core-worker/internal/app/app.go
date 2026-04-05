package app

import (
	"context"
	"time"

	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/core-worker/internal/config"
	"github.com/gofxq/gaoming/services/core-worker/internal/service"
)

type App struct {
	runner *service.Runner
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("core-worker")
	runner := service.NewRunner(logger, time.Duration(cfg.LoopIntervalSec)*time.Second)
	return &App{runner: runner}
}

func (a *App) Run(ctx context.Context) error {
	return a.runner.Run(ctx)
}
