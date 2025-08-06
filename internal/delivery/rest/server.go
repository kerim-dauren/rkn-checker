package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/application"
)

type Server struct {
	server          *http.Server
	blockingService application.BlockingChecker
	port            int
}

func NewServer(blockingService application.BlockingChecker, port int) *Server {
	return &Server{
		blockingService: blockingService,
		port:            port,
	}
}

func (s *Server) Start(ctx context.Context) error {
	handler := NewHandler(s.blockingService)

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/check", handler.CheckURL)
	mux.HandleFunc("/api/v1/stats", handler.GetStats)
	mux.HandleFunc("/health", handler.HealthCheck)

	// Apply middleware chain
	finalHandler := CORSMiddleware(LoggingMiddleware(RecoveryMiddleware(mux)))

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	slog.Info("Starting REST server", "port", s.port)

	go func() {
		<-ctx.Done()
		slog.Info("Shutting down REST server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown REST server gracefully", "error", err)
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("REST server failed: %w", err)
	}

	return nil
}

func (s *Server) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}
