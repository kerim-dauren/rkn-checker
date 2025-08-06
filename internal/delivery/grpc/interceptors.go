package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(start)
	code := codes.OK
	if err != nil {
		if st, ok := status.FromError(err); ok {
			code = st.Code()
		} else {
			code = codes.Unknown
		}
	}

	if err != nil {
		slog.Error("gRPC request failed",
			"method", info.FullMethod,
			"duration", duration.String(),
			"code", code.String(),
			"error", err.Error())
	} else {
		slog.Info("gRPC request completed",
			"method", info.FullMethod,
			"duration", duration.String(),
			"code", code.String())
	}

	return resp, err
}

func recoveryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("gRPC request panicked",
				"method", info.FullMethod,
				"panic", r)

			err = status.Error(codes.Internal, "Internal server error")
		}
	}()

	return handler(ctx, req)
}
