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

	application, err := app.New()
	if err != nil {
		panic(err)
	}
	if err := application.Run(sigCtx); err != nil {
		panic(err)
	}
}
