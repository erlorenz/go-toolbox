package config_test

// import (
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"testing"
// 	"time"

// 	"github.com/erlorenz/go-toolbox/config"
// )

// // TestConfig is a test configuration struct with various field types and naming patterns
// type TestConfig struct {
// 	// Simple fields with different types
// 	ServerHost       string        `default:"localhost" desc:"Server hostname"`
// 	ServerPort       int           `default:"8080" desc:"Server port"`
// 	EnableDebug      bool          `default:"false" desc:"Enable debug mode"`
// 	RequestTimeout   time.Duration `default:"30s" desc:"Request timeout"`
// 	MaxRetryCount    int64         `default:"5" desc:"Maximum retry count"`
// 	RetryInterval    float64       `default:"1.5" desc:"Retry interval multiplier"`
// 	AllowedProtocols []string      `default:"http,https" desc:"Allowed protocols"`

// 	// Fields with explicit naming overrides
// 	APIKey      string `env:"API_KEY" flag:"api-key" optional:"true" desc:"API key for external service"`
// 	SecretToken string `env:"SECRET_TOKEN" flag:"secret" optional:"true" desc:"Secret token"`

// 	// Nested struct for database settings
// 	DB struct {
// 		Host              string        `default:"127.0.0.1" desc:"Database host"`
// 		Port              int           `default:"5432" desc:"Database port"`
// 		Name              string        `default:"myapp" desc:"Database name"`
// 		User              string        `default:"postgres" desc:"Database username"`
// 		Password          string        `optional:"true" desc:"Database password"`
// 		MaxConnections    int           `default:"20" desc:"Maximum database connections"`
// 		ConnectionTimeout time.Duration `default:"5s" desc:"Database connection timeout"`

// 		// Fields with multiple words to test case conversion
// 		EnableSSLMode       bool `default:"true" desc:"Enable SSL mode for database connection"`
// 		ReconnectAttemptMax int  `default:"3" desc:"Maximum reconnection attempts"`
// 	}

// 	// Nested struct for logging settings
// 	Logging struct {
// 		Level       string `default:"info" validate:"oneof=debug|info|warn|error" desc:"Logging level"`
// 		FilePath    string `default:"/var/log/myapp.log" desc:"Log file path"`
// 		MaxFileSize int    `default:"100" desc:"Maximum log file size in MB"`
// 		MaxBackups  int    `default:"5" desc:"Maximum number of log backups"`

// 		// Double nested struct to test deep nesting
// 		Rotation struct {
// 			Enabled  bool          `default:"true" desc:"Enable log rotation"`
// 			Interval time.Duration `default:"24h" desc:"Log rotation interval"`
// 		}
// 	}
// }

// // createTempYamlFile creates a temporary YAML file with test configuration
// func createTempYamlFile(t *testing.T) string {
// 	content := `
// serverHost: yaml-host.example.com
// serverPort: 9090
// enableDebug: true
// requestTimeout: 60s
// db:
//   host: db.example.com
//   port: 5433
//   maxConnections: 50
//   enableSSLMode: false
//   reconnectAttemptMax: 5
// logging:
//   level: debug
//   rotation:
//     interval: 12h
// `
// 	tmpDir := t.TempDir()
// 	tmpFile := filepath.Join(tmpDir, "config.yaml")

// 	err := os.WriteFile(tmpFile, []byte(content), 0644)
// 	if err != nil {
// 		t.Fatalf("Failed to create temp YAML file: %v", err)
// 	}

// 	return tmpFile
// }

// // TestDefaultValues tests that default values are applied correctly
// func TestDefaultValues(t *testing.T) {
// 	cfg := TestConfig{}

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		SkipEnv:   true,
// 		SkipFlags: true,
// 		SkipYaml:  true,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with default values: %v", err)
// 	}

// 	// Verify default values were applied correctly
// 	if cfg.ServerHost != "localhost" {
// 		t.Errorf("Expected ServerHost to be 'localhost', got '%s'", cfg.ServerHost)
// 	}
// 	if cfg.ServerPort != 8080 {
// 		t.Errorf("Expected ServerPort to be 8080, got %d", cfg.ServerPort)
// 	}
// 	if cfg.EnableDebug != false {
// 		t.Errorf("Expected EnableDebug to be false, got %v", cfg.EnableDebug)
// 	}
// 	if cfg.RequestTimeout != 30*time.Second {
// 		t.Errorf("Expected RequestTimeout to be 30s, got %v", cfg.RequestTimeout)
// 	}
// 	if cfg.Logging.Level != "info" {
// 		t.Errorf("Expected Logging.Level to be 'info', got '%s'", cfg.Logging.Level)
// 	}
// 	if !cfg.DB.EnableSSLMode {
// 		t.Errorf("Expected Database.EnableSSLMode to be true, got false")
// 	}
// 	if cfg.DB.ReconnectAttemptMax != 3 {
// 		t.Errorf("Expected Database.ReconnectAttemptMax to be 3, got %d", cfg.DB.ReconnectAttemptMax)
// 	}
// }

// // TestYamlParsing tests that YAML values are applied correctly
// func TestYamlParsing(t *testing.T) {
// 	cfg := TestConfig{}

// 	yamlFile := createTempYamlFile(t)

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		YamlFiles: []string{yamlFile},
// 		SkipEnv:   true,
// 		SkipFlags: true,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with YAML values: %v", err)
// 	}

// 	// Verify YAML values were applied correctly
// 	if cfg.ServerHost != "yaml-host.example.com" {
// 		t.Errorf("Expected ServerHost to be 'yaml-host.example.com', got '%s'", cfg.ServerHost)
// 	}
// 	if cfg.ServerPort != 9090 {
// 		t.Errorf("Expected ServerPort to be 9090, got %d", cfg.ServerPort)
// 	}
// 	if !cfg.EnableDebug {
// 		t.Errorf("Expected EnableDebug to be true, got false")
// 	}
// 	if cfg.RequestTimeout != 60*time.Second {
// 		t.Errorf("Expected RequestTimeout to be 60s, got %v", cfg.RequestTimeout)
// 	}
// 	if cfg.DB.Host != "db.example.com" {
// 		t.Errorf("Expected DB.Host to be 'db.example.com', got '%s'", cfg.DB.Host)
// 	}
// 	if cfg.DB.Port != 5433 {
// 		t.Errorf("Expected DB.Port to be 5433, got %d", cfg.DB.Port)
// 	}
// 	if cfg.DB.MaxConnections != 50 {
// 		t.Errorf("Expected DB.MaxConnections to be 50, got %d", cfg.DB.MaxConnections)
// 	}
// 	if cfg.DB.EnableSSLMode {
// 		t.Errorf("Expected DB.EnableSSLMode to be false, got true")
// 	}
// 	if cfg.DB.ReconnectAttemptMax != 5 {
// 		t.Errorf("Expected Database.ReconnectAttemptMax to be 5, got %d", cfg.DB.ReconnectAttemptMax)
// 	}
// 	if cfg.Logging.Level != "debug" {
// 		t.Errorf("Expected Logging.Level to be 'debug', got '%s'", cfg.Logging.Level)
// 	}
// 	if cfg.Logging.Rotation.Interval != 12*time.Hour {
// 		t.Errorf("Expected Logging.Rotation.Interval to be 12h, got %v", cfg.Logging.Rotation.Interval)
// 	}
// }

// // TestEnvironmentVariables tests that environment variables are applied correctly
// func TestEnvironmentVariables(t *testing.T) {
// 	// Save original environment variables to restore later
// 	originalEnv := os.Environ()
// 	t.Cleanup(func() {
// 		os.Clearenv()
// 		for _, env := range originalEnv {
// 			env := env
// 			key, value, _ := strings.Cut(env, "=")
// 			os.Setenv(key, value)
// 		}
// 	})

// 	// Set up test environment
// 	os.Clearenv()
// 	os.Setenv("SERVER_HOST", "env-host.example.com")
// 	os.Setenv("SERVER_PORT", "7070")
// 	os.Setenv("REQUEST_TIMEOUT", "45s")
// 	os.Setenv("API_KEY", "env-api-key") // Explicit override
// 	os.Setenv("DATABASE_HOST", "env-db.example.com")
// 	os.Setenv("DATABASE_PORT", "1234")
// 	os.Setenv("DATABASE_ENABLE_SSL_MODE", "true")
// 	os.Setenv("DATABASE_RECONNECT_ATTEMPT_MAX", "7")
// 	os.Setenv("LOGGING_LEVEL", "warn")
// 	os.Setenv("LOGGING_ROTATION_ENABLED", "false")

// 	cfg := TestConfig{}

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with environment variables: %v", err)
// 	}

// 	// Verify environment variables were applied correctly
// 	if cfg.ServerHost != "env-host.example.com" {
// 		t.Errorf("Expected ServerHost to be 'env-host.example.com', got '%s'", cfg.ServerHost)
// 	}
// 	if cfg.ServerPort != 7070 {
// 		t.Errorf("Expected ServerPort to be 7070, got %d", cfg.ServerPort)
// 	}
// 	if cfg.RequestTimeout != 45*time.Second {
// 		t.Errorf("Expected RequestTimeout to be 45s, got %v", cfg.RequestTimeout)
// 	}
// 	if cfg.APIKey != "env-api-key" {
// 		t.Errorf("Expected APIKey to be 'env-api-key', got '%s'", cfg.APIKey)
// 	}
// 	if cfg.DB.Host != "env-db.example.com" {
// 		t.Errorf("Expected DB.Host to be 'env-db.example.com', got '%s'", cfg.DB.Host)
// 	}
// 	if cfg.DB.Port != 1234 {
// 		t.Errorf("Expected DB.Port to be 1234, got %d", cfg.DB.Port)
// 	}
// 	if !cfg.DB.EnableSSLMode {
// 		t.Errorf("Expected DB.EnableSSLMode to be true, got false")
// 	}
// 	if cfg.DB.ReconnectAttemptMax != 7 {
// 		t.Errorf("Expected DB.ReconnectAttemptMax to be 7, got %d", cfg.DB.ReconnectAttemptMax)
// 	}
// 	if cfg.Logging.Level != "warn" {
// 		t.Errorf("Expected Logging.Level to be 'warn', got '%s'", cfg.Logging.Level)
// 	}
// 	if cfg.Logging.Rotation.Enabled {
// 		t.Errorf("Expected Logging.Rotation.Enabled to be false, got true")
// 	}
// }

// // TestEnvironmentVariablesWithPrefix tests that environment variables with prefix are applied correctly
// func TestEnvironmentVariablesWithPrefix(t *testing.T) {
// 	// Save original environment variables to restore later
// 	originalEnv := os.Environ()
// 	defer func() {
// 		os.Clearenv()
// 		for _, env := range originalEnv {
// 			env := env
// 			key, value, _ := strings.Cut(env, "=")
// 			os.Setenv(key, value)
// 		}
// 	}()

// 	// Set up test environment with prefix
// 	os.Clearenv()
// 	os.Setenv("MYAPP_SERVER_HOST", "prefixed-host.example.com")
// 	os.Setenv("MYAPP_SERVER_PORT", "6060")
// 	os.Setenv("MYAPP_API_KEY", "prefixed-api-key") // Should not work since explicit env name
// 	os.Setenv("API_KEY", "explicit-api-key")       // Explicit override should work
// 	os.Setenv("MYAPP_DATABASE_HOST", "prefixed-db.example.com")
// 	os.Setenv("MYAPP_DATABASE_ENABLE_SSL_MODE", "false")
// 	os.Setenv("MYAPP_DATABASE_RECONNECT_ATTEMPT_MAX", "8")
// 	os.Setenv("MYAPP_LOGGING_LEVEL", "error")

// 	cfg := TestConfig{}

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		EnvPrefix: "MYAPP",
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with prefixed environment variables: %v", err)
// 	}

// 	// Verify environment variables with prefix were applied correctly
// 	if cfg.ServerHost != "prefixed-host.example.com" {
// 		t.Errorf("Expected ServerHost to be 'prefixed-host.example.com', got '%s'", cfg.ServerHost)
// 	}
// 	if cfg.ServerPort != 6060 {
// 		t.Errorf("Expected ServerPort to be 6060, got %d", cfg.ServerPort)
// 	}
// 	if cfg.APIKey != "explicit-api-key" {
// 		t.Errorf("Expected APIKey to be 'explicit-api-key', got '%s'", cfg.APIKey)
// 	}
// 	if cfg.DB.Host != "prefixed-db.example.com" {
// 		t.Errorf("Expected Database.Host to be 'prefixed-db.example.com', got '%s'", cfg.DB.Host)
// 	}
// 	if cfg.DB.EnableSSLMode {
// 		t.Errorf("Expected Database.EnableSSLMode to be false, got true")
// 	}
// 	if cfg.DB.ReconnectAttemptMax != 8 {
// 		t.Errorf("Expected Database.ReconnectAttemptMax to be 8, got %d", cfg.DB.ReconnectAttemptMax)
// 	}
// 	if cfg.Logging.Level != "error" {
// 		t.Errorf("Expected Logging.Level to be 'error', got '%s'", cfg.Logging.Level)
// 	}
// }

// // TestCommandLineFlags tests that command line flags are applied correctly
// func TestCommandLineFlags(t *testing.T) {
// 	// Create a mock command line for testing
// 	testArgs := []string{
// 		"--server-host", "flag-host.example.com",
// 		"--server-port", "5050",
// 		"--request-timeout", "90s",
// 		"--api-key", "flag-api-key", // Explicit override
// 		"--database-host", "flag-db.example.com",
// 		"--database-port", "3456",
// 		"--database-enable-ssl-mode", "false",
// 		"--database-reconnect-attempt-max", "9",
// 		"--logging-level", "error",
// 	}

// 	cfg := TestConfig{}

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		SkipYaml: true,
// 		SkipEnv:  true,
// 		Args:     testArgs,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with command line flags: %v", err)
// 	}

// 	// Verify command line flags were applied correctly
// 	if cfg.ServerHost != "flag-host.example.com" {
// 		t.Errorf("Expected ServerHost to be 'flag-host.example.com', got '%s'", cfg.ServerHost)
// 	}
// 	if cfg.ServerPort != 5050 {
// 		t.Errorf("Expected ServerPort to be 5050, got %d", cfg.ServerPort)
// 	}
// 	if cfg.RequestTimeout != 90*time.Second {
// 		t.Errorf("Expected RequestTimeout to be 90s, got %v", cfg.RequestTimeout)
// 	}
// 	if cfg.APIKey != "flag-api-key" {
// 		t.Errorf("Expected APIKey to be 'flag-api-key', got '%s'", cfg.APIKey)
// 	}
// 	if cfg.DB.Host != "flag-db.example.com" {
// 		t.Errorf("Expected Database.Host to be 'flag-db.example.com', got '%s'", cfg.DB.Host)
// 	}
// 	if cfg.DB.Port != 3456 {
// 		t.Errorf("Expected Database.Port to be 3456, got %d", cfg.DB.Port)
// 	}
// 	if cfg.DB.EnableSSLMode {
// 		t.Errorf("Expected Database.EnableSSLMode to be false, got true")
// 	}
// 	if cfg.DB.ReconnectAttemptMax != 9 {
// 		t.Errorf("Expected Database.ReconnectAttemptMax to be 9, got %d", cfg.DB.ReconnectAttemptMax)
// 	}
// 	if cfg.Logging.Level != "error" {
// 		t.Errorf("Expected Logging.Level to be 'error', got '%s'", cfg.Logging.Level)
// 	}
// }

// // TestPrecedenceOrder tests the precedence order of configuration sources:
// // command line args > environment vars > yaml files > defaults
// func TestPrecedenceOrder(t *testing.T) {
// 	// Save original environment variables to restore later
// 	originalEnv := os.Environ()
// 	defer func() {
// 		os.Clearenv()
// 		for _, env := range originalEnv {
// 			env := env
// 			key, value, _ := strings.Cut(env, "=")
// 			os.Setenv(key, value)
// 		}
// 	}()

// 	// Create temp YAML file
// 	yamlFile := createTempYamlFile(t)

// 	// Set up environment variables
// 	os.Clearenv()
// 	os.Setenv("SERVER_HOST", "env-host.example.com") // Should override YAML
// 	os.Setenv("DATABASE_PORT", "5678")               // Should override YAML

// 	// Set up command line args
// 	testArgs := []string{
// 		"--server-port", "4040", // Should override env and YAML
// 	}

// 	cfg := TestConfig{}

// 	err := config.Parse(&cfg, config.ConfigOptions{
// 		YamlFiles: []string{yamlFile},
// 		Args:      testArgs,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed in precedence test: %v", err)
// 	}

// 	// Verify precedence:
// 	// 1. ServerHost: env (env-host) should override YAML (yaml-host)
// 	if cfg.ServerHost != "env-host.example.com" {
// 		t.Errorf("Precedence error: Expected ServerHost from env 'env-host.example.com', got '%s'", cfg.ServerHost)
// 	}

// 	// 2. ServerPort: flag (4040) should override YAML (9090)
// 	if cfg.ServerPort != 4040 {
// 		t.Errorf("Precedence error: Expected ServerPort from flag 4040, got %d", cfg.ServerPort)
// 	}

// 	// 3. Database.Port: env (5678) should override YAML (5433)
// 	if cfg.DB.Port != 5678 {
// 		t.Errorf("Precedence error: Expected Database.Port from env 5678, got %d", cfg.DB.Port)
// 	}

// 	// 4. EnableDebug: from YAML (true) should override default (false)
// 	if !cfg.EnableDebug {
// 		t.Errorf("Precedence error: Expected EnableDebug from YAML true, got false")
// 	}
// }

// // TestAutomaticOptionality tests that fields with defaults are automatically optional
// func TestAutomaticOptionality(t *testing.T) {
// 	type OptionalityTestConfig struct {
// 		// Field with default - should be optional
// 		WithDefault string `default:"default-value"`

// 		// Field without default - should be required
// 		Required string

// 		// Field without default but marked optional
// 		ExplicitlyOptional string `optional:"true"`

// 		// Nested struct
// 		Nested struct {
// 			// Field with default in nested struct
// 			WithDefault int `default:"123"`

// 			// Required field in nested struct
// 			Required int
// 		}
// 	}

// 	// Test with all required fields missing - should fail
// 	cfg1 := OptionalityTestConfig{}
// 	err1 := config.Parse(&cfg1, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipEnv:   true,
// 		SkipFlags: true,
// 	})

// 	if err1 == nil {
// 		t.Errorf("Expected validation error for missing required fields, got nil")
// 	}

// 	// Test with required fields provided via environment variables - should succeed
// 	os.Clearenv()
// 	os.Setenv("REQUIRED", "value1")
// 	os.Setenv("NESTED_REQUIRED", "456")

// 	cfg2 := OptionalityTestConfig{}
// 	err2 := config.Parse(&cfg2, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err2 != nil {
// 		t.Errorf("Unexpected error when required fields are provided: %v", err2)
// 	}

// 	// Verify values
// 	if cfg2.Required != "value1" {
// 		t.Errorf("Expected Required to be 'value1', got '%s'", cfg2.Required)
// 	}
// 	if cfg2.Nested.Required != 456 {
// 		t.Errorf("Expected Nested.Required to be 456, got %d", cfg2.Nested.Required)
// 	}
// 	if cfg2.WithDefault != "default-value" {
// 		t.Errorf("Expected WithDefault to be 'default-value', got '%s'", cfg2.WithDefault)
// 	}
// 	if cfg2.Nested.WithDefault != 123 {
// 		t.Errorf("Expected Nested.WithDefault to be 123, got %d", cfg2.Nested.WithDefault)
// 	}
// }

// // TestValidationRules tests that validation rules are properly applied
// func TestValidationRules(t *testing.T) {
// 	type ValidationTestConfig struct {
// 		// Field with validation rule
// 		Port int `default:"8080" validate:"min=1024,max=65535" desc:"Server port"`

// 		// Field with validation rule and no default
// 		LogLevel string `validate:"oneof=debug|info|warn|error" desc:"Logging level"`

// 		// Nested struct with validation
// 		Database struct {
// 			MaxConns int `default:"10" validate:"min=1,max=100" desc:"Maximum database connections"`
// 		}
// 	}

// 	// Test with valid values
// 	os.Clearenv()
// 	os.Setenv("PORT", "8080")
// 	os.Setenv("LOG_LEVEL", "info")

// 	cfg1 := ValidationTestConfig{}
// 	err1 := config.Parse(&cfg1, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err1 != nil {
// 		t.Errorf("Unexpected validation error with valid values: %v", err1)
// 	}

// 	// Test with invalid port (out of range)
// 	os.Clearenv()
// 	os.Setenv("PORT", "80") // Below min=1024
// 	os.Setenv("LOG_LEVEL", "info")

// 	cfg2 := ValidationTestConfig{}
// 	err2 := config.Parse(&cfg2, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err2 == nil {
// 		t.Errorf("Expected validation error for out-of-range port, got nil")
// 	}

// 	// Test with invalid log level (not in allowed values)
// 	os.Clearenv()
// 	os.Setenv("PORT", "8080")
// 	os.Setenv("LOG_LEVEL", "verbose") // Not in oneof=debug|info|warn|error

// 	cfg3 := ValidationTestConfig{}
// 	err3 := config.Parse(&cfg3, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err3 == nil {
// 		t.Errorf("Expected validation error for invalid log level, got nil")
// 	}

// 	// Test with multiple validation errors
// 	os.Clearenv()
// 	os.Setenv("PORT", "100000")            // Above max=65535
// 	os.Setenv("LOG_LEVEL", "trace")        // Not in allowed values
// 	os.Setenv("DATABASE_MAX_CONNS", "200") // Above max=100

// 	cfg4 := ValidationTestConfig{}
// 	err4 := config.Parse(&cfg4, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	if err4 == nil {
// 		t.Errorf("Expected validation errors for multiple invalid values, got nil")
// 	}
// }

// // TestConfigWithoutPointer tests that an error is returned when a non-pointer is passed
// func TestConfigWithoutPointer(t *testing.T) {
// 	cfg := TestConfig{}

// 	// Pass the struct directly, not a pointer
// 	err := config.Parse(cfg, config.ConfigOptions{})

// 	if err == nil {
// 		t.Errorf("Expected error when passing non-pointer to Parse, got nil")
// 	}
// }

// // TestErrorHandling tests various error conditions
// func TestErrorHandling(t *testing.T) {
// 	// Test with invalid YAML file
// 	cfg1 := TestConfig{}
// 	err1 := config.Parse(&cfg1, config.ConfigOptions{
// 		YamlFiles: []string{"nonexistent.yaml", "/tmp/also-nonexistent.yaml"},
// 		SkipEnv:   true,
// 		SkipFlags: true,
// 	})

// 	// This should not return an error, as nonexistent files are skipped silently
// 	if err1 != nil {
// 		t.Errorf("Unexpected error with nonexistent YAML files: %v", err1)
// 	}

// 	// Test with corrupted environment variable (wrong type)
// 	os.Clearenv()
// 	os.Setenv("SERVER_PORT", "not-an-integer")

// 	cfg2 := TestConfig{}
// 	err2 := config.Parse(&cfg2, config.ConfigOptions{
// 		SkipYaml:  true,
// 		SkipFlags: true,
// 	})

// 	// This should return an error
// 	if err2 == nil {
// 		t.Errorf("Expected error with invalid environment variable type, got nil")
// 	}

// 	// Test with corrupted flag value (wrong type)
// 	testArgs := []string{
// 		"--server-port", "not-an-integer",
// 	}

// 	cfg3 := TestConfig{}
// 	err3 := config.Parse(&cfg3, config.ConfigOptions{
// 		SkipYaml: true,
// 		SkipEnv:  true,
// 		Args:     testArgs,
// 	})

// 	// This should return an error
// 	if err3 == nil {
// 		t.Errorf("Expected error with invalid flag value type, got nil")
// 	}
// }

// // TestMultipleYamlFiles tests loading configuration from multiple YAML files with override
// func TestMultipleYamlFiles(t *testing.T) {
// 	// Create first YAML file (base config)
// 	baseYamlContent := `
// serverHost: base-host.example.com
// serverPort: 8001
// database:
//   host: base-db.example.com
//   port: 5000
// `
// 	tmpDir := t.TempDir()
// 	baseYamlFile := filepath.Join(tmpDir, "base.yaml")
// 	err := os.WriteFile(baseYamlFile, []byte(baseYamlContent), 0644)
// 	if err != nil {
// 		t.Fatalf("Failed to create base YAML file: %v", err)
// 	}

// 	// Create second YAML file (override)
// 	overrideYamlContent := `
// serverHost: override-host.example.com
// database:
//   port: 6000
// `
// 	overrideYamlFile := filepath.Join(tmpDir, "override.yaml")
// 	err = os.WriteFile(overrideYamlFile, []byte(overrideYamlContent), 0644)
// 	if err != nil {
// 		t.Fatalf("Failed to create override YAML file: %v", err)
// 	}

// 	cfg := TestConfig{}

// 	err = config.Parse(&cfg, config.ConfigOptions{
// 		YamlFiles: []string{baseYamlFile, overrideYamlFile},
// 		SkipEnv:   true,
// 		SkipFlags: true,
// 	})

// 	if err != nil {
// 		t.Fatalf("Parse failed with multiple YAML files: %v", err)
// 	}

// 	// Verify that override file took precedence for serverHost
// 	if cfg.ServerHost != "override-host.example.com" {
// 		t.Errorf("Expected ServerHost to be 'override-host.example.com', got '%s'", cfg.ServerHost)
// 	}

// 	// Verify that override file took precedence for database.port
// 	if cfg.DB.Port != 6000 {
// 		t.Errorf("Expected Database.Port to be 6000, got %d", cfg.DB.Port)
// 	}

// 	// Verify that serverPort from base file was applied (not overridden)
// 	if cfg.ServerPort != 8001 {
// 		t.Errorf("Expected ServerPort to be 8001, got %d", cfg.ServerPort)
// 	}

// 	// Verify that database.host from base file was applied (not overridden)
// 	if cfg.DB.Host != "base-db.example.com" {
// 		t.Errorf("Expected Database.Host to be 'base-db.example.com', got '%s'", cfg.DB.Host)
// 	}
// }
