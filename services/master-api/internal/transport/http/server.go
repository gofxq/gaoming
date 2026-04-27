package http

import (
	"errors"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/master-api/internal/auth"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

const userContextKey = "session_user"

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

	authGroup := engine.Group("/master/api/v1/auth")
	authGroup.GET("/session", s.handleSession)
	authGroup.GET("/wechat/url", s.handleWeChatLoginURL)
	authGroup.GET("/wechat/callback", s.handleWeChatCallback)
	authGroup.POST("/logout", s.handleLogout)

	engine.POST("/master/api/v1/install/tenant", s.handleAllocateInstallTenant)
	engine.POST("/master/api/v1/agents/register", s.handleRegisterAgent)

	protected := engine.Group("/master/api/v1")
	protected.Use(s.requireSession())
	protected.GET("/stream/hosts", s.handleHostStream)
	protected.GET("/hosts", s.handleListHosts)
	protected.GET("/hosts/:hostUID", s.handleGetHost)
	protected.POST("/ops/maintenance", s.handleCreateMaintenance)
	protected.POST("/ops/alerts/:alertID/ack", s.handleAckAlert)

	admin := protected.Group("/admin")
	admin.Use(requireAdmin())
	admin.GET("/users", s.handleListUsers)
	admin.PATCH("/users/:userID", s.handleUpdateUser)

	return engine
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(nethttp.StatusOK, s.svc.Health())
}

func (s *Server) handleAllocateInstallTenant(c *gin.Context) {
	resp, err := s.svc.AllocateInstallTenant(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleRegisterAgent(c *gin.Context) {
	var req contracts.RegisterAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, err)
		return
	}

	resp, err := s.svc.RegisterAgent(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleListHosts(c *gin.Context) {
	items, err := s.svc.ListHosts(c.Request.Context(), tenantCodeFromContext(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"items": items})
}

func (s *Server) handleGetHost(c *gin.Context) {
	hostUID := strings.TrimSpace(c.Param("hostUID"))
	if hostUID == "" {
		writeError(c, errors.New("missing host uid"))
		return
	}

	host, ok, err := s.svc.GetHost(c.Request.Context(), hostUID, tenantCodeFromContext(c))
	if err != nil {
		writeError(c, err)
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
		writeError(c, err)
		return
	}

	resp, err := s.svc.CreateMaintenance(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusCreated, resp)
}

func (s *Server) handleAckAlert(c *gin.Context) {
	alertID := strings.TrimSpace(c.Param("alertID"))
	if alertID == "" {
		writeError(c, errors.New("invalid alert ack path"))
		return
	}

	var req contracts.AckAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, err)
		return
	}

	resp, err := s.svc.AckAlert(c.Request.Context(), alertID, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleSession(c *gin.Context) {
	resp, err := s.svc.GetSession(c.Request.Context(), sessionTokenFromRequest(c, s.cookieName()))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleWeChatLoginURL(c *gin.Context) {
	resp, err := s.svc.GetWeChatLoginURL(
		c.Query("return_to"),
		strings.TrimSpace(c.Query("tenant")),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, resp)
}

func (s *Server) handleWeChatCallback(c *gin.Context) {
	session, returnTo, err := s.svc.HandleWeChatCallback(
		c.Request.Context(),
		c.Query("code"),
		c.Query("state"),
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	setSessionCookie(c, s.cookieName(), session.Token, session.ExpiresAt)
	c.Redirect(nethttp.StatusFound, returnTo)
}

func (s *Server) handleLogout(c *gin.Context) {
	if err := s.svc.Logout(c.Request.Context(), sessionTokenFromRequest(c, s.cookieName())); err != nil {
		writeError(c, err)
		return
	}
	clearSessionCookie(c, s.cookieName())
	c.Status(nethttp.StatusNoContent)
}

func (s *Server) handleListUsers(c *gin.Context) {
	items, err := s.svc.ListUsers(c.Request.Context(), currentUser(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"items": items})
}

func (s *Server) handleUpdateUser(c *gin.Context) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("userID")), 10, 64)
	if err != nil || userID <= 0 {
		writeError(c, errors.New("invalid user id"))
		return
	}

	var req struct {
		DisplayName *string          `json:"display_name"`
		Role        *auth.UserRole   `json:"role"`
		Status      *auth.UserStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, err)
		return
	}
	if req.DisplayName != nil {
		value := strings.TrimSpace(*req.DisplayName)
		req.DisplayName = &value
	}

	item, err := s.svc.UpdateUser(c.Request.Context(), currentUser(c), userID, auth.UserUpdate{
		DisplayName: req.DisplayName,
		Role:        req.Role,
		Status:      req.Status,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, item)
}

func tenantCodeFromContext(c *gin.Context) string {
	if user, ok := c.Get(userContextKey); ok {
		if current, ok := user.(auth.User); ok && current.TenantCode != "" {
			return current.TenantCode
		}
	}
	return strings.TrimSpace(c.Query("tenant"))
}

func (s *Server) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, _, ok, err := s.svc.ResolveSessionUser(c.Request.Context(), sessionTokenFromRequest(c, s.cookieName()))
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
		if !ok {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(userContextKey, user)
		c.Next()
	}
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUser(c).Role != auth.UserRoleAdmin {
			writeError(c, service.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) auth.User {
	value, _ := c.Get(userContextKey)
	user, _ := value.(auth.User)
	return user
}

func (s *Server) cookieName() string {
	if s.svc.SessionCookieName() != "" {
		return s.svc.SessionCookieName()
	}
	return "gaoming_session"
}

func sessionTokenFromRequest(c *gin.Context, cookieName string) string {
	cookie, err := c.Request.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(c *gin.Context, cookieName string, value string, expiresAt time.Time) {
	c.SetCookie(cookieName, value, max(1, int(time.Until(expiresAt).Seconds())), "/", "", false, true)
}

func clearSessionCookie(c *gin.Context, cookieName string) {
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
}

func writeError(c *gin.Context, err error) {
	switch {
	case err == nil:
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "unknown error"})
	case errors.Is(err, service.ErrUnauthorized):
		c.JSON(nethttp.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(nethttp.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrAuthNotConfigured):
		c.JSON(nethttp.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidAuthState):
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
