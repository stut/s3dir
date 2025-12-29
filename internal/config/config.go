package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Host string
	Port int

	// Storage configuration
	DataDir string

	// Authentication configuration
	AccessKeyID     string
	SecretAccessKey string
	EnableAuth      bool

	// Server options
	ReadOnly bool
	Verbose  bool
}

// Load loads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := &Config{
		Host:            getEnv("S3DIR_HOST", "0.0.0.0"),
		Port:            getEnvAsInt("S3DIR_PORT", 8000),
		DataDir:         getEnv("S3DIR_DATA_DIR", "./data"),
		AccessKeyID:     getEnv("S3DIR_ACCESS_KEY_ID", ""),
		SecretAccessKey: getEnv("S3DIR_SECRET_ACCESS_KEY", ""),
		EnableAuth:      getEnvAsBool("S3DIR_ENABLE_AUTH", false),
		ReadOnly:        getEnvAsBool("S3DIR_READ_ONLY", false),
		Verbose:         getEnvAsBool("S3DIR_VERBOSE", false),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}

	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	if c.EnableAuth {
		if c.AccessKeyID == "" {
			return fmt.Errorf("access key ID is required when authentication is enabled")
		}
		if c.SecretAccessKey == "" {
			return fmt.Errorf("secret access key is required when authentication is enabled")
		}
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	return nil
}

// Address returns the server address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt reads an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool reads an environment variable as a boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
