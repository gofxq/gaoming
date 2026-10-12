package http

import (
	"errors"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/pkg/ids"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/server/internal/master/auth"
	"github.com/gofxq/gaoming/server/internal/master/service"
	"github.com/jackc/pgx/v5/pgconn"
)

const userContextKey = "session_user"

type Server struct {
	svc       *service.Service
	logger    *logx.Logger
	authGuard *authGuard
}

func NewServer(svc *service.Service, logger *logx.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	return &Server{svc: svc, logger: logger, authGuard: newAuthGuard()}
}

func (s *Server) Handler() nethttp.Handler {
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.Use(logx.GinMiddleware(s.logger))
	engine.Use(gin.Recovery())

	engine.GET("/master/healthz", s.handleHealth)

	authGroup := engine.Group("/master/api/v1/auth")
	authGroup.Use(s.requireAllowedOrigin())
	authGroup.POST("/setup", s.limitAuthWork(), s.handleSetup)
	authGroup.GET("/setup-status", s.handleSetupStatus)
	authGroup.POST("/login", s.limitAuthWork(), s.handleLogin)
	authGroup.GET("/csrf", s.requireSession(), s.handleCSRF)
	authGroup.GET("/session", s.handleSession)
	authGroup.POST("/logout", s.requireSession(), s.requireCSRF(), s.handleLogout)

	engine.POST("/master/api/v1/install/tenant", s.handleAllocateInstallTenant)
	engine.POST("/master/api/v1/agents/register", s.handleRegisterAgent)

	privateReads := engine.Group("/master/api/v1")
	privateReads.Use(s.requireSession())
	privateReads.GET("/stream/hosts", s.handleHostStream)
	privateReads.GET("/hosts", s.handleListHosts)
	privateReads.GET("/hosts/:hostUID/metrics/latest", s.handleGetLatestMetricSnapshot)
	privateReads.GET("/hosts/:hostUID", s.handleGetHost)

	shared := engine.Group("/master/api/v1/public/shares")
	shared.Use(s.requireAllowedOrigin())
	shared.POST("/exchange", s.limitAuthWork(), s.handleExchangeShare)
	shared.Use(s.requireShare())
	shared.GET("/session", s.handleShareInfo)
	shared.GET("/hosts", s.handleListHosts)
	shared.GET("/hosts/:hostUID", s.handleGetHost)
	shared.GET("/hosts/:hostUID/metrics/latest", s.handlePublicLatestMetrics)
	shared.GET("/stream/hosts", s.handleHostStream)

	demo := engine.Group("/master/api/v1/public/demo")
	demo.Use(s.requireDemo())
	demo.GET("/session", s.handleShareInfo)
	demo.GET("/hosts", s.handleListHosts)
	demo.GET("/hosts/:hostUID", s.handleGetHost)
	demo.GET("/hosts/:hostUID/metrics/latest", s.handlePublicLatestMetrics)
	demo.GET("/stream/hosts", s.handleHostStream)

	protected := engine.Group("/master/api/v1")
	protected.Use(s.requireSession(), s.requireAllowedOrigin(), s.requireCSRF())
	protected.POST("/ops/maintenance", s.handleCreateMaintenance)
	protected.POST("/ops/alerts/:alertID/ack", s.handleAckAlert)
	protected.GET("/shares", requireAdmin(), s.handleListShares)
	protected.POST("/shares", requireAdmin(), s.handleCreateShare)
	protected.DELETE("/shares/:shareID", requireAdmin(), s.handleRevokeShare)

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
	p := principalFromContext(c)
	items, err := s.svc.ReadHosts(c.Request.Context(), p)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"items": service.ProjectHosts(p, items), "next_cursor": nil})
}

func (s *Server) handleGetHost(c *gin.Context) {
	hostUID := strings.TrimSpace(c.Param("hostUID"))
	if hostUID == "" {
		writeError(c, errors.New("missing host uid"))
		return
	}

	p := principalFromContext(c)
	host, ok, err := s.svc.ReadHost(c.Request.Context(), p, hostUID)
	if err != nil {
		writeError(c, err)
		return
	}
	if !ok {
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	c.JSON(nethttp.StatusOK, service.ProjectHost(p, host))
}

func (s *Server) handleGetLatestMetricSnapshot(c *gin.Context) {
	hostUID := strings.TrimSpace(c.Param("hostUID"))
	if hostUID == "" {
		writeError(c, errors.New("missing host uid"))
		return
	}
	tenantCode := tenantCodeFromContext(c)
	if tenantCode == "" {
		writeError(c, errors.New("missing tenant"))
		return
	}

	_, ok, err := s.svc.GetHost(c.Request.Context(), hostUID, tenantCode)
	if err != nil {
		s.logger.Error("verify metric snapshot host", "host_uid", hostUID, "error", err)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to verify host"})
		return
	}
	if !ok {
		c.JSON(nethttp.StatusNotFound, gin.H{"error": "host not found"})
		return
	}

	snapshot, found, err := s.svc.GetMetricSnapshot(c.Request.Context(), hostUID)
	if err != nil {
		s.logger.Error("load latest metric snapshot", "host_uid", hostUID, "error", err)
		c.JSON(nethttp.StatusInternalServerError, gin.H{"error": "failed to load metric snapshot"})
		return
	}
	c.Header("Cache-Control", "no-store")
	if !found {
		c.JSON(nethttp.StatusOK, gin.H{"snapshot": nil})
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"snapshot": snapshot})
}

func (s *Server) handleCreateMaintenance(c *gin.Context) {
	var req contracts.CreateMaintenanceWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, err)
		return
	}

	resp, err := s.svc.CreateMaintenance(c.Request.Context(), principalFromContext(c), req)
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

	resp, err := s.svc.AckAlert(c.Request.Context(), principalFromContext(c), alertID)
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

func (s *Server) handleLogout(c *gin.Context) {
	if err := s.svc.Logout(c.Request.Context(), sessionTokenFromRequest(c, s.cookieName())); err != nil {
		writeError(c, err)
		return
	}
	s.clearSessionCookie(c)
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
	return ""
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
		if user.ID <= 0 || user.TenantCode == "" || user.Status != auth.UserStatusActive {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		if requested := strings.TrimSpace(c.Query("tenant")); requested != "" && requested != user.TenantCode {
			writeError(c, service.ErrForbidden)
			c.Abort()
			return
		}
		c.Header("Cache-Control", "no-store")
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

func principalFromContext(c *gin.Context) access.Principal {
	if value, ok := c.Get(principalContextKey); ok {
		if principal, ok := value.(access.Principal); ok {
			return principal
		}
	}
	user := currentUser(c)
	return access.Principal{Kind: access.User, TenantCode: user.TenantCode, UserID: user.ID, Role: string(user.Role)}
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

func (s *Server) setSessionCookie(c *gin.Context, value string, expiresAt time.Time) {
	c.SetSameSite(nethttp.SameSiteLaxMode)
	c.SetCookie(s.cookieName(), value, max(1, int(time.Until(expiresAt).Seconds())), "/", "", s.svc.CookieSecure(), true)
}

func (s *Server) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(nethttp.SameSiteLaxMode)
	c.SetCookie(s.cookieName(), "", -1, "/", "", s.svc.CookieSecure(), true)
}

func writeError(c *gin.Context, err error) {
	status := nethttp.StatusBadRequest
	code, message := "invalid_request", "invalid request"
	var databaseError *pgconn.PgError
	switch {
	case err == nil:
		status, code, message = 500, "internal_error", "request failed"
	case errors.Is(err, service.ErrUnauthorized), errors.Is(err, access.ErrUnauthorized):
		status, code, message = 401, "unauthorized", "unauthorized"
	case errors.Is(err, access.ErrQuota):
		c.Header("Retry-After", "60")
		status, code, message = 429, "quota_exceeded", "access quota exceeded"
	case errors.Is(err, service.ErrForbidden), errors.Is(err, access.ErrForbidden):
		status, code, message = 403, "forbidden", "forbidden"
	case errors.Is(err, access.ErrNotFound):
		status, code, message = 404, "not_found", "resource not found"
	case errors.Is(err, service.ErrAuthNotConfigured):
		status, code, message = 503, "unavailable", "auth is not configured"
	case errors.As(err, &databaseError):
		status, code, message = 500, "internal_error", "storage unavailable"
	default:
		message = err.Error()
	}
	c.JSON(status, gin.H{"code": code, "message": message, "error": message, "request_id": ids.New("req")})
}
