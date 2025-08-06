package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerim-dauren/rkn-checker/internal/application"
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
		name            string
		method          string
		body            interface{}
		setup           func(*mockBlockingService)
		expectedStatus  int
		expectedBlocked *bool
	}{
		{
			name:            "valid URL should return success",
			method:          http.MethodPost,
			body:            CheckURLRequest{URL: "https://example.com"},
			expectedStatus:  http.StatusOK,
			expectedBlocked: func() *bool { b := false; return &b }(),
		},
		{
			name:            "blocked URL should return blocked result",
			method:          http.MethodPost,
			body:            CheckURLRequest{URL: "https://blocked.com"},
			expectedStatus:  http.StatusOK,
			expectedBlocked: func() *bool { b := true; return &b }(),
			setup: func(m *mockBlockingService) {
				m.checkURLFunc = func(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
					rule, _ := domain.NewBlockingRule(domain.BlockingTypeDomain, "blocked.com")
					result := domain.NewBlockingResult(true, "blocked.com", rule)
					return result, nil
				}
			},
		},
		{
			name:           "empty URL should return bad request",
			method:         http.MethodPost,
			body:           CheckURLRequest{URL: ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid URL should return bad request",
			method:         http.MethodPost,
			body:           CheckURLRequest{URL: "not-a-url"},
			expectedStatus: http.StatusBadRequest,
			setup: func(m *mockBlockingService) {
				m.checkURLFunc = func(ctx context.Context, rawURL string) (*domain.BlockingResult, error) {
					return nil, domain.ErrInvalidURL
				}
			},
		},
		{
			name:           "invalid JSON should return bad request",
			method:         http.MethodPost,
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET method should return method not allowed",
			method:         http.MethodGet,
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockBlockingService{}
			if tt.setup != nil {
				tt.setup(mockService)
			}

			handler := NewHandler(mockService)

			var body bytes.Buffer
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body.WriteString(str)
				} else {
					json.NewEncoder(&body).Encode(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/api/v1/check", &body)
			w := httptest.NewRecorder()

			handler.CheckURL(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, but got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBlocked != nil && w.Code == http.StatusOK {
				var resp CheckURLResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode response: %v", err)
					return
				}

				if resp.Blocked != *tt.expectedBlocked {
					t.Errorf("Expected blocked=%t, but got %t", *tt.expectedBlocked, resp.Blocked)
				}
			}
		})
	}
}

func TestHandler_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET method should return success",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST method should return method not allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockBlockingService{}
			handler := NewHandler(mockService)

			req := httptest.NewRequest(tt.method, "/api/v1/stats", nil)
			w := httptest.NewRecorder()

			handler.GetStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, but got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp StatsResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode response: %v", err)
					return
				}

				if resp.TotalEntries != 1000 {
					t.Errorf("Expected TotalEntries=1000, but got %d", resp.TotalEntries)
				}
			}
		})
	}
}

func TestHandler_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET method should return success",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST method should return method not allowed",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockBlockingService{}
			handler := NewHandler(mockService)

			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			handler.HealthCheck(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, but got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp HealthResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Errorf("Failed to decode response: %v", err)
					return
				}

				if resp.Status != "healthy" {
					t.Errorf("Expected status='healthy', but got %s", resp.Status)
				}
			}
		})
	}
}
