package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/grpc/proto"
	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

type Handler struct {
	proto.UnimplementedBlockingServiceServer
	blockingService application.BlockingChecker
}

func NewHandler(blockingService application.BlockingChecker) *Handler {
	return &Handler{
		blockingService: blockingService,
	}
}

func (h *Handler) CheckURL(ctx context.Context, req *proto.CheckURLRequest) (*proto.CheckURLResponse, error) {
	if req.Url == "" {
		return nil, status.Error(codes.InvalidArgument, "URL is required")
	}

	result, err := h.blockingService.CheckURL(ctx, req.Url)
	if err != nil {
		switch err {
		case domain.ErrEmptyURL:
			return nil, status.Error(codes.InvalidArgument, "URL is empty")
		case domain.ErrInvalidURL:
			return nil, status.Error(codes.InvalidArgument, "Invalid URL format")
		default:
			return nil, status.Error(codes.Internal, "Internal server error")
		}
	}

	response := &proto.CheckURLResponse{
		Blocked:       result.IsBlocked,
		NormalizedUrl: result.NormalizedURL,
		Reason:        "",
		Match:         "",
	}

	if result.IsBlocked {
		response.Reason = result.Reason.String()
		if result.Rule != nil {
			response.Match = result.Rule.Pattern
		}
	}

	return response, nil
}

func (h *Handler) GetStats(ctx context.Context, req *proto.GetStatsRequest) (*proto.GetStatsResponse, error) {
	stats, err := h.blockingService.GetStats(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get statistics")
	}

	return &proto.GetStatsResponse{
		TotalEntries:    stats.TotalEntries,
		DomainEntries:   stats.DomainEntries,
		WildcardEntries: stats.WildcardEntries,
		IpEntries:       stats.IPEntries,
		UrlPatterns:     stats.URLPatterns,
		LastUpdate:      stats.LastUpdate,
		Version:         stats.Version,
	}, nil
}

func (h *Handler) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{
		Status:  proto.HealthCheckResponse_SERVING,
		Message: "Service is healthy",
	}, nil
}
