package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/grpc/proto"
)

type Server struct {
	server          *grpc.Server
	blockingService application.BlockingChecker
	port            int
}

func NewServer(blockingService application.BlockingChecker, port int) *Server {
	keepaliveParams := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second,
		MaxConnectionAge:      30 * time.Second,
		MaxConnectionAgeGrace: 5 * time.Second,
		Time:                  5 * time.Second,
		Timeout:               1 * time.Second,
	}

	keepalivePolicy := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepaliveParams),
		grpc.KeepaliveEnforcementPolicy(keepalivePolicy),
		grpc.ChainUnaryInterceptor(recoveryInterceptor, loggingInterceptor),
	}

	server := grpc.NewServer(opts...)

	return &Server{
		server:          server,
		blockingService: blockingService,
		port:            port,
	}
}

func (s *Server) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	handler := NewHandler(s.blockingService)
	proto.RegisterBlockingServiceServer(s.server, handler)

	slog.Info("Starting gRPC server", "port", s.port)

	go func() {
		<-ctx.Done()
		slog.Info("Shutting down gRPC server...")
		s.server.GracefulStop()
	}()

	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}

func (s *Server) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}
