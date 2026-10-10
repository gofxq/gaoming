// The admin command runs explicit, offline administrative tasks.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofxq/gaoming/pkg/clock"
	"github.com/gofxq/gaoming/pkg/migrations"
	"github.com/gofxq/gaoming/services/master-api/internal/auth"
	"github.com/gofxq/gaoming/services/master-api/internal/config"
	"github.com/gofxq/gaoming/services/master-api/internal/service"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	flags := flag.NewFlagSet("gaoming-admin", flag.ContinueOnError)
	configFile := flags.String("config", config.DefaultConfigFile, "master-api configuration file")
	timeout := flags.Duration("timeout", 2*time.Minute, "maximum command duration, including lock wait")
	tenant := flags.String("tenant", "", "administrator tenant code")
	username := flags.String("username", "", "administrator username")
	displayName := flags.String("display-name", "", "administrator display name")
	passwordStdin := flags.Bool("password-stdin", false, "read administrator password from standard input")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 1 || (flags.Arg(0) != "migrate" && flags.Arg(0) != "schema-check" && flags.Arg(0) != "create-admin") {
		return fmt.Errorf("usage: gaoming-admin [options] migrate|schema-check|create-admin")
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	dsn := os.Getenv("GAOMING_POSTGRES_DSN")
	if dsn == "" {
		cfg, err := config.LoadFromFile(*configFile)
		if err != nil {
			return err
		}
		dsn = cfg.PostgresDSN
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("invalid database configuration")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection failed (check configuration, network and credentials)")
	}
	if flags.Arg(0) == "create-admin" {
		if !*passwordStdin || *tenant == "" || *username == "" || *displayName == "" {
			return fmt.Errorf("create-admin requires -tenant, -username, -display-name and -password-stdin")
		}
		if err := migrations.Check(ctx, db); err != nil {
			return err
		}
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 75))
		if err != nil || len(raw) > 74 {
			return fmt.Errorf("password input unavailable or too long")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
		orm, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			return fmt.Errorf("database unavailable")
		}
		svc := service.New(nil, nil, nil, nil, auth.NewStore(orm), service.AuthConfig{}, clock.Real{}, nil)
		_, err = svc.CreateAdmin(ctx, service.SetupRequest{TenantCode: *tenant, Username: *username, DisplayName: *displayName, Password: password})
		if err != nil {
			return err
		}
		fmt.Println("administrator account created")
		return nil
	}
	if flags.Arg(0) == "schema-check" {
		err = migrations.Check(ctx, db)
	} else {
		err = migrations.Run(ctx, db)
	}
	if err != nil {
		return err
	}
	fmt.Println("database schema is current")
	return nil
}
