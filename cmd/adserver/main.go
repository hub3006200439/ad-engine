package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"ad-engine/internal/config"
	httpapi "ad-engine/internal/http"
	"ad-engine/internal/logger"
	"ad-engine/internal/migrate"
	"ad-engine/internal/repo/postgres"
	"ad-engine/internal/usecase"
)

func main() {
	var configPath string

	flag.StringVar(&configPath, "config", "", "path to config TOML file (overrides CONFIG_PATH and default configs/config.toml)")
	flag.Parse()

	if err := run(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		logger.Shutdown()

		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {

	}

	if cfg.Server.Params.MaxProcs > 0 {
		runtime.GOMAXPROCS(cfg.Server.Params.MaxProcs)
	}

	if err := logger.Init(cfg.Logger); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	logger.LogInfo("logger initialized")

	pool, err := postgres.NewDB(cfg.PG)
	if err != nil {
		logger.LogError("failed to init db: {err}", err)
		return fmt.Errorf("init db: %w", err)
	}
	defer pool.Close()
	logger.LogInfo("db connection pool initialized")

	if err := migrate.Run(context.Background(), pool); err != nil {
		logger.LogError("db migrations failed: {err}", err)
		return fmt.Errorf("migrations: %w", err)
	}
	logger.LogInfo("db migrations applied (if MIGRATE enabled)")

	freqRepo := postgres.NewFreqRepository(pool)
	freqSvc := usecase.NewFrequencyService(freqRepo)

	adRepo := postgres.NewAdRepository(pool, freqSvc)
	statsRepo := postgres.NewStatsRepository(pool)

	adSvc := usecase.NewAdService(adRepo, freqSvc)
	statsSvc := usecase.NewStatsService(statsRepo)

	// HTTP-сервер
	apiServer := httpapi.NewServer(adSvc, statsSvc)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	fhServer := httpapi.BuildHTTPServer(cfg.Server, apiServer)

	logger.LogInfo("adengine server starting on {addr}", addr)

	errCh := make(chan error, 1)

	go func() {
		if err := fhServer.ListenAndServe(addr); err != nil {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.LogInfo("received shutdown signal: {sig}", sig.String())
	case err := <-errCh:
		logger.LogError("http server error: {err}", err)
		return fmt.Errorf("http server: %w", err)
	}

	logger.LogInfo("adengine server shutting down")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		if err := fhServer.Shutdown(); err != nil {
			logger.LogError("http server shutdown error: {err}", err)
		} else {
			logger.LogInfo("http server shutdown complete")
		}
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
		logger.LogWarning("http server shutdown timed out")
	}

	return nil
}
