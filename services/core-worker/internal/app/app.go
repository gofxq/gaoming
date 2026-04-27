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

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger := logx.New("core-worker")
	runner := service.NewRunner(logger, time.Duration(cfg.LoopIntervalSec)*time.Second)
	return &App{runner: runner}, nil
}

func (a *App) Run(ctx context.Context) error {
	return a.runner.Run(ctx)
}
