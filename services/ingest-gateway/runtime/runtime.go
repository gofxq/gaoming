// Package runtime exposes ingestion transports and reconciliation to a shared process.
package runtime

import (
	"context"
	"net/http"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/clock"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	postgresrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/postgres"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	grpctransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/grpc"
	httptransport "github.com/gofxq/gaoming/services/ingest-gateway/internal/transport/http"
	"google.golang.org/grpc"
)

type Module struct {
	HTTP   http.Handler
	GRPC   *grpc.Server
	svc    *service.Service
	logger *logx.Logger
}

func New(store *postgresrepo.Store, metrics hostruntime.MetricWindowStore, events hostruntime.EventBus, logger *logx.Logger) *Module {
	svc := service.New(logger, clock.Real{}, store, metrics, events)
	grpcServer := grpc.NewServer()
	monitorv1.RegisterAgentControlServiceServer(grpcServer, grpctransport.NewServer(svc))
	monitorv1.RegisterMetricsIngestServiceServer(grpcServer, grpctransport.NewServer(svc))
	return &Module{HTTP: httptransport.NewServer(svc, logger).Handler(), GRPC: grpcServer, svc: svc, logger: logger}
}

func (m *Module) RunReconciler(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if changed, err := m.svc.ReconcileOfflineHosts(ctx); err != nil {
				m.logger.Error("reconcile offline hosts failed", "error", err)
			} else if changed > 0 {
				m.logger.Info("reconciled offline hosts", "count", changed)
			}
		}
	}
}
