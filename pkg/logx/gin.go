package logx

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func GinMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		kv := []any{
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"client_ip", c.ClientIP(),
			"latency_ms", time.Since(started).Milliseconds(),
			"body_bytes", c.Writer.Size(),
			"user_agent", c.Request.UserAgent(),
		}
		if errText := strings.TrimSpace(c.Errors.String()); errText != "" {
			kv = append(kv, "error", errText)
		}

		switch status := c.Writer.Status(); {
		case status >= 500:
			logger.Error("http access", kv...)
		case status >= 400:
			logger.Warn("http access", kv...)
		default:
			logger.Info("http access", kv...)
		}
	}
}
