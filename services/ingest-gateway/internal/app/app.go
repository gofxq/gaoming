package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/clock"
	postgresrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/postgres"
	redisrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/redis"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/config"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	grpctransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/grpc"
	httptransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/http"
	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type App struct {
	httpServer   *http.Server
	grpcServer   *grpc.Server
	grpcAddr     string
	grpcListener net.Listener
	logger       *slog.Logger
	svc          *service.Service
	cancel       context.CancelFunc
	postgres     *sql.DB
	redisClient  *goredis.Client
}

func New() (*App, error) {
	cfg := config.Load()
	logger := logx.New("ingest-gateway")
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
		TenantCode:            cfg.TenantCode,
		TenantName:            cfg.TenantName,
		AllowCustomTenantCode: cfg.AllowCustomTenantCode,
	})
	if err != nil {
		_ = sqlDB.Close()
		_ = redisClient.Close()
		return nil, err
	}
	metricStore := redisrepo.NewMetricWindowStore(redisClient, "", 60, 2*time.Hour)
	eventBus := redisrepo.NewEventBus(redisClient, "")
	svc := service.New(logger, clock.Real{}, hostStore, metricStore, eventBus)
	handler := httptransport.NewServer(svc).Handler()
	grpcServer := grpc.NewServer()
	monitorv1.RegisterAgentControlServiceServer(grpcServer, grpctransport.NewServer(svc))
	monitorv1.RegisterMetricsIngestServiceServer(grpcServer, grpctransport.NewServer(svc))

	bgCtx, cancel := context.WithCancel(context.Background())
	app := &App{
		httpServer: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
		grpcServer:  grpcServer,
		grpcAddr:    cfg.GRPCAddr,
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
	grpcListener, err := net.Listen("tcp", a.grpcAddr)
	if err != nil {
		return err
	}
	a.grpcListener = grpcListener

	a.logger.Info("starting ingest-gateway", "http_addr", a.httpServer.Addr, "grpc_addr", a.grpcAddr)

	errCh := make(chan error, 2)
	go func() {
		errCh <- a.grpcServer.Serve(grpcListener)
	}()
	go func() {
		errCh <- a.httpServer.ListenAndServe()
	}()
	return <-errCh
}

func (a *App) Shutdown(ctx context.Context) error {
	a.cancel()
	a.grpcServer.GracefulStop()
	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
	}
	if err := a.httpServer.Shutdown(ctx); err != nil {
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
