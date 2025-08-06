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

	// RKN API specific configuration
	RKN RKNConfig `json:"rkn,omitempty"`
}

// RKNConfig holds RKN API specific configuration
type RKNConfig struct {
	// Authentication files (paths or base64 encoded content)
	RequestFilePath    string `json:"request_file_path,omitempty"`
	SignatureFilePath  string `json:"signature_file_path,omitempty"`
	EMCHDFilePath      string `json:"emchd_file_path,omitempty"`
	EMCHDSignaturePath string `json:"emchd_signature_path,omitempty"`

	// Request configuration
	DumpFormatVersion string        `json:"dump_format_version,omitempty"`
	PollInterval      time.Duration `json:"poll_interval,omitempty"`
	MaxPollAttempts   int           `json:"max_poll_attempts,omitempty"`

	// TLS configuration for client certificates
	CertFilePath       string `json:"cert_file_path,omitempty"`
	KeyFilePath        string `json:"key_file_path,omitempty"`
	CAFilePath         string `json:"ca_file_path,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
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
			RKN: RKNConfig{
				DumpFormatVersion:  "2.4",
				PollInterval:       30 * time.Second,
				MaxPollAttempts:    20, // Max 10 minutes of polling
				InsecureSkipVerify: false,
			},
		},
	}
}
