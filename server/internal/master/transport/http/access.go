package http

import (
	"context"
	nethttp "net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofxq/gaoming/pkg/access"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/server/internal/master/service"
)

const principalContextKey = "access_principal"
const shareCookieName = "gaoming_share"
const shareCookiePath = "/master/api/v1/public/shares"

func (s *Server) requireShare() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		p, err := s.svc.AuthenticateShareSession(c.Request.Context(), sessionTokenFromRequest(c, shareCookieName))
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
		if tenant := strings.TrimSpace(c.Query("tenant")); tenant != "" && tenant != p.TenantCode {
			writeError(c, service.ErrForbidden)
			c.Abort()
			return
		}
		c.Set(principalContextKey, p)
		c.Next()
	}
}

func (s *Server) requireDemo() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		p, err := s.svc.DemoPrincipal(c.Request.Context())
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
		if tenant := strings.TrimSpace(c.Query("tenant")); tenant != "" && tenant != p.TenantCode {
			writeError(c, service.ErrForbidden)
			c.Abort()
			return
		}
		c.Set(principalContextKey, p)
		c.Next()
	}
}

func (s *Server) recheckReadAccess(c *gin.Context, p access.Principal) bool {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	switch p.Kind {
	case access.User:
		user, _, ok, err := s.svc.ResolveSessionUser(ctx, sessionTokenFromRequest(c, s.cookieName()))
		return err == nil && ok && user.ID == p.UserID && user.TenantCode == p.TenantCode && string(user.Role) == p.Role
	case access.Share:
		current, err := s.svc.AuthenticateShareSession(ctx, sessionTokenFromRequest(c, shareCookieName))
		return err == nil && current.ShareID == p.ShareID && current.TenantCode == p.TenantCode && reflect.DeepEqual(current.AllowedHosts, p.AllowedHosts) && reflect.DeepEqual(current.Fields, p.Fields)
	case access.Demo:
		current, err := s.svc.DemoPrincipal(ctx)
		return err == nil && reflect.DeepEqual(current, p)
	default:
		return false
	}
}

func (s *Server) handleCreateShare(c *gin.Context) {
	var req access.CreateShareRequest
	if !bindAuthJSON(c, &req) {
		return
	}
	result, err := s.svc.CreateShare(c.Request.Context(), principalFromContext(c), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(201, result)
}

func (s *Server) handleListShares(c *gin.Context) {
	items, err := s.svc.ListShares(c.Request.Context(), principalFromContext(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "next_cursor": nil})
}

func (s *Server) handleRevokeShare(c *gin.Context) {
	if err := s.svc.RevokeShare(c.Request.Context(), principalFromContext(c), strings.TrimSpace(c.Param("shareID"))); err != nil {
		writeError(c, err)
		return
	}
	c.Status(204)
}

func (s *Server) handleExchangeShare(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if !bindAuthJSON(c, &req) {
		return
	}
	session, err := s.svc.ExchangeShare(c.Request.Context(), req.Token)
	if err != nil {
		writeError(c, err)
		return
	}
	c.SetSameSite(nethttp.SameSiteLaxMode)
	c.SetCookie(shareCookieName, session.Token, max(1, int(time.Until(session.ExpiresAt).Seconds())), shareCookiePath, "", s.svc.CookieSecure(), true)
	c.JSON(200, gin.H{"expires_at": session.ExpiresAt, "fields": session.Principal.Fields})
}

func (s *Server) handleShareInfo(c *gin.Context) {
	p := principalFromContext(c)
	c.JSON(200, gin.H{"mode": p.Kind, "fields": p.Fields, "host_count": len(p.AllowedHosts)})
}

// Public metric views expose only explicitly shared fixed summaries. Dynamic
// labels, filesystem/device identities and collector details remain private.
func (s *Server) handlePublicLatestMetrics(c *gin.Context) {
	p := principalFromContext(c)
	if !p.AllowsField("metrics") {
		writeError(c, service.ErrForbidden)
		return
	}
	host, found, err := s.svc.ReadHost(c.Request.Context(), p, strings.TrimSpace(c.Param("hostUID")))
	if err != nil {
		writeError(c, err)
		return
	}
	if !found {
		writeError(c, access.ErrNotFound)
		return
	}
	if host.LastMetricAt.IsZero() {
		c.JSON(200, gin.H{"snapshot": nil})
		return
	}
	points := make([]contracts.MetricPoint, 0, 16)
	for name, point := range latestMetricPointsFromSnapshot(host) {
		points = append(points, contracts.MetricPoint{Name: string(name), Value: point.Value, TS: point.TS})
	}
	c.JSON(200, gin.H{"snapshot": contracts.MetricSnapshot{CollectedAt: host.LastMetricAt, Points: points}})
}
