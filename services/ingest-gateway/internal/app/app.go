package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/config"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	grpctransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/grpc"
	httptransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/http"
	"google.golang.org/grpc"
)

type App struct {
	httpServer   *http.Server
	grpcServer   *grpc.Server
	grpcAddr     string
	grpcListener net.Listener
	logger       *slog.Logger
}

func New() *App {
	cfg := config.Load()
	logger := logx.New("ingest-gateway")
	svc := service.New(logger, clock.Real{})
	handler := httptransport.NewServer(svc).Handler()
	grpcServer := grpc.NewServer()
	monitorv1.RegisterMetricsIngestServiceServer(grpcServer, grpctransport.NewServer(svc))

	return &App{
		httpServer: &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		},
		grpcServer: grpcServer,
		grpcAddr:   cfg.GRPCAddr,
		logger:     logger,
	}
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
	a.grpcServer.GracefulStop()
	if a.grpcListener != nil {
		_ = a.grpcListener.Close()
	}
	return a.httpServer.Shutdown(ctx)
}
