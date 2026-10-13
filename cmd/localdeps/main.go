package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofxq/gaoming/internal/localdeps"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func execute() error {
	config := flag.String("config", "config/server.yml", "development server configuration")
	dir := flag.String("dir", ".local-data", "managed dependency data directory")
	template := flag.String("template", "deployments/server-config.local.yaml", "initial config template")
	compose := flag.String("docker-compose", "docker compose", "Docker Compose command")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: localdeps [options] init|up|down|clean")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	manager := localdeps.Manager{Dir: *dir}
	switch flag.Arg(0) {
	case "init":
		if err := localdeps.InitConfig(*config, *template); err != nil {
			return err
		}
		fmt.Printf("Configuration ready: %s (existing contents preserved)\n", *config)
		return nil
	case "up":
		cfg, err := localdeps.LoadConfig(*config)
		if err != nil {
			return err
		}
		return manager.DockerUp(ctx, cfg, *compose)
	case "clean":
		return manager.Clean()
	case "down":
		return manager.DockerDown(ctx, *compose)
	default:
		return fmt.Errorf("usage: localdeps [options] init|up|down|clean")
	}
}
