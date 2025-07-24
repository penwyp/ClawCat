package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvWithDefault(t *testing.T) {
	key := "TEST_ENV_STRING"
	defaultValue := "default"
	customValue := "custom"

	// Test with env var not set
	result := GetEnvWithDefault(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with env var set
	os.Setenv(key, customValue)
	defer os.Unsetenv(key)
	result = GetEnvWithDefault(key, defaultValue)
	assert.Equal(t, customValue, result)
}

func TestGetEnvBool(t *testing.T) {
	key := "TEST_ENV_BOOL"
	defaultValue := false

	// Test with env var not set
	result := GetEnvBool(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with valid bool values
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"invalid", defaultValue}, // Should return default for invalid values
	}

	for _, tt := range tests {
		os.Setenv(key, tt.value)
		result := GetEnvBool(key, defaultValue)
		assert.Equal(t, tt.expected, result)
	}

	os.Unsetenv(key)
}

func TestGetEnvInt(t *testing.T) {
	key := "TEST_ENV_INT"
	defaultValue := 42

	// Test with env var not set
	result := GetEnvInt(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with valid int values
	tests := []struct {
		value    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"-456", -456},
		{"invalid", defaultValue}, // Should return default for invalid values
	}

	for _, tt := range tests {
		os.Setenv(key, tt.value)
		result := GetEnvInt(key, defaultValue)
		assert.Equal(t, tt.expected, result)
	}

	os.Unsetenv(key)
}

func TestGetEnvFloat(t *testing.T) {
	key := "TEST_ENV_FLOAT"
	defaultValue := 3.14

	// Test with env var not set
	result := GetEnvFloat(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with valid float values
	tests := []struct {
		value    string
		expected float64
	}{
		{"1.23", 1.23},
		{"0.0", 0.0},
		{"-4.56", -4.56},
		{"invalid", defaultValue}, // Should return default for invalid values
	}

	for _, tt := range tests {
		os.Setenv(key, tt.value)
		result := GetEnvFloat(key, defaultValue)
		assert.Equal(t, tt.expected, result)
	}

	os.Unsetenv(key)
}

func TestGetEnvDuration(t *testing.T) {
	key := "TEST_ENV_DURATION"
	defaultValue := 5 * time.Second

	// Test with env var not set
	result := GetEnvDuration(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with valid duration values
	tests := []struct {
		value    string
		expected time.Duration
	}{
		{"1s", time.Second},
		{"5m", 5 * time.Minute},
		{"2h", 2 * time.Hour},
		{"100ms", 100 * time.Millisecond},
		{"invalid", defaultValue}, // Should return default for invalid values
	}

	for _, tt := range tests {
		os.Setenv(key, tt.value)
		result := GetEnvDuration(key, defaultValue)
		assert.Equal(t, tt.expected, result)
	}

	os.Unsetenv(key)
}

func TestGetEnvSlice(t *testing.T) {
	key := "TEST_ENV_SLICE"
	defaultValue := []string{"default"}

	// Test with env var not set
	result := GetEnvSlice(key, defaultValue)
	assert.Equal(t, defaultValue, result)

	// Test with comma-separated values
	tests := []struct {
		value    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a, b, c", []string{"a", "b", "c"}}, // Test trimming spaces
	}

	for _, tt := range tests {
		os.Setenv(key, tt.value)
		result := GetEnvSlice(key, defaultValue)
		assert.Equal(t, tt.expected, result)
	}

	os.Unsetenv(key)
}

func TestEnvMapper_ToCamelCase(t *testing.T) {
	mapper := NewEnvMapper("TEST")

	tests := []struct {
		input    string
		expected string
	}{
		{"log_level", "LogLevel"},
		{"max_file_size", "MaxFileSize"},
		{"gc_interval", "GCInterval"},
		{"simple", "Simple"},
		{"", ""},
	}

	for _, tt := range tests {
		result := mapper.toCamelCase(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestEnvMapper_Apply(t *testing.T) {
	// Set up test environment variables
	testEnvVars := map[string]string{
		"CLAWCAT_LOG_LEVEL":    "debug",
		"CLAWCAT_THEME":        "light",
		"CLAWCAT_CACHE_SIZE":   "100",
		"CLAWCAT_WORKER_COUNT": "8",
		"CLAWCAT_CACHE_ENABLED": "false",
	}

	for key, value := range testEnvVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	// Create config and apply environment variables
	cfg := DefaultConfig()
	mapper := NewEnvMapper("CLAWCAT")
	err := mapper.Apply(cfg)

	assert.NoError(t, err)
	assert.Equal(t, "debug", cfg.App.LogLevel)
	assert.Equal(t, "light", cfg.UI.Theme)
	assert.Equal(t, 100, cfg.Data.CacheSize)
	assert.Equal(t, 8, cfg.Performance.WorkerCount)
	assert.False(t, cfg.Data.CacheEnabled)
}

func TestLoadFromEnv(t *testing.T) {
	// Set up test environment variables
	os.Setenv("CLAWCAT_LOG_LEVEL", "warn")
	os.Setenv("CLAWCAT_THEME", "high-contrast")
	defer os.Unsetenv("CLAWCAT_LOG_LEVEL")
	defer os.Unsetenv("CLAWCAT_THEME")

	cfg := DefaultConfig()
	err := LoadFromEnv(cfg)

	assert.NoError(t, err)
	assert.Equal(t, "warn", cfg.App.LogLevel)
	assert.Equal(t, "high-contrast", cfg.UI.Theme)
}

func TestSetEnvDefaults(t *testing.T) {
	// Clear test environment first
	testKeys := []string{"CLAWCAT_LOG_LEVEL", "CLAWCAT_THEME", "CLAWCAT_PLAN"}
	for _, key := range testKeys {
		os.Unsetenv(key)
	}

	// Set defaults
	SetEnvDefaults()

	// Check that defaults were set
	assert.Equal(t, "info", os.Getenv("CLAWCAT_LOG_LEVEL"))
	assert.Equal(t, "dark", os.Getenv("CLAWCAT_THEME"))
	assert.Equal(t, "pro", os.Getenv("CLAWCAT_PLAN"))

	// Clean up
	for _, key := range testKeys {
		os.Unsetenv(key)
	}
}

func TestStandardEnvMappings(t *testing.T) {
	// Test that all standard mappings are present
	expectedMappings := []string{
		"CLAWCAT_LOG_LEVEL",
		"CLAWCAT_THEME",
		"CLAWCAT_PLAN",
		"CLAWCAT_CACHE_SIZE",
		"CLAWCAT_WORKER_COUNT",
	}

	for _, key := range expectedMappings {
		_, exists := StandardEnvMappings[key]
		assert.True(t, exists, "Missing standard mapping for %s", key)
	}
}