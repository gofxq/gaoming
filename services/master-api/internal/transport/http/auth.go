package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
)

func (s *Server) requireAllowedOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if c.Request.Method == nethttp.MethodGet || c.Request.Method == nethttp.MethodHead || c.Request.Method == nethttp.MethodOptions {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		origins := s.svc.AllowedOrigins()
		if len(origins) == 0 {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			origins = []string{scheme + "://" + c.Request.Host}
		}
		for _, allowed := range origins {
			if origin == allowed {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(nethttp.StatusForbidden, gin.H{"error": "request origin is not allowed"})
	}
}

func csrfToken(sessionToken string) string {
	mac := hmac.New(sha256.New, []byte(sessionToken))
	_, _ = mac.Write([]byte("gaoming-csrf-v1"))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) requireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == nethttp.MethodGet || c.Request.Method == nethttp.MethodHead || c.Request.Method == nethttp.MethodOptions {
			c.Next()
			return
		}
		raw := sessionTokenFromRequest(c, s.cookieName())
		expected := csrfToken(raw)
		if raw == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-CSRF-Token")), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(nethttp.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}
		c.Next()
	}
}

func (s *Server) handleCSRF(c *gin.Context) {
	c.JSON(nethttp.StatusOK, gin.H{"token": csrfToken(sessionTokenFromRequest(c, s.cookieName()))})
}

func (s *Server) handleSetupStatus(c *gin.Context) {
	available, err := s.svc.SetupAvailable(c.Request.Context())
	if err != nil {
		c.JSON(nethttp.StatusServiceUnavailable, gin.H{"error": "setup status unavailable"})
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"available": available})
}

func bindAuthJSON(c *gin.Context, value any) bool {
	if c.ContentType() != "application/json" {
		c.JSON(nethttp.StatusUnsupportedMediaType, gin.H{"error": "application/json required"})
		return false
	}
	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if err := c.ShouldBindJSON(value); err != nil {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "invalid authentication request"})
		return false
	}
	return true
}

func (s *Server) handleSetup(c *gin.Context) {
	var req service.SetupRequest
	if !bindAuthJSON(c, &req) {
		return
	}
	session, err := s.svc.Setup(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	s.setSessionCookie(c, session.Token, session.ExpiresAt)
	c.JSON(nethttp.StatusCreated, service.AuthSessionView{Authenticated: true, User: session.User})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req service.LoginRequest
	if !bindAuthJSON(c, &req) {
		return
	}
	if len(req.Username) > 64 || len(req.TenantCode) > 64 || len(req.Password) > 72 || strings.TrimSpace(req.Username) == "" {
		writeError(c, service.ErrUnauthorized)
		return
	}
	session, err := s.svc.Login(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	// A successful login replaces this browser's old session instead of leaving
	// the displaced cookie credential usable until its TTL expires.
	if old := sessionTokenFromRequest(c, s.cookieName()); old != "" {
		if err := s.svc.Logout(c.Request.Context(), old); err != nil {
			_ = s.svc.Logout(c.Request.Context(), session.Token)
			c.JSON(nethttp.StatusServiceUnavailable, gin.H{"error": "session rotation failed"})
			return
		}
	}
	s.setSessionCookie(c, session.Token, session.ExpiresAt)
	c.JSON(nethttp.StatusOK, service.AuthSessionView{Authenticated: true, User: session.User})
}
