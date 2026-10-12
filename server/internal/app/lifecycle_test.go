package app

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestRunCancellationClosesListeners(t *testing.T) {
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{httpServer: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })}, grpcServer: grpc.NewServer()}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.serve(ctx, httpListener, grpcListener) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("shutdown blocked")
	}
	for _, addr := range []string{httpListener.Addr().String(), grpcListener.Addr().String()} {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			t.Fatal("listener remained open")
		}
	}
}

func TestRunBindFailureReleasesHTTPPort(t *testing.T) {
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpAddr := httpListener.Addr().String()
	httpListener.Close()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	a := &App{httpServer: &http.Server{Addr: httpAddr}, grpcServer: grpc.NewServer(), grpcAddr: occupied.Addr().String()}
	if err := a.Run(context.Background()); err == nil {
		t.Fatal("occupied gRPC port accepted")
	}
	listener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		t.Fatal("HTTP port leaked", err)
	}
	listener.Close()
}
