package main

import (
	"context"
	"errors"
	"flag"
	platform "github.com/enterprise-labs/seismic-event-associator/internal/platform/application"
	runtimepkg "github.com/enterprise-labs/seismic-event-associator/internal/platform/infrastructure"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	path := flag.String("config", "configs/local.yaml", "configuration path")
	flag.Parse()
	cfg, err := platform.Load(*path)
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	runtime, err := runtimepkg.NewRuntime(ctx, cfg, logger)
	if err != nil {
		logger.Error("runtime failed", "error", err)
		os.Exit(1)
	}
	done := make(chan error, 1)
	go func() {
		logger.Info("seismic event associator started", "address", cfg.Address)
		done <- runtime.Server.ListenAndServe()
	}()
	select {
	case err = <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		stopCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err = runtime.Shutdown(stopCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
			os.Exit(1)
		}
		err = <-done
	}
	logger.Info("seismic event associator stopped")
}
