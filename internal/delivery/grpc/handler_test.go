package grpc

import (
	"context"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/application"
	"github.com/kerim-dauren/rkn-checker/internal/delivery/grpc/proto"
	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

type mockBlockingService struct {
	checkURLFunc func(ctx context.Context, rawURL string) (*domain.BlockingResult, error)
	getStatsFunc func(ctx context.Context) (*application.BlockingStats, error)
}

func (m *mockBlockingService) CheckURL(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
	if m.checkURLFunc != nil {
		return m.checkURLFunc(ctx, rawURL)
	}
	return domain.NewBlockingResult(false, rawURL, nil), nil
}

func (m *mockBlockingService) GetStats(ctx context.Context) (*application.BlockingStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}
	return &application.BlockingStats{
		TotalEntries:    1000,
		DomainEntries:   500,
		WildcardEntries: 300,
		IPEntries:       200,
		URLPatterns:     0,
		LastUpdate:      "2024-01-01T00:00:00Z",
		Version:         "v1.0.0",
	}, nil
}

func TestHandler_CheckURL(t *testing.T) {
	tests := []struct {
		name    string
		request *proto.CheckURLRequest
		setup   func(*mockBlockingService)
		wantErr bool
	}{
		{
			name:    "empty URL should return error",
			request: &proto.CheckURLRequest{Url: ""},
			wantErr: true,
		},
		{
			name:    "valid URL should return success",
			request: &proto.CheckURLRequest{Url: "https://example.com"},
			setup: func(m *mockBlockingService) {
				m.checkURLFunc = func(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
					return domain.NewBlockingResult(false, "example.com", nil), nil
				}
			},
			wantErr: false,
		},
		{
			name:    "blocked URL should return blocked result",
			request: &proto.CheckURLRequest{Url: "https://blocked.com"},
			setup: func(m *mockBlockingService) {
				m.checkURLFunc = func(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
					rule, _ := domain.NewBlockingRule(domain.BlockingTypeDomain, "blocked.com")
					result := domain.NewBlockingResult(true, "blocked.com", rule)
					return result, nil
				}
			},
			wantErr: false,
		},
		{
			name:    "invalid URL should return error",
			request: &proto.CheckURLRequest{Url: "not-a-url"},
			setup: func(m *mockBlockingService) {
				m.checkURLFunc = func(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
					return nil, domain.ErrInvalidURL
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockBlockingService{}
			if tt.setup != nil {
				tt.setup(mockService)
			}

			handler := NewHandler(mockService)
			resp, err := handler.CheckURL(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Expected response, but got nil")
				return
			}

			if tt.request.Url == "https://blocked.com" && !resp.Blocked {
				t.Errorf("Expected blocked=true, but got false")
			}

			if tt.request.Url == "https://example.com" && resp.Blocked {
				t.Errorf("Expected blocked=false, but got true")
			}
		})
	}
}

func TestHandler_GetStats(t *testing.T) {
	mockService := &mockBlockingService{}
	handler := NewHandler(mockService)

	resp, err := handler.GetStats(context.Background(), &proto.GetStatsRequest{})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if resp == nil {
		t.Errorf("Expected response, but got nil")
		return
	}

	if resp.TotalEntries != 1000 {
		t.Errorf("Expected TotalEntries=1000, but got %d", resp.TotalEntries)
	}

	if resp.DomainEntries != 500 {
		t.Errorf("Expected DomainEntries=500, but got %d", resp.DomainEntries)
	}
}

func TestHandler_HealthCheck(t *testing.T) {
	mockService := &mockBlockingService{}
	handler := NewHandler(mockService)

	resp, err := handler.HealthCheck(context.Background(), &proto.HealthCheckRequest{})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if resp == nil {
		t.Errorf("Expected response, but got nil")
		return
	}

	if resp.Status != proto.HealthCheckResponse_SERVING {
		t.Errorf("Expected status=SERVING, but got %v", resp.Status)
	}

	if resp.Message != "Service is healthy" {
		t.Errorf("Expected message='Service is healthy', but got %s", resp.Message)
	}
}
