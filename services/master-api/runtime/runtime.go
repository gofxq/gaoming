// Package runtime exposes master-api composition to the unified server.
package runtime

import (
	"net/http"
	"time"

	"github.com/gofxq/gaoming/pkg/access"
	accesspg "github.com/gofxq/gaoming/pkg/access/postgres"
	"github.com/gofxq/gaoming/pkg/clock"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	postgresrepo "github.com/gofxq/gaoming/pkg/hostruntime/repository/postgres"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/auth"
	"github.com/gofxq/gaoming/services/master-api/internal/config"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
	httptransport "github.com/gofxq/gaoming/services/master-api/internal/transport/http"
	"gorm.io/gorm"
)

type Config = config.Config

func LoadConfig(path string) (Config, error) { return config.LoadFromFile(path) }

func NewHandler(cfg Config, hostStore *postgresrepo.Store, db *gorm.DB, metrics hostruntime.MetricWindowStore, events hostruntime.EventBus, logger *logx.Logger) http.Handler {
	svc := service.New(hostStore, metrics, hostStore, events, auth.NewStore(db), service.AuthConfig{
		SessionTTL: time.Duration(cfg.SessionTTLHours) * time.Hour, SessionCookieName: cfg.SessionCookieName, SessionSecret: cfg.SessionSecret, SetupToken: cfg.SetupToken, CookieSecure: cfg.SessionCookieSecure, AllowedOrigins: cfg.AllowedOrigins,
	}, clock.Real{}, logger)
	svc.ConfigureAccess(accesspg.NewStore(db), access.Principal{Kind: access.Demo, TenantCode: cfg.DemoTenantCode, AllowedHosts: cfg.DemoHostUIDs, Fields: cfg.DemoFields})
	return httptransport.NewServer(svc, logger).Handler()
}
