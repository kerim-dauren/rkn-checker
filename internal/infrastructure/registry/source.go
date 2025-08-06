package registry

import (
	"context"
	"time"
)

// Source represents a registry data source (GitHub mirror, official API, etc.)
type Source interface {
	// Fetch retrieves raw registry data from the source
	Fetch(ctx context.Context) ([]byte, error)

	// Name returns a human-readable name for the source
	Name() string

	// IsHealthy checks if the source is currently available
	IsHealthy(ctx context.Context) bool
}

// SourceType represents different types of registry sources
type SourceType string

const (
	SourceTypeOfficial SourceType = "official"
)

// SourceConfig holds configuration for a registry source
type SourceConfig struct {
	Type       SourceType
	URL        string
	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
}

// DefaultSourceConfigs returns default configurations for known sources
func DefaultSourceConfigs() []SourceConfig {
	return []SourceConfig{
		{
			Type:       SourceTypeOfficial,
			URL:        "https://vigruzki.rkn.gov.ru/services/OperatorRequest/",
			Timeout:    60 * time.Second,
			MaxRetries: 2,
			UserAgent:  "RKN-Checker/1.0",
		},
	}
}
