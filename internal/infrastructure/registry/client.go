package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/domain"
)

// Client manages multiple registry sources with fallback logic
type Client struct {
	sources []Source
	parser  *Parser

	// Configuration
	maxConcurrent int
	timeout       time.Duration

	// State tracking
	lastSuccessfulSource string
	lastUpdateTime       time.Time
	consecutiveFailures  int
}

// ClientConfig holds configuration for the registry client
type ClientConfig struct {
	Sources       []SourceConfig
	MaxConcurrent int
	Timeout       time.Duration
}

// NewClient creates a new registry client with configured sources
func NewClient(config ClientConfig) (*Client, error) {
	if len(config.Sources) == 0 {
		return nil, fmt.Errorf("at least one source must be configured")
	}

	client := &Client{
		sources:       make([]Source, 0, len(config.Sources)),
		parser:        NewParser(),
		maxConcurrent: config.MaxConcurrent,
		timeout:       config.Timeout,
	}

	// Initialize sources based on configuration
	for _, srcConfig := range config.Sources {
		source, err := client.createSource(srcConfig)
		if err != nil {
			return nil, fmt.Errorf("creating source %s: %w", srcConfig.Type, err)
		}
		client.sources = append(client.sources, source)
	}

	return client, nil
}

// createSource creates a source instance based on configuration
func (c *Client) createSource(config SourceConfig) (Source, error) {
	switch config.Type {
	case SourceTypeOfficial:
		return NewOfficialSource(config), nil
	default:
		return nil, fmt.Errorf("unsupported source type: %s", config.Type)
	}
}

// FetchRegistry attempts to fetch registry data from all configured sources
func (c *Client) FetchRegistry(ctx context.Context) (*domain.Registry, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Try sources in order, starting with the last successful one
	sources := c.orderSources()

	var lastErr error
	for _, source := range sources {
		registry, err := c.fetchFromSource(ctx, source)
		if err == nil {
			c.onFetchSuccess(source.Name())
			return registry, nil
		}

		lastErr = err
		c.onFetchFailure(source.Name(), err)

		// Check if context was cancelled
		if ctx.Err() != nil {
			break
		}
	}

	c.consecutiveFailures++
	return nil, fmt.Errorf("%w: last error: %v", ErrAllSourcesFailed, lastErr)
}

// fetchFromSource attempts to fetch and parse data from a single source
func (c *Client) fetchFromSource(ctx context.Context, source Source) (*domain.Registry, error) {
	// Check if source is healthy before attempting fetch
	if !source.IsHealthy(ctx) {
		return nil, NewSourceError(source.Name(), "health_check",
			fmt.Errorf("source is not healthy"))
	}

	// Fetch raw data
	data, err := source.Fetch(ctx)
	if err != nil {
		return nil, NewSourceError(source.Name(), "fetch", err)
	}

	// Parse data into registry
	registry, err := c.parser.Parse(data)
	if err != nil {
		return nil, NewSourceError(source.Name(), "parse", err)
	}

	// Set registry metadata
	registry.Source = source.Name()
	registry.LastUpdated = time.Now()

	return registry, nil
}

// orderSources returns sources ordered by preference
func (c *Client) orderSources() []Source {
	if c.lastSuccessfulSource == "" {
		return c.sources
	}

	// Move last successful source to front
	ordered := make([]Source, 0, len(c.sources))
	var lastSuccessful Source

	for _, source := range c.sources {
		if source.Name() == c.lastSuccessfulSource {
			lastSuccessful = source
		} else {
			ordered = append(ordered, source)
		}
	}

	if lastSuccessful != nil {
		ordered = append([]Source{lastSuccessful}, ordered...)
	}

	return ordered
}

// onFetchSuccess handles successful fetch operations
func (c *Client) onFetchSuccess(sourceName string) {
	c.lastSuccessfulSource = sourceName
	c.lastUpdateTime = time.Now()
	c.consecutiveFailures = 0
}

// onFetchFailure handles failed fetch operations
func (c *Client) onFetchFailure(sourceName string, err error) {
	// Log error (in production, use proper logging)
	// For now, we'll just track failures
}

// GetHealthStatus returns the current health status of all sources
func (c *Client) GetHealthStatus(ctx context.Context) map[string]bool {
	status := make(map[string]bool)

	for _, source := range c.sources {
		status[source.Name()] = source.IsHealthy(ctx)
	}

	return status
}

// GetLastUpdateTime returns the time of the last successful update
func (c *Client) GetLastUpdateTime() time.Time {
	return c.lastUpdateTime
}

// GetConsecutiveFailures returns the number of consecutive failures
func (c *Client) GetConsecutiveFailures() int {
	return c.consecutiveFailures
}

// GetLastSuccessfulSource returns the name of the last successful source
func (c *Client) GetLastSuccessfulSource() string {
	return c.lastSuccessfulSource
}
