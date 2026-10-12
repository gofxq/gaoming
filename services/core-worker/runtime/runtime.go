// Package runtime composes the existing core worker within a shared process.
package runtime

import (
	"context"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/core-worker/internal/service"
	"time"
)

func Run(ctx context.Context, interval time.Duration, logger *logx.Logger) error {
	return service.NewRunner(logger, interval).Run(ctx)
}
