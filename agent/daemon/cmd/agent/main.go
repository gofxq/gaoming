package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/gofxq/gaoming/agent/daemon/internal/app"
)

func main() {
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application := app.New()
	if err := application.Run(sigCtx); err != nil {
		panic(err)
	}
}
