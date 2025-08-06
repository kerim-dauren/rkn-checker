package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/grpc"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/rest"
	"github.com/kerim-dauren/rkn-checker/internal/domain/services"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/config"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/registry"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/storage"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/updater"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	setupLogging(cfg.Logging)

	slog.Info("Starting Roskomnadzor URL Blocking Service")

	normalizer := services.NewURLNormalizer()
	store := storage.NewMemoryStore()
	blockingService := application.NewBlockingService(normalizer, store)

	registryClientConfig := registry.ClientConfig{
		Sources:       cfg.Registry.Sources,
		MaxConcurrent: cfg.Registry.MaxConcurrent,
		Timeout:       cfg.Registry.Timeout,
	}

	registryClient, err := registry.NewClient(registryClientConfig)
	if err != nil {
		slog.Error("Failed to create registry client", "error", err)
		os.Exit(1)
	}

	scheduler := updater.NewScheduler(registryClient, store, cfg.Registry.UpdateConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting registry update scheduler")
	go func() {
		if err := scheduler.Start(ctx); err != nil {
			slog.Error("Registry update scheduler failed", "error", err)
		}
	}()

	grpcServer := grpc.NewServer(blockingService, cfg.Server.GRPCPort)
	restServer := rest.NewServer(blockingService, cfg.Server.RESTPort)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcServer.Start(ctx); err != nil {
			slog.Error("gRPC server failed", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := restServer.Start(ctx); err != nil {
			slog.Error("REST server failed", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Shutdown signal received")

	cancel()

	slog.Info("Waiting for servers to shut down...")
	wg.Wait()

	slog.Info("Service stopped")
}

func setupLogging(cfg config.LoggingConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		slog.Error("Invalid log level", "level", cfg.Level)
		os.Exit(1)
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	slog.SetDefault(slog.New(handler))
}
