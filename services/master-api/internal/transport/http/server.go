package http

import (
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
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
	engine.GET("/master/healthz", s.handleHealth)
	engine.GET("/master/api/v1/stream/hosts", s.handleHostStream)
	engine.POST("/master/api/v1/install/tenant", s.handleAllocateInstallTenant)
	engine.POST("/master/api/v1/agents/register", s.handleRegisterAgent)
	engine.GET("/master/api/v1/hosts", s.handleListHosts)
	engine.GET("/master/api/v1/hosts/:hostUID", s.handleGetHost)
	engine.POST("/master/api/v1/ops/maintenance", s.handleCreateMaintenance)
	engine.POST("/master/api/v1/ops/alerts/:alertID/ack", s.handleAckAlert)
	return engine
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(nethttp.StatusOK, s.svc.Health())
}

func (s *Server) handleAllocateInstallTenant(c *gin.Context) {
	resp, err := s.svc.AllocateInstallTenant(c.Request.Context())
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleRegisterAgent(c *gin.Context) {
	var req contracts.RegisterAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.svc.RegisterAgent(c.Request.Context(), req)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleListHosts(c *gin.Context) {
	items, err := s.svc.ListHosts(c.Request.Context(), tenantCodeFromContext(c))
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"items": items})
}

func (s *Server) handleGetHost(c *gin.Context) {
	hostUID := strings.TrimSpace(c.Param("hostUID"))
	if hostUID == "" {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "missing host uid"})
		return
	}

	host, ok, err := s.svc.GetHost(c.Request.Context(), hostUID, tenantCodeFromContext(c))
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	c.JSON(nethttp.StatusOK, host)
}

func (s *Server) handleCreateMaintenance(c *gin.Context) {
	var req contracts.CreateMaintenanceWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.svc.CreateMaintenance(c.Request.Context(), req)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusCreated, resp)
}

func (s *Server) handleAckAlert(c *gin.Context) {
	alertID := strings.TrimSpace(c.Param("alertID"))
	if alertID == "" {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid alert ack path"})
		return
	}

	var req contracts.AckAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := s.svc.AckAlert(c.Request.Context(), alertID, req)
	if err != nil {
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func tenantCodeFromContext(c *gin.Context) string {
	return strings.TrimSpace(c.Query("tenant"))
}
