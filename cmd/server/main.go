package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aillion-co/infrastructure-lz/internal/api"
	"github.com/aillion-co/infrastructure-lz/internal/config"
	"github.com/aillion-co/infrastructure-lz/internal/generator"
	"github.com/aillion-co/infrastructure-lz/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	shutdown, err := telemetry.Setup(ctx, "iac-generator", config.Version, cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("setting up telemetry: %w", err)
	}
	defer shutdown(context.Background())

	gen := generator.New()
	activationGen := generator.NewActivationGenerator()
	router := api.NewRouter(gen, activationGen)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port, "version", config.Version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
		slog.Info("shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
	}

	return nil
}
