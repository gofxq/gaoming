package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	postgresrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/postgres"
	redisrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/redis"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/pkg/migrations"
	coreruntime "github.com/gofxq/gaoming/services/core-worker/runtime"
	ingestruntime "github.com/gofxq/gaoming/services/ingest-gateway/runtime"
	masterruntime "github.com/gofxq/gaoming/services/master-api/runtime"
	proberuntime "github.com/gofxq/gaoming/services/probe-worker/runtime"
	"github.com/gofxq/gaoming/services/server/internal/config"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// App owns all listeners, background workers and shared storage connections.
type App struct {
	httpServer  *http.Server
	grpcServer  *grpc.Server
	grpcAddr    string
	postgres    *sql.DB
	redisClient *redis.Client
	logger      *logx.Logger
	cfg         config.Config
	reconciler  func(context.Context) error
	closeOnce   sync.Once
}

func New(ctx context.Context, cfg config.Config) (_ *App, err error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := gorm.Open(postgres.Open(cfg.Master.PostgresDSN), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("invalid PostgreSQL configuration")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection")
	}
	a := &App{postgres: sqlDB, cfg: cfg, grpcAddr: cfg.GRPCAddr, logger: logx.New("server")}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()
	if err = sqlDB.PingContext(initCtx); err != nil {
		return nil, fmt.Errorf("PostgreSQL unavailable; check config/server.yml and start native PostgreSQL")
	}
	if err = migrations.Check(initCtx, sqlDB); err != nil {
		return nil, err
	}
	a.redisClient = redis.NewClient(&redis.Options{Addr: cfg.Master.RedisAddr, Password: cfg.Master.RedisPassword, DB: cfg.Master.RedisDB})
	if err = a.redisClient.Ping(initCtx).Err(); err != nil {
		return nil, fmt.Errorf("Redis unavailable; check config/server.yml and start native Redis")
	}
	store, err := postgresrepo.NewStore(initCtx, db, postgresrepo.Config{TenantCode: cfg.Master.TenantCode, TenantName: cfg.Master.TenantName, AllowCustomTenantCode: cfg.Master.AllowCustomTenantCode})
	if err != nil {
		return nil, err
	}
	metricPrefix, eventChannel := "", ""
	if cfg.RedisNamespace != "" {
		metricPrefix = cfg.RedisNamespace + ":metrics"
		eventChannel = cfg.RedisNamespace + ":host-events"
	}
	metrics := redisrepo.NewMetricWindowStore(a.redisClient, metricPrefix, 60, 2*time.Hour)
	events := redisrepo.NewEventBus(a.redisClient, eventChannel)
	masterHandler := masterruntime.NewHandler(cfg.Master, store, db, metrics, events, a.logger)
	ingest := ingestruntime.New(store, metrics, events, a.logger)
	mux := http.NewServeMux()
	mux.Handle("/master/", masterHandler)
	mux.Handle("/ingest/", ingest.HTTP)
	a.httpServer = &http.Server{Addr: cfg.Master.HTTPAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	a.grpcServer = ingest.GRPC
	a.reconciler = ingest.RunReconciler
	return a, nil
}

// Close releases storage only; Run stops active transports before calling Close.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		if a.redisClient != nil {
			_ = a.redisClient.Close()
		}
		if a.postgres != nil {
			_ = a.postgres.Close()
		}
		if a.logger != nil {
			_ = a.logger.Sync()
		}
	})
}

func (a *App) Run(ctx context.Context) error {
	defer a.Close()
	if err := ctx.Err(); err != nil {
		return nil
	}
	httpListener, err := net.Listen("tcp", a.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	defer httpListener.Close()
	grpcListener, err := net.Listen("tcp", a.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	defer grpcListener.Close()
	return a.serve(ctx, httpListener, grpcListener)
}

func (a *App) serve(ctx context.Context, httpListener, grpcListener net.Listener) error {
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 5)
	var workers sync.WaitGroup
	start := func(name string, run func() error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			err := run()
			if err != nil {
				err = fmt.Errorf("%s: %w", name, err)
			}
			errCh <- err
		}()
	}
	start("HTTP", func() error { return a.httpServer.Serve(httpListener) })
	start("gRPC", func() error { return a.grpcServer.Serve(grpcListener) })
	if a.logger != nil {
		a.logger.Info("starting unified backend", "http_addr", httpListener.Addr().String(), "grpc_addr", grpcListener.Addr().String())
		start("core-worker", func() error {
			return coreruntime.Run(workCtx, time.Duration(a.cfg.CoreIntervalSec)*time.Second, a.logger)
		})
		start("probe-worker", func() error {
			base := localHTTPBase(httpListener.Addr().String())
			target := a.cfg.ProbeTargetURL
			if target == "" {
				target = base + "/master/healthz"
			}
			return proberuntime.Run(workCtx, proberuntime.Config{WorkerID: "probe-worker-local", TargetURL: target, ReportURL: base + "/ingest/api/v1/probes", Region: "local", Interval: time.Duration(a.cfg.ProbeIntervalSec) * time.Second}, a.logger)
		})
	}
	if a.reconciler != nil {
		start("offline reconciler", func() error { return a.reconciler(workCtx) })
	}
	var result error
	select {
	case <-ctx.Done():
	case result = <-errCh:
		if ctx.Err() == nil && result == nil {
			result = fmt.Errorf("backend component stopped unexpectedly")
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	// HTTP streams and persistent gRPC streams share the same shutdown deadline.
	stopped := make(chan struct{})
	go func() { a.grpcServer.GracefulStop(); close(stopped) }()
	httpStopped := make(chan error, 1)
	go func() { httpStopped <- a.httpServer.Shutdown(shutdownCtx) }()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		a.grpcServer.Stop()
	}
	if err := <-httpStopped; err != nil {
		_ = a.httpServer.Close()
	}
	workers.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if errors.Is(result, http.ErrServerClosed) || errors.Is(result, grpc.ErrServerStopped) {
		return fmt.Errorf("backend listener stopped unexpectedly: %w", result)
	}
	return result
}

func localHTTPBase(addr string) string {
	host, port, _ := net.SplitHostPort(addr)
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
