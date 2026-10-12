package http

import (
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type authAttempt struct {
	started time.Time
	count   int
}
type authGuard struct {
	mu       sync.Mutex
	attempts map[string]authAttempt
	slots    chan struct{}
}

func newAuthGuard() *authGuard {
	return &authGuard{attempts: make(map[string]authAttempt), slots: make(chan struct{}, 4)}
}

func (g *authGuard) allow(peer string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, attempt := range g.attempts {
		if now.Sub(attempt.started) >= time.Minute {
			delete(g.attempts, key)
		}
	}
	attempt, exists := g.attempts[peer]
	if !exists {
		if len(g.attempts) >= 1024 {
			return false
		}
		attempt = authAttempt{started: now}
	}
	if attempt.count >= 20 {
		return false
	}
	attempt.count++
	g.attempts[peer] = attempt
	return true
}

func (s *Server) limitAuthWork() gin.HandlerFunc {
	return func(c *gin.Context) {
		peer, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			peer = c.Request.RemoteAddr
		}
		// Never trust arbitrary forwarding headers to choose the quota identity.
		if !s.authGuard.allow(peer, time.Now()) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(429, gin.H{"error": "too many authentication attempts"})
			return
		}
		select {
		case s.authGuard.slots <- struct{}{}:
			defer func() { <-s.authGuard.slots }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(429, gin.H{"error": "authentication is busy"})
		}
	}
}
