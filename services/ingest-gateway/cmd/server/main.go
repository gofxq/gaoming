package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofxq/gaoming/services/ingest-gateway/internal/app"
	"google.golang.org/grpc"
)

func main() {
	application, err := app.New()
	if err != nil {
		panic(err)
	}

	go func() {
		if err := application.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			panic(err)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = application.Shutdown(ctx)
	os.Exit(0)
}
