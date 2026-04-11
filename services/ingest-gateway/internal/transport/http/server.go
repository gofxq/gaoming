package http

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
)

type Server struct {
	svc    *service.Service
	logger *logx.Logger
}

func NewServer(svc *service.Service, logger *logx.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	return &Server{svc: svc, logger: logger}
}

func (s *Server) Handler() nethttp.Handler {
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.Use(logx.GinMiddleware(s.logger))
	engine.Use(gin.Recovery())
	engine.GET("/ingest/healthz", s.handleHealth)
	engine.POST("/ingest/api/v1/events", s.handleEvents)
	engine.POST("/ingest/api/v1/probes", s.handleProbes)
	engine.GET("/ingest/debug/counters", s.handleCounters)
	return engine
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(nethttp.StatusOK, s.svc.Health())
}

func (s *Server) handleEvents(c *gin.Context) {
	var req contracts.PushEventBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusAccepted, s.svc.PushEventBatch(req))
}

func (s *Server) handleProbes(c *gin.Context) {
	var req contracts.ReportProbeResultsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusAccepted, s.svc.ReportProbeResults(req))
}

func (s *Server) handleCounters(c *gin.Context) {
	c.JSON(nethttp.StatusOK, s.svc.Stats())
}
