package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/registry"
	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/updater"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Registry RegistryConfig `json:"registry"`
	Storage  StorageConfig  `json:"storage"`
	Logging  LoggingConfig  `json:"logging"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	GRPCPort int    `json:"grpc_port"`
	RESTPort int    `json:"rest_port"`
	Host     string `json:"host"`
	Env      string `json:"env"`
}

// RegistryConfig holds registry-related configuration
type RegistryConfig struct {
	Sources       []registry.SourceConfig `json:"sources"`
	UpdateConfig  updater.Config          `json:"update"`
	MaxConcurrent int                     `json:"max_concurrent"`
	Timeout       time.Duration           `json:"timeout"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	BloomFilterSize   int `json:"bloom_filter_size"`
	BloomFilterHashes int `json:"bloom_filter_hashes"`
	MaxRegistrySize   int `json:"max_registry_size"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			GRPCPort: getEnvInt("GRPC_PORT", 9090),
			RESTPort: getEnvInt("REST_PORT", 80),
			Host:     getEnvString("HOST", "0.0.0.0"),
			Env:      getEnvString("SERVER_ENV", "development"),
		},
		Registry: RegistryConfig{
			Sources:       getDefaultSources(),
			UpdateConfig:  getUpdateConfig(),
			MaxConcurrent: getEnvInt("REGISTRY_MAX_CONCURRENT", 5),
			Timeout:       getEnvDuration("REGISTRY_TIMEOUT", 30*time.Second),
		},
		Storage: StorageConfig{
			BloomFilterSize:   getEnvInt("BLOOM_FILTER_SIZE", 10000000),
			BloomFilterHashes: getEnvInt("BLOOM_FILTER_HASHES", 7),
			MaxRegistrySize:   getEnvInt("MAX_REGISTRY_SIZE", 5000000),
		},
		Logging: LoggingConfig{
			Level:  getEnvString("LOG_LEVEL", "info"),
			Format: getEnvString("LOG_FORMAT", "text"),
		},
	}

	// Override sources with custom URLs if provided
	if primaryURL := getEnvString("REGISTRY_PRIMARY_URL", ""); primaryURL != "" {
		config.Registry.Sources[0].URL = primaryURL
	}

	if fallbackURL := getEnvString("REGISTRY_FALLBACK_URL", ""); fallbackURL != "" && len(config.Registry.Sources) > 1 {
		config.Registry.Sources[1].URL = fallbackURL
	}

	return config, nil
}

// getDefaultSources returns default registry source configurations
func getDefaultSources() []registry.SourceConfig {
	return []registry.SourceConfig{
		{
			Type:       registry.SourceTypeGitHub,
			URL:        "https://raw.githubusercontent.com/zapret-info/z-i/master/dump.csv",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			UserAgent:  "RKN-Checker/1.0",
		},
		{
			Type:       registry.SourceTypeOfficial,
			URL:        "https://vigruzki.rkn.gov.ru/services/OperatorRequest/",
			Timeout:    60 * time.Second,
			MaxRetries: 2,
			UserAgent:  "RKN-Checker/1.0",
		},
	}
}

// getUpdateConfig returns update scheduler configuration
func getUpdateConfig() updater.Config {
	return updater.Config{
		Interval:      getEnvDuration("UPDATE_INTERVAL", 48*time.Hour),
		MaxRetries:    getEnvInt("UPDATE_MAX_RETRIES", 3),
		RetryDelay:    getEnvDuration("UPDATE_RETRY_DELAY", 5*time.Minute),
		UpdateTimeout: getEnvDuration("UPDATE_TIMEOUT", 10*time.Minute),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server configuration
	if c.Server.GRPCPort < 1 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid GRPC port: %d", c.Server.GRPCPort)
	}

	if c.Server.RESTPort < 1 || c.Server.RESTPort > 65535 {
		return fmt.Errorf("invalid REST port: %d", c.Server.RESTPort)
	}

	// Validate registry configuration
	if len(c.Registry.Sources) == 0 {
		return fmt.Errorf("at least one registry source must be configured")
	}

	for i, source := range c.Registry.Sources {
		if source.URL == "" {
			return fmt.Errorf("registry source %d has empty URL", i)
		}
		if source.Timeout <= 0 {
			return fmt.Errorf("registry source %d has invalid timeout", i)
		}
	}

	// Validate storage configuration
	if c.Storage.BloomFilterSize <= 0 {
		return fmt.Errorf("bloom filter size must be positive")
	}

	if c.Storage.BloomFilterHashes <= 0 {
		return fmt.Errorf("bloom filter hash count must be positive")
	}

	// Validate logging configuration
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"text": true, "json": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// Utility functions for reading environment variables

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
