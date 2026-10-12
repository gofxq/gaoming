// Package runtime composes the existing probe worker within a shared process.
package runtime

import (
	"context"
	"github.com/gofxq/gaoming/pkg/logx"
	"github.com/gofxq/gaoming/services/probe-worker/internal/service"
)

type Config = service.Config

func Run(ctx context.Context, cfg Config, logger *logx.Logger) error {
	return service.NewRunner(cfg, logger).Run(ctx)
}
