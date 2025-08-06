package updater

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

// RegistryClient represents the interface for fetching registry data
type RegistryClient interface {
	FetchRegistry(ctx context.Context) (*domain.Registry, error)
}

// RegistryStore represents the interface for storing registry data
type RegistryStore interface {
	Update(registry *domain.Registry) error
	GetLastUpdateTime() time.Time
	Size() int
}

// Scheduler manages automatic registry updates
type Scheduler struct {
	// Dependencies
	client RegistryClient
	store  RegistryStore

	// Configuration
	interval      time.Duration
	maxRetries    int
	retryDelay    time.Duration
	updateTimeout time.Duration

	// State
	mu                  sync.RWMutex
	running             bool
	lastUpdate          time.Time
	lastError           error
	consecutiveFailures int
	totalUpdates        int
	successfulUpdates   int

	// Control channels
	stopCh    chan struct{}
	triggerCh chan struct{}
	doneCh    chan struct{}
}

// Config holds configuration for the update scheduler
type Config struct {
	Interval      time.Duration // How often to update (e.g., 48 hours)
	MaxRetries    int           // Maximum retry attempts per update
	RetryDelay    time.Duration // Delay between retries
	UpdateTimeout time.Duration // Timeout for each update operation
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() Config {
	return Config{
		Interval:      48 * time.Hour, // RKN registry updates every 48 hours
		MaxRetries:    3,
		RetryDelay:    5 * time.Minute,
		UpdateTimeout: 10 * time.Minute,
	}
}

// NewScheduler creates a new update scheduler
func NewScheduler(client RegistryClient, store RegistryStore, config Config) *Scheduler {
	return &Scheduler{
		client:        client,
		store:         store,
		interval:      config.Interval,
		maxRetries:    config.MaxRetries,
		retryDelay:    config.RetryDelay,
		updateTimeout: config.UpdateTimeout,
		stopCh:        make(chan struct{}),
		triggerCh:     make(chan struct{}, 1),
		doneCh:        make(chan struct{}),
	}
}

// Start begins the update scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}
	s.running = true
	s.mu.Unlock()

	go s.run(ctx)

	return nil
}

// Stop stops the update scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is not running")
	}

	close(s.stopCh)
	s.mu.Unlock()

	// Wait for scheduler to finish
	<-s.doneCh

	return nil
}

// TriggerUpdate triggers an immediate update
func (s *Scheduler) TriggerUpdate() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
		// Channel is full, update already pending
	}
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	defer close(s.doneCh)
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Perform initial update
	s.performUpdate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.performUpdate(ctx)
		case <-s.triggerCh:
			s.performUpdate(ctx)
		}
	}
}

// performUpdate executes a registry update with retry logic
func (s *Scheduler) performUpdate(ctx context.Context) {
	s.mu.Lock()
	s.totalUpdates++
	s.mu.Unlock()

	updateCtx, cancel := context.WithTimeout(ctx, s.updateTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := s.retryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-updateCtx.Done():
				s.recordFailure(updateCtx.Err())
				return
			case <-time.After(delay):
			}
		}

		err := s.executeUpdate(updateCtx)
		if err == nil {
			s.recordSuccess()
			return
		}

		lastErr = err
	}

	s.recordFailure(fmt.Errorf("all retry attempts failed, last error: %w", lastErr))
}

// executeUpdate performs a single update attempt
func (s *Scheduler) executeUpdate(ctx context.Context) error {
	// Fetch new registry data
	registry, err := s.client.FetchRegistry(ctx)
	if err != nil {
		return fmt.Errorf("fetching registry: %w", err)
	}

	// Validate registry
	if registry.Size() == 0 {
		return fmt.Errorf("received empty registry")
	}

	// Update store atomically
	if err := s.store.Update(registry); err != nil {
		return fmt.Errorf("updating store: %w", err)
	}

	return nil
}

// recordSuccess records a successful update
func (s *Scheduler) recordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUpdate = time.Now()
	s.lastError = nil
	s.consecutiveFailures = 0
	s.successfulUpdates++
}

// recordFailure records a failed update
func (s *Scheduler) recordFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastError = err
	s.consecutiveFailures++
}

// GetStatus returns the current scheduler status
func (s *Scheduler) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		Running:             s.running,
		LastUpdate:          s.lastUpdate,
		LastError:           s.lastError,
		ConsecutiveFailures: s.consecutiveFailures,
		TotalUpdates:        s.totalUpdates,
		SuccessfulUpdates:   s.successfulUpdates,
		NextUpdate:          s.getNextUpdateTime(),
		RegistrySize:        s.store.Size(),
	}
}

// getNextUpdateTime calculates when the next update should occur
func (s *Scheduler) getNextUpdateTime() time.Time {
	if s.lastUpdate.IsZero() {
		return time.Now()
	}
	return s.lastUpdate.Add(s.interval)
}

// IsHealthy returns true if the scheduler is operating normally
func (s *Scheduler) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Consider unhealthy if too many consecutive failures
	if s.consecutiveFailures >= 5 {
		return false
	}

	// Consider unhealthy if no successful update in too long
	if !s.lastUpdate.IsZero() && time.Since(s.lastUpdate) > s.interval*2 {
		return false
	}

	return true
}

// Status represents the current state of the scheduler
type Status struct {
	Running             bool
	LastUpdate          time.Time
	LastError           error
	ConsecutiveFailures int
	TotalUpdates        int
	SuccessfulUpdates   int
	NextUpdate          time.Time
	RegistrySize        int
}

// SuccessRate returns the success rate as a percentage
func (s Status) SuccessRate() float64 {
	if s.TotalUpdates == 0 {
		return 0
	}
	return float64(s.SuccessfulUpdates) / float64(s.TotalUpdates) * 100
}
