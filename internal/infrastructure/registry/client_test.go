package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

// mockSource is a test implementation of the Source interface
type mockSource struct {
	name            string
	data            []byte
	err             error
	healthy         bool
	fetchCallCount  int
	healthCallCount int
}

func (m *mockSource) Fetch(ctx context.Context) ([]byte, error) {
	m.fetchCallCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func (m *mockSource) Name() string {
	return m.name
}

func (m *mockSource) IsHealthy(ctx context.Context) bool {
	m.healthCallCount++
	return m.healthy
}

func TestNewClient(t *testing.T) {
	config := ClientConfig{
		Sources: []SourceConfig{
			{Type: SourceTypeGitHub, URL: "https://example.com"},
		},
		MaxConcurrent: 5,
		Timeout:       30 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("client is nil")
	}

	if len(client.sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(client.sources))
	}
}

func TestNewClient_NoSources(t *testing.T) {
	config := ClientConfig{
		Sources:       []SourceConfig{},
		MaxConcurrent: 5,
		Timeout:       30 * time.Second,
	}

	_, err := NewClient(config)
	if err == nil {
		t.Error("expected error for empty sources")
	}
}

func TestClient_FetchRegistry_Success(t *testing.T) {
	testData := []byte("id;url;date\n1;example.com;2023-01-01")

	mockSrc := &mockSource{
		name:    "test-source",
		data:    testData,
		healthy: true,
	}

	client := &Client{
		sources: []Source{mockSrc},
		parser:  NewParser(),
		timeout: 30 * time.Second,
	}

	ctx := context.Background()
	registry, err := client.FetchRegistry(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if registry == nil {
		t.Fatal("registry is nil")
	}

	// Explicit type check to ensure domain import is used
	var _ *domain.Registry = registry

	if registry.Source != "test-source" {
		t.Errorf("expected source 'test-source', got %q", registry.Source)
	}

	if mockSrc.fetchCallCount != 1 {
		t.Errorf("expected 1 fetch call, got %d", mockSrc.fetchCallCount)
	}

	if mockSrc.healthCallCount != 1 {
		t.Errorf("expected 1 health call, got %d", mockSrc.healthCallCount)
	}
}

func TestClient_FetchRegistry_AllSourcesFail(t *testing.T) {
	mockSrc1 := &mockSource{
		name:    "source-1",
		err:     errors.New("source 1 error"),
		healthy: true,
	}

	mockSrc2 := &mockSource{
		name:    "source-2",
		err:     errors.New("source 2 error"),
		healthy: true,
	}

	client := &Client{
		sources: []Source{mockSrc1, mockSrc2},
		parser:  NewParser(),
		timeout: 30 * time.Second,
	}

	ctx := context.Background()
	_, err := client.FetchRegistry(ctx)

	if err == nil {
		t.Error("expected error when all sources fail")
	}

	if !errors.Is(err, ErrAllSourcesFailed) {
		t.Errorf("expected ErrAllSourcesFailed, got %v", err)
	}

	if client.consecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", client.consecutiveFailures)
	}
}

func TestClient_FetchRegistry_UnhealthySource(t *testing.T) {
	mockSrc := &mockSource{
		name:    "unhealthy-source",
		data:    []byte("test data"),
		healthy: false,
	}

	client := &Client{
		sources: []Source{mockSrc},
		parser:  NewParser(),
		timeout: 30 * time.Second,
	}

	ctx := context.Background()
	_, err := client.FetchRegistry(ctx)

	if err == nil {
		t.Error("expected error for unhealthy source")
	}

	// Should check health but not fetch
	if mockSrc.healthCallCount != 1 {
		t.Errorf("expected 1 health call, got %d", mockSrc.healthCallCount)
	}

	if mockSrc.fetchCallCount != 0 {
		t.Errorf("expected 0 fetch calls, got %d", mockSrc.fetchCallCount)
	}
}

func TestClient_FetchRegistry_FallbackToSecondSource(t *testing.T) {
	testData := []byte("id;url;date\n1;example.com;2023-01-01")

	mockSrc1 := &mockSource{
		name:    "source-1",
		err:     errors.New("first source error"),
		healthy: true,
	}

	mockSrc2 := &mockSource{
		name:    "source-2",
		data:    testData,
		healthy: true,
	}

	client := &Client{
		sources: []Source{mockSrc1, mockSrc2},
		parser:  NewParser(),
		timeout: 30 * time.Second,
	}

	ctx := context.Background()
	registry, err := client.FetchRegistry(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if registry.Source != "source-2" {
		t.Errorf("expected source 'source-2', got %q", registry.Source)
	}

	// Both sources should be tried
	if mockSrc1.fetchCallCount != 1 {
		t.Errorf("expected 1 fetch call on source-1, got %d", mockSrc1.fetchCallCount)
	}

	if mockSrc2.fetchCallCount != 1 {
		t.Errorf("expected 1 fetch call on source-2, got %d", mockSrc2.fetchCallCount)
	}

	// Last successful source should be updated
	if client.lastSuccessfulSource != "source-2" {
		t.Errorf("expected last successful source 'source-2', got %q", client.lastSuccessfulSource)
	}
}

func TestClient_orderSources(t *testing.T) {
	mockSrc1 := &mockSource{name: "source-1"}
	mockSrc2 := &mockSource{name: "source-2"}
	mockSrc3 := &mockSource{name: "source-3"}

	client := &Client{
		sources: []Source{mockSrc1, mockSrc2, mockSrc3},
	}

	// Initially, should return sources in original order
	ordered := client.orderSources()
	if len(ordered) != 3 {
		t.Errorf("expected 3 sources, got %d", len(ordered))
	}
	if ordered[0].Name() != "source-1" {
		t.Errorf("expected first source 'source-1', got %q", ordered[0].Name())
	}

	// Set last successful source
	client.lastSuccessfulSource = "source-3"
	ordered = client.orderSources()

	if ordered[0].Name() != "source-3" {
		t.Errorf("expected first source 'source-3', got %q", ordered[0].Name())
	}
}

func TestClient_GetHealthStatus(t *testing.T) {
	mockSrc1 := &mockSource{name: "source-1", healthy: true}
	mockSrc2 := &mockSource{name: "source-2", healthy: false}

	client := &Client{
		sources: []Source{mockSrc1, mockSrc2},
	}

	ctx := context.Background()
	status := client.GetHealthStatus(ctx)

	if len(status) != 2 {
		t.Errorf("expected 2 status entries, got %d", len(status))
	}

	if !status["source-1"] {
		t.Error("source-1 should be healthy")
	}

	if status["source-2"] {
		t.Error("source-2 should not be healthy")
	}
}

func TestClient_FetchRegistry_ContextTimeout(t *testing.T) {
	mockSrc := &mockSource{
		name:    "slow-source",
		healthy: true,
	}

	client := &Client{
		sources: []Source{mockSrc},
		parser:  NewParser(),
		timeout: 10 * time.Millisecond, // Very short timeout
	}

	ctx := context.Background()
	_, err := client.FetchRegistry(ctx)

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestClient_GetMetrics(t *testing.T) {
	client := &Client{}

	// Initially empty
	if client.GetLastUpdateTime() != (time.Time{}) {
		t.Error("expected zero time for last update")
	}

	if client.GetConsecutiveFailures() != 0 {
		t.Error("expected 0 consecutive failures")
	}

	if client.GetLastSuccessfulSource() != "" {
		t.Error("expected empty last successful source")
	}

	// After setting values
	client.lastSuccessfulSource = "test-source"
	client.consecutiveFailures = 5
	client.lastUpdateTime = time.Now()

	if client.GetLastSuccessfulSource() != "test-source" {
		t.Error("last successful source not set correctly")
	}

	if client.GetConsecutiveFailures() != 5 {
		t.Error("consecutive failures not set correctly")
	}

	if client.GetLastUpdateTime().IsZero() {
		t.Error("last update time should not be zero")
	}
}

func BenchmarkClient_FetchRegistry(b *testing.B) {
	testData := []byte("id;url;date\n1;example.com;2023-01-01")

	mockSrc := &mockSource{
		name:    "bench-source",
		data:    testData,
		healthy: true,
	}

	client := &Client{
		sources: []Source{mockSrc},
		parser:  NewParser(),
		timeout: 30 * time.Second,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.FetchRegistry(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
