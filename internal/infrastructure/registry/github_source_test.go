package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGitHubSource_Name(t *testing.T) {
	config := SourceConfig{Type: SourceTypeGitHub}
	source := NewGitHubSource(config)

	expected := "GitHub Mirror"
	if source.Name() != expected {
		t.Errorf("expected name %q, got %q", expected, source.Name())
	}
}

func TestGitHubSource_Fetch_Success(t *testing.T) {
	// Create test server
	testData := "id;url;date\n1;example.com;2023-01-01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Error("User-Agent header not set correctly")
		}
		if r.Header.Get("Accept") != "text/csv, application/zip, */*" {
			t.Error("Accept header not set correctly")
		}

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testData))
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	data, err := source.Fetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(data) != testData {
		t.Errorf("expected data %q, got %q", testData, string(data))
	}
}

func TestGitHubSource_Fetch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("expected error but got none")
	}

	var sourceErr *SourceError
	if !errors.As(err, &sourceErr) {
		t.Error("expected SourceError")
	}
}

func TestGitHubSource_Fetch_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return empty body
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("expected error for empty response")
	}

	if !errors.Is(err, ErrEmptyData) {
		t.Errorf("expected ErrEmptyData, got %v", err)
	}
}

func TestGitHubSource_Fetch_TooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "200000000") // 200MB
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("expected error for large response")
	}
}

func TestGitHubSource_Fetch_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	data, err := source.Fetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(data) != "success" {
		t.Errorf("expected 'success', got %q", string(data))
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestGitHubSource_Fetch_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data"))
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := source.Fetch(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}

	if ctx.Err() == nil {
		t.Error("context should be cancelled")
	}
}

func TestGitHubSource_IsHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:      SourceTypeGitHub,
		URL:       server.URL,
		UserAgent: "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	if !source.IsHealthy(ctx) {
		t.Error("source should be healthy")
	}
}

func TestGitHubSource_IsHealthy_Cached(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:      SourceTypeGitHub,
		URL:       server.URL,
		UserAgent: "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	// First call should hit the server
	source.IsHealthy(ctx)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}

	// Second call should use cache (within 5 minute window)
	source.IsHealthy(ctx)
	if callCount != 1 {
		t.Errorf("expected cached result, but got %d calls", callCount)
	}
}

func TestGitHubSource_IsHealthy_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:      SourceTypeGitHub,
		URL:       server.URL,
		UserAgent: "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	if source.IsHealthy(ctx) {
		t.Error("source should not be healthy")
	}
}

func BenchmarkGitHubSource_Fetch(b *testing.B) {
	testData := make([]byte, 1024) // 1KB test data
	for i := range testData {
		testData[i] = byte('a' + (i % 26))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer server.Close()

	config := SourceConfig{
		Type:       SourceTypeGitHub,
		URL:        server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		UserAgent:  "test-agent",
	}

	source := NewGitHubSource(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := source.Fetch(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
