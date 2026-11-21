// Command api runs the ORTALYQ-NEW HTTP server.
//
//	api serve            start HTTP server (default)
//	api migrate up|down|status|reset
//	api seed             local demo data (APP_ENV=local only)
//	api create-admin <email> <full name>   first administrator; password is read from stdin
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ortalyq/backend/internal/config"
	"ortalyq/backend/internal/database"
	"ortalyq/backend/internal/router"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "migrate":
		sub := "up"
		if len(args) > 1 {
			sub = args[1]
		}
		return database.Migrate(cfg.DatabaseURL, sub)
	case "seed":
		return seed(cfg)
	case "create-admin":
		return createAdmin(cfg, args[1:], os.Stdin)
	case "serve":
		return serve(cfg)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func serve(cfg config.Config) error {
	if cfg.AutoMigrate {
		if err := database.Migrate(cfg.DatabaseURL, "up"); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	app, err := router.New(cfg, pool)
	if err != nil {
		return err
	}
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = app.Auth.Cleanup(ctx)
			}
		}
	}()
	go app.Mail.Run(ctx, 15*time.Second)
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: app.Engine, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	slog.Info("listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
