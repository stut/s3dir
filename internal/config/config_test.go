package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Clear environment
	os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check defaults
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", cfg.Host)
	}

	if cfg.Port != 8000 {
		t.Errorf("Expected default port 8000, got %d", cfg.Port)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("Expected default data dir './data', got '%s'", cfg.DataDir)
	}

	if cfg.EnableAuth != false {
		t.Error("Expected authentication disabled by default")
	}

	if cfg.ReadOnly != false {
		t.Error("Expected read-only disabled by default")
	}
}

func TestLoadWithEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("S3DIR_HOST", "127.0.0.1")
	os.Setenv("S3DIR_PORT", "9000")
	os.Setenv("S3DIR_DATA_DIR", "/tmp/test-data")
	os.Setenv("S3DIR_ACCESS_KEY_ID", "test-key")
	os.Setenv("S3DIR_SECRET_ACCESS_KEY", "test-secret")
	os.Setenv("S3DIR_ENABLE_AUTH", "true")
	os.Setenv("S3DIR_READ_ONLY", "true")
	os.Setenv("S3DIR_VERBOSE", "true")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", cfg.Host)
	}

	if cfg.Port != 9000 {
		t.Errorf("Expected port 9000, got %d", cfg.Port)
	}

	if cfg.DataDir != "/tmp/test-data" {
		t.Errorf("Expected data dir '/tmp/test-data', got '%s'", cfg.DataDir)
	}

	if cfg.AccessKeyID != "test-key" {
		t.Errorf("Expected access key 'test-key', got '%s'", cfg.AccessKeyID)
	}

	if cfg.SecretAccessKey != "test-secret" {
		t.Errorf("Expected secret key 'test-secret', got '%s'", cfg.SecretAccessKey)
	}

	if !cfg.EnableAuth {
		t.Error("Expected authentication enabled")
	}

	if !cfg.ReadOnly {
		t.Error("Expected read-only enabled")
	}

	if !cfg.Verbose {
		t.Error("Expected verbose enabled")
	}

	// Clean up test directory
	os.RemoveAll("/tmp/test-data")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Host:       "0.0.0.0",
				Port:       8000,
				DataDir:    "/tmp/test-s3dir-valid",
				EnableAuth: false,
			},
			wantError: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				Host:    "0.0.0.0",
				Port:    0,
				DataDir: "/tmp/test-s3dir",
			},
			wantError: true,
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Host:    "0.0.0.0",
				Port:    65536,
				DataDir: "/tmp/test-s3dir",
			},
			wantError: true,
		},
		{
			name: "empty data directory",
			config: &Config{
				Host:    "0.0.0.0",
				Port:    8000,
				DataDir: "",
			},
			wantError: true,
		},
		{
			name: "auth enabled without access key",
			config: &Config{
				Host:            "0.0.0.0",
				Port:            8000,
				DataDir:         "/tmp/test-s3dir",
				EnableAuth:      true,
				SecretAccessKey: "secret",
			},
			wantError: true,
		},
		{
			name: "auth enabled without secret key",
			config: &Config{
				Host:        "0.0.0.0",
				Port:        8000,
				DataDir:     "/tmp/test-s3dir",
				EnableAuth:  true,
				AccessKeyID: "key",
			},
			wantError: true,
		},
		{
			name: "auth enabled with credentials",
			config: &Config{
				Host:            "0.0.0.0",
				Port:            8000,
				DataDir:         "/tmp/test-s3dir-auth",
				EnableAuth:      true,
				AccessKeyID:     "key",
				SecretAccessKey: "secret",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}

			// Clean up test directory
			if tt.config.DataDir != "" {
				os.RemoveAll(tt.config.DataDir)
			}
		})
	}
}

func TestAddress(t *testing.T) {
	cfg := &Config{
		Host: "localhost",
		Port: 9000,
	}

	expected := "localhost:9000"
	if addr := cfg.Address(); addr != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, addr)
	}
}

func TestGetEnv(t *testing.T) {
	os.Clearenv()

	// Test with default value
	val := getEnv("TEST_VAR", "default")
	if val != "default" {
		t.Errorf("Expected 'default', got '%s'", val)
	}

	// Test with set value
	os.Setenv("TEST_VAR", "custom")
	val = getEnv("TEST_VAR", "default")
	if val != "custom" {
		t.Errorf("Expected 'custom', got '%s'", val)
	}

	os.Clearenv()
}

func TestGetEnvAsInt(t *testing.T) {
	os.Clearenv()

	// Test with default value
	val := getEnvAsInt("TEST_INT", 42)
	if val != 42 {
		t.Errorf("Expected 42, got %d", val)
	}

	// Test with valid integer
	os.Setenv("TEST_INT", "100")
	val = getEnvAsInt("TEST_INT", 42)
	if val != 100 {
		t.Errorf("Expected 100, got %d", val)
	}

	// Test with invalid integer
	os.Setenv("TEST_INT", "invalid")
	val = getEnvAsInt("TEST_INT", 42)
	if val != 42 {
		t.Errorf("Expected 42 (default), got %d", val)
	}

	os.Clearenv()
}

func TestGetEnvAsBool(t *testing.T) {
	os.Clearenv()

	// Test with default value
	val := getEnvAsBool("TEST_BOOL", false)
	if val != false {
		t.Errorf("Expected false, got %v", val)
	}

	// Test with valid boolean
	os.Setenv("TEST_BOOL", "true")
	val = getEnvAsBool("TEST_BOOL", false)
	if val != true {
		t.Errorf("Expected true, got %v", val)
	}

	// Test with invalid boolean
	os.Setenv("TEST_BOOL", "invalid")
	val = getEnvAsBool("TEST_BOOL", false)
	if val != false {
		t.Errorf("Expected false (default), got %v", val)
	}

	os.Clearenv()
}
