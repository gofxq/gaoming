package app

import (
	"context"
	"time"

	"github.com/gofxq/gaoming/agent/daemon/internal/config"
	"github.com/gofxq/gaoming/agent/daemon/internal/identity"
	"github.com/gofxq/gaoming/agent/daemon/internal/service"
	"github.com/gofxq/gaoming/pkg/logx"
)

type App struct {
	agent *service.Agent
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("agent")
	host := identity.Discover(cfg.Region, cfg.Env, cfg.Role)
	host.TenantCode = cfg.TenantCode

	agent := service.New(service.Config{
		MasterAPIURL:     cfg.MasterAPIURL,
		IngestGatewayURL: cfg.IngestGatewayURL,
		LoopInterval:     time.Duration(cfg.LoopIntervalSec) * time.Second,
		Host:             host,
		PersistTenant: func(tenantCode string) error {
			return config.SaveTenant(cfg.ConfigPath, tenantCode)
		},
	}, logger)

	return &App{agent: agent}
}

func (a *App) Run(ctx context.Context) error {
	return a.agent.Run(ctx)
}
