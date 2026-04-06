package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/config"
	postgresrepo "github.com/gofxq/gaoming/services/master-api/internal/repository/postgres"
	redisrepo "github.com/gofxq/gaoming/services/master-api/internal/repository/redis"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
	httptransport "github.com/gofxq/gaoming/services/master-api/internal/transport/http"
	goredis "github.com/redis/go-redis/v9"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type App struct {
	server      *http.Server
	logger      *slog.Logger
	svc         *service.Service
	cancel      context.CancelFunc
	postgres    *sql.DB
	redisClient *goredis.Client
}

func New() (*App, error) {
	cfg := config.Load()
	logger := logx.New("master-api")
	if cfg.RuntimeBackend != "pg_redis" {
		return nil, fmt.Errorf("unsupported runtime backend %q", cfg.RuntimeBackend)
	}

	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	gormDB, err := gorm.Open(gormpostgres.Open(cfg.PostgresDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("open postgres sql db: %w", err)
	}
	if err := sqlDB.PingContext(initCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := redisClient.Ping(initCtx).Err(); err != nil {
		_ = sqlDB.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	hostStore, err := postgresrepo.NewStore(initCtx, gormDB, postgresrepo.Config{
		TenantCode: cfg.TenantCode,
		TenantName: cfg.TenantName,
	})
	if err != nil {
		_ = sqlDB.Close()
		_ = redisClient.Close()
		return nil, err
	}
	metricStore := redisrepo.NewMetricWindowStore(redisClient, "", 3600, 2*time.Hour)
	eventBus := redisrepo.NewEventBus(redisClient, "")
	svc := service.New(hostStore, metricStore, hostStore, eventBus, clock.Real{}, logger)
	handler := httptransport.NewServer(svc).Handler()
	bgCtx, cancel := context.WithCancel(context.Background())

	app := &App{
		server: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
		logger:      logger,
		svc:         svc,
		cancel:      cancel,
		postgres:    sqlDB,
		redisClient: redisClient,
	}

	go app.runOfflineReconciler(bgCtx)
	return app, nil
}

func (a *App) Run() error {
	a.logger.Info("starting master-api", "addr", a.server.Addr)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	if a.postgres != nil {
		a.postgres.Close()
	}
	if a.redisClient != nil {
		return a.redisClient.Close()
	}
	return nil
}

func (a *App) runOfflineReconciler(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := a.svc.ReconcileOfflineHosts(ctx)
			if err != nil {
				a.logger.Error("reconcile offline hosts failed", "error", err)
				continue
			}
			if changed > 0 {
				a.logger.Info("reconciled offline hosts", "count", changed)
			}
		}
	}
}
