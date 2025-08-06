package config

import (
	"os"
	"testing"
	"time"

	"github.com/kerim-dauren/rkn-checker/internal/infrastructure/registry"
)

func TestLoadConfig_DefaultValues(t *testing.T) {
	// Clear any existing environment variables
	clearEnv()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test default server config
	if config.Server.GRPCPort != 9090 {
		t.Errorf("expected GRPC port 9090, got %d", config.Server.GRPCPort)
	}

	if config.Server.RESTPort != 80 {
		t.Errorf("expected REST port 80, got %d", config.Server.RESTPort)
	}

	if config.Server.Host != "0.0.0.0" {
		t.Errorf("expected host '0.0.0.0', got %q", config.Server.Host)
	}

	if config.Server.Env != "development" {
		t.Errorf("expected env 'development', got %q", config.Server.Env)
	}

	// Test default registry config
	if len(config.Registry.Sources) != 1 {
		t.Errorf("expected 1 registry source, got %d", len(config.Registry.Sources))
	}

	if config.Registry.Sources[0].Type != registry.SourceTypeOfficial {
		t.Errorf("expected first source to be Official, got %v", config.Registry.Sources[0].Type)
	}

	if config.Registry.UpdateConfig.Interval != 48*time.Hour {
		t.Errorf("expected update interval 48h, got %v", config.Registry.UpdateConfig.Interval)
	}

	// Test default storage config
	if config.Storage.BloomFilterSize != 10000000 {
		t.Errorf("expected bloom filter size 10000000, got %d", config.Storage.BloomFilterSize)
	}

	// Test default logging config
	if config.Logging.Level != "info" {
		t.Errorf("expected log level 'info', got %q", config.Logging.Level)
	}

	if config.Logging.Format != "text" {
		t.Errorf("expected log format 'text', got %q", config.Logging.Format)
	}
}

func TestLoadConfig_EnvironmentOverrides(t *testing.T) {
	// Clear and set environment variables
	clearEnv()

	os.Setenv("GRPC_PORT", "8080")
	os.Setenv("REST_PORT", "3000")
	os.Setenv("HOST", "localhost")
	os.Setenv("SERVER_ENV", "production")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("UPDATE_INTERVAL", "24h")
	os.Setenv("BLOOM_FILTER_SIZE", "5000000")

	defer clearEnv()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Server.GRPCPort != 8080 {
		t.Errorf("expected GRPC port 8080, got %d", config.Server.GRPCPort)
	}

	if config.Server.RESTPort != 3000 {
		t.Errorf("expected REST port 3000, got %d", config.Server.RESTPort)
	}

	if config.Server.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %q", config.Server.Host)
	}

	if config.Server.Env != "production" {
		t.Errorf("expected env 'production', got %q", config.Server.Env)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", config.Logging.Level)
	}

	if config.Logging.Format != "json" {
		t.Errorf("expected log format 'json', got %q", config.Logging.Format)
	}

	if config.Registry.UpdateConfig.Interval != 24*time.Hour {
		t.Errorf("expected update interval 24h, got %v", config.Registry.UpdateConfig.Interval)
	}

	if config.Storage.BloomFilterSize != 5000000 {
		t.Errorf("expected bloom filter size 5000000, got %d", config.Storage.BloomFilterSize)
	}
}

func TestLoadConfig_CustomRegistryURLs(t *testing.T) {
	clearEnv()

	os.Setenv("REGISTRY_OFFICIAL_URL", "https://custom-official.com/api")

	defer clearEnv()

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Registry.Sources[0].URL != "https://custom-official.com/api" {
		t.Errorf("expected custom official URL, got %q", config.Registry.Sources[0].URL)
	}
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			GRPCPort: 9090,
			RESTPort: 80,
		},
		Registry: RegistryConfig{
			Sources: []registry.SourceConfig{
				{URL: "https://example.com", Timeout: 30 * time.Second},
			},
		},
		Storage: StorageConfig{
			BloomFilterSize:   1000000,
			BloomFilterHashes: 7,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestConfig_Validate_InvalidPorts(t *testing.T) {
	tests := []struct {
		name     string
		grpcPort int
		restPort int
	}{
		{"GRPC port too low", 0, 80},
		{"GRPC port too high", 65536, 80},
		{"REST port too low", 9090, 0},
		{"REST port too high", 9090, 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: tt.grpcPort,
					RESTPort: tt.restPort,
				},
				Registry: RegistryConfig{
					Sources: []registry.SourceConfig{
						{URL: "https://example.com", Timeout: 30 * time.Second},
					},
				},
				Storage: StorageConfig{
					BloomFilterSize:   1000000,
					BloomFilterHashes: 7,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := config.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_InvalidRegistry(t *testing.T) {
	tests := []struct {
		name    string
		sources []registry.SourceConfig
	}{
		{"No sources", []registry.SourceConfig{}},
		{"Empty URL", []registry.SourceConfig{{URL: "", Timeout: 30 * time.Second}}},
		{"Invalid timeout", []registry.SourceConfig{{URL: "https://example.com", Timeout: 0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 9090,
					RESTPort: 80,
				},
				Registry: RegistryConfig{
					Sources: tt.sources,
				},
				Storage: StorageConfig{
					BloomFilterSize:   1000000,
					BloomFilterHashes: 7,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := config.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_InvalidStorage(t *testing.T) {
	tests := []struct {
		name       string
		filterSize int
		hashCount  int
	}{
		{"Zero filter size", 0, 7},
		{"Negative filter size", -1, 7},
		{"Zero hash count", 1000000, 0},
		{"Negative hash count", 1000000, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 9090,
					RESTPort: 80,
				},
				Registry: RegistryConfig{
					Sources: []registry.SourceConfig{
						{URL: "https://example.com", Timeout: 30 * time.Second},
					},
				},
				Storage: StorageConfig{
					BloomFilterSize:   tt.filterSize,
					BloomFilterHashes: tt.hashCount,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := config.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_Validate_InvalidLogging(t *testing.T) {
	tests := []struct {
		name   string
		level  string
		format string
	}{
		{"Invalid level", "invalid", "json"},
		{"Invalid format", "info", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 9090,
					RESTPort: 80,
				},
				Registry: RegistryConfig{
					Sources: []registry.SourceConfig{
						{URL: "https://example.com", Timeout: 30 * time.Second},
					},
				},
				Storage: StorageConfig{
					BloomFilterSize:   1000000,
					BloomFilterHashes: 7,
				},
				Logging: LoggingConfig{
					Level:  tt.level,
					Format: tt.format,
				},
			}

			err := config.Validate()
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	config := &Config{
		Server: ServerConfig{Env: "development"},
	}

	if !config.IsDevelopment() {
		t.Error("should be development")
	}

	if config.IsProduction() {
		t.Error("should not be production")
	}
}

func TestConfig_IsProduction(t *testing.T) {
	config := &Config{
		Server: ServerConfig{Env: "production"},
	}

	if !config.IsProduction() {
		t.Error("should be production")
	}

	if config.IsDevelopment() {
		t.Error("should not be development")
	}
}

func TestUtilityFunctions(t *testing.T) {
	// Test getEnvString
	os.Setenv("TEST_STRING", "test_value")
	defer os.Unsetenv("TEST_STRING")

	if getEnvString("TEST_STRING", "default") != "test_value" {
		t.Error("getEnvString should return environment value")
	}

	if getEnvString("NON_EXISTENT", "default") != "default" {
		t.Error("getEnvString should return default for non-existent key")
	}

	// Test getEnvInt
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if getEnvInt("TEST_INT", 10) != 42 {
		t.Error("getEnvInt should return environment value")
	}

	if getEnvInt("NON_EXISTENT", 10) != 10 {
		t.Error("getEnvInt should return default for non-existent key")
	}

	// Test getEnvDuration
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")

	if getEnvDuration("TEST_DURATION", time.Hour) != 5*time.Minute {
		t.Error("getEnvDuration should return environment value")
	}

	if getEnvDuration("NON_EXISTENT", time.Hour) != time.Hour {
		t.Error("getEnvDuration should return default for non-existent key")
	}

	// Test getEnvBool
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")

	if !getEnvBool("TEST_BOOL", false) {
		t.Error("getEnvBool should return environment value")
	}

	if getEnvBool("NON_EXISTENT", false) {
		t.Error("getEnvBool should return default for non-existent key")
	}
}

// clearEnv removes all test-related environment variables
func clearEnv() {
	vars := []string{
		"GRPC_PORT", "REST_PORT", "HOST", "SERVER_ENV",
		"LOG_LEVEL", "LOG_FORMAT", "UPDATE_INTERVAL",
		"BLOOM_FILTER_SIZE", "BLOOM_FILTER_HASHES",
		"REGISTRY_OFFICIAL_URL",
		"TEST_STRING", "TEST_INT", "TEST_DURATION", "TEST_BOOL",
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}
}
