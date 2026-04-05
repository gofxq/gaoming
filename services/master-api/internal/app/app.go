package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
	svc    *service.Service
	cancel context.CancelFunc
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("master-api")
	store := memory.NewStore()
	svc := service.New(store, clock.Real{}, logger)
	handler := httptransport.NewServer(svc).Handler()
	bgCtx, cancel := context.WithCancel(context.Background())

	app := &App{
		server: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
		logger: logger,
		svc:    svc,
		cancel: cancel,
	}

	go app.runOfflineReconciler(bgCtx)
	return app
}

func (a *App) Run() error {
	a.logger.Info("starting master-api", "addr", a.server.Addr)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	return a.server.Shutdown(ctx)
}

func (a *App) runOfflineReconciler(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed := a.svc.ReconcileOfflineHosts()
			if changed > 0 {
				a.logger.Info("reconciled offline hosts", "count", changed)
			}
		}
	}
}
