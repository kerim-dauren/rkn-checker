package updater

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

// createTestRegistry creates a registry with one entry for testing
func createTestRegistry() *domain.Registry {
	registry := domain.NewRegistry()
	entry, _ := domain.NewRegistryEntry(domain.BlockingTypeDomain, "test.com")
	registry.AddEntry(entry)
	return registry
}

// mockRegistryClient is a test implementation
type mockRegistryClient struct {
	registry  *domain.Registry
	err       error
	callCount int
	mu        sync.Mutex
}

func (m *mockRegistryClient) FetchRegistry(ctx context.Context) (*domain.Registry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.registry, nil
}

// mockRegistryStore is a test implementation
type mockRegistryStore struct {
	registry       *domain.Registry
	updateErr      error
	lastUpdateTime time.Time
	updateCount    int
	mu             sync.Mutex
}

func (m *mockRegistryStore) Update(registry *domain.Registry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCount++
	if m.updateErr != nil {
		return m.updateErr
	}
	m.registry = registry
	m.lastUpdateTime = time.Now()
	return nil
}

func (m *mockRegistryStore) GetLastUpdateTime() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastUpdateTime
}

func (m *mockRegistryStore) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.registry == nil {
		return 0
	}
	return m.registry.Size()
}

func TestNewScheduler(t *testing.T) {
	client := &mockRegistryClient{}
	store := &mockRegistryStore{}
	config := DefaultConfig()

	scheduler := NewScheduler(client, store, config)

	if scheduler == nil {
		t.Fatal("scheduler is nil")
	}

	if scheduler.interval != config.Interval {
		t.Errorf("expected interval %v, got %v", config.Interval, scheduler.interval)
	}

	if scheduler.maxRetries != config.MaxRetries {
		t.Errorf("expected maxRetries %d, got %d", config.MaxRetries, scheduler.maxRetries)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}
	config := Config{
		Interval:      100 * time.Millisecond,
		MaxRetries:    1,
		RetryDelay:    10 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	// Test starting scheduler
	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error starting scheduler: %v", err)
	}

	// Wait a bit to ensure initial update happens
	time.Sleep(50 * time.Millisecond)

	status := scheduler.GetStatus()
	if !status.Running {
		t.Error("scheduler should be running")
	}

	// Test stopping scheduler
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("unexpected error stopping scheduler: %v", err)
	}

	status = scheduler.GetStatus()
	if status.Running {
		t.Error("scheduler should not be running")
	}
}

func TestScheduler_StartAlreadyRunning(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}
	config := DefaultConfig()

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	// Try to start again
	err = scheduler.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running scheduler")
	}
}

func TestScheduler_StopNotRunning(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}
	config := DefaultConfig()

	scheduler := NewScheduler(client, store, config)

	err := scheduler.Stop()
	if err == nil {
		t.Error("expected error when stopping non-running scheduler")
	}
}

func TestScheduler_PerformUpdate_Success(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      1 * time.Hour,
		MaxRetries:    1,
		RetryDelay:    10 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	scheduler.performUpdate(ctx)

	status := scheduler.GetStatus()
	if status.LastError != nil {
		t.Errorf("expected no error, got %v", status.LastError)
	}

	if status.SuccessfulUpdates != 1 {
		t.Errorf("expected 1 successful update, got %d", status.SuccessfulUpdates)
	}

	if status.TotalUpdates != 1 {
		t.Errorf("expected 1 total update, got %d", status.TotalUpdates)
	}

	if store.updateCount != 1 {
		t.Errorf("expected 1 store update, got %d", store.updateCount)
	}
}

func TestScheduler_PerformUpdate_ClientError(t *testing.T) {
	client := &mockRegistryClient{
		err: errors.New("client error"),
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      1 * time.Hour,
		MaxRetries:    2,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	scheduler.performUpdate(ctx)

	status := scheduler.GetStatus()
	if status.LastError == nil {
		t.Error("expected error")
	}

	if status.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", status.ConsecutiveFailures)
	}

	if status.SuccessfulUpdates != 0 {
		t.Errorf("expected 0 successful updates, got %d", status.SuccessfulUpdates)
	}

	// Should retry
	if client.callCount != 2 {
		t.Errorf("expected 2 client calls (with retry), got %d", client.callCount)
	}
}

func TestScheduler_PerformUpdate_StoreError(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{
		updateErr: errors.New("store error"),
	}

	config := Config{
		Interval:      1 * time.Hour,
		MaxRetries:    1,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	scheduler.performUpdate(ctx)

	status := scheduler.GetStatus()
	if status.LastError == nil {
		t.Error("expected error")
	}

	if status.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", status.ConsecutiveFailures)
	}
}

func TestScheduler_PerformUpdate_EmptyRegistry(t *testing.T) {
	registry := domain.NewRegistry() // Empty registry

	client := &mockRegistryClient{
		registry: registry,
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      1 * time.Hour,
		MaxRetries:    1,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	scheduler.performUpdate(ctx)

	status := scheduler.GetStatus()
	if status.LastError == nil {
		t.Error("expected error for empty registry")
	}

	if status.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", status.ConsecutiveFailures)
	}
}

func TestScheduler_TriggerUpdate(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      1 * time.Hour, // Long interval
		MaxRetries:    1,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	// Wait for initial update
	time.Sleep(10 * time.Millisecond)

	initialCount := store.updateCount

	// Trigger manual update
	scheduler.TriggerUpdate()

	// Wait for triggered update
	time.Sleep(50 * time.Millisecond)

	if store.updateCount <= initialCount {
		t.Error("triggered update should have occurred")
	}
}

func TestScheduler_IsHealthy(t *testing.T) {
	client := &mockRegistryClient{}
	store := &mockRegistryStore{}
	config := DefaultConfig()

	scheduler := NewScheduler(client, store, config)

	// Initially healthy
	if !scheduler.IsHealthy() {
		t.Error("scheduler should be healthy initially")
	}

	// After too many failures
	scheduler.consecutiveFailures = 5
	if scheduler.IsHealthy() {
		t.Error("scheduler should not be healthy after many failures")
	}

	// Reset failures but set old last update
	scheduler.consecutiveFailures = 0
	scheduler.lastUpdate = time.Now().Add(-100 * time.Hour) // Very old
	if scheduler.IsHealthy() {
		t.Error("scheduler should not be healthy with very old last update")
	}
}

func TestScheduler_PeriodicUpdates(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      50 * time.Millisecond, // Short interval for testing
		MaxRetries:    1,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer scheduler.Stop()

	// Wait for multiple updates
	time.Sleep(150 * time.Millisecond)

	if store.updateCount < 2 {
		t.Errorf("expected at least 2 updates, got %d", store.updateCount)
	}
}

func TestScheduler_ContextCancellation(t *testing.T) {
	client := &mockRegistryClient{
		registry: createTestRegistry(),
	}
	store := &mockRegistryStore{}

	config := Config{
		Interval:      10 * time.Millisecond,
		MaxRetries:    1,
		RetryDelay:    1 * time.Millisecond,
		UpdateTimeout: 1 * time.Second,
	}

	scheduler := NewScheduler(client, store, config)

	ctx, cancel := context.WithCancel(context.Background())

	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cancel context after a short time
	time.Sleep(20 * time.Millisecond)
	cancel()

	// Wait a bit more to ensure scheduler stops
	time.Sleep(50 * time.Millisecond)

	status := scheduler.GetStatus()
	if status.Running {
		t.Error("scheduler should have stopped after context cancellation")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Interval != 48*time.Hour {
		t.Errorf("expected interval 48h, got %v", config.Interval)
	}

	if config.MaxRetries != 3 {
		t.Errorf("expected maxRetries 3, got %d", config.MaxRetries)
	}

	if config.RetryDelay != 5*time.Minute {
		t.Errorf("expected retryDelay 5m, got %v", config.RetryDelay)
	}

	if config.UpdateTimeout != 10*time.Minute {
		t.Errorf("expected updateTimeout 10m, got %v", config.UpdateTimeout)
	}
}

func TestStatus_SuccessRate(t *testing.T) {
	tests := []struct {
		name              string
		totalUpdates      int
		successfulUpdates int
		expectedRate      float64
	}{
		{"No updates", 0, 0, 0},
		{"All successful", 10, 10, 100},
		{"Half successful", 10, 5, 50},
		{"No successful", 10, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := Status{
				TotalUpdates:      tt.totalUpdates,
				SuccessfulUpdates: tt.successfulUpdates,
			}

			rate := status.SuccessRate()
			if rate != tt.expectedRate {
				t.Errorf("expected success rate %.2f, got %.2f", tt.expectedRate, rate)
			}
		})
	}
}
