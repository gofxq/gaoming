package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/config"
	"github.com/gofxq/gaoming/services/master-api/internal/repository/memory"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
	httptransport "github.com/gofxq/gaoming/services/master-api/internal/transport/http"
)

type App struct {
	server *http.Server
	logger *slog.Logger
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("master-api")
	store := memory.NewStore()
	svc := service.New(store, clock.Real{}, logger)
	handler := httptransport.NewServer(svc).Handler()

	return &App{
		server: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
		logger: logger,
	}
}

func (a *App) Run() error {
	a.logger.Info("starting master-api", "addr", a.server.Addr)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}
