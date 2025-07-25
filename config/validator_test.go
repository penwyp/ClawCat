package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidatePlan(t *testing.T) {
	tests := []struct {
		plan    string
		wantErr bool
	}{
		{"free", false},
		{"pro", false},
		{"team", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			err := ValidatePlan(tt.plan)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTheme(t *testing.T) {
	tests := []struct {
		theme   string
		wantErr bool
	}{
		{"dark", false},
		{"light", false},
		{"high-contrast", false},
		{"auto", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.theme, func(t *testing.T) {
			err := ValidateTheme(tt.theme)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"fatal", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			err := ValidateLogLevel(tt.level)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePaths(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{"empty paths", []string{}, false},
		{"valid path", []string{"."}, false},
		{"invalid path", []string{"/nonexistent/path"}, true},
		{"empty string in paths", []string{""}, true},
		{"mixed valid/invalid", []string{".", "/nonexistent"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePaths(tt.paths)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_ValidateApp(t *testing.T) {
	validator := NewStandardValidator()

	tests := []struct {
		name    string
		app     AppConfig
		wantErr bool
	}{
		{
			name: "valid config",
			app: AppConfig{
				LogLevel: "info",
				Timezone: "Local",
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			app: AppConfig{
				LogLevel: "invalid",
				Timezone: "Local",
			},
			wantErr: true,
		},
		{
			name: "invalid timezone",
			app: AppConfig{
				LogLevel: "info",
				Timezone: "Invalid/Timezone",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateApp(&tt.app)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_ValidateData(t *testing.T) {
	validator := NewStandardValidator()

	tests := []struct {
		name    string
		data    DataConfig
		wantErr bool
	}{
		{
			name: "valid config",
			data: DataConfig{
				WatchInterval: 100 * time.Millisecond,
				MaxFileSize:   1024 * 1024,
				CacheSize:     50,
			},
			wantErr: false,
		},
		{
			name: "watch interval too small",
			data: DataConfig{
				WatchInterval: 5 * time.Millisecond,
				MaxFileSize:   1024 * 1024,
				CacheSize:     50,
			},
			wantErr: true,
		},
		{
			name: "max file size too small",
			data: DataConfig{
				WatchInterval: 100 * time.Millisecond,
				MaxFileSize:   100,
				CacheSize:     50,
			},
			wantErr: true,
		},
		{
			name: "negative cache size",
			data: DataConfig{
				WatchInterval: 100 * time.Millisecond,
				MaxFileSize:   1024 * 1024,
				CacheSize:     -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateData(&tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_ValidateUI(t *testing.T) {
	validator := NewStandardValidator()

	tests := []struct {
		name    string
		ui      UIConfig
		wantErr bool
	}{
		{
			name: "valid config",
			ui: UIConfig{
				Theme:         "dark",
				RefreshRate:   time.Second,
				ChartHeight:   10,
				TablePageSize: 20,
				DateFormat:    "2006-01-02",
				TimeFormat:    "15:04:05",
			},
			wantErr: false,
		},
		{
			name: "invalid theme",
			ui: UIConfig{
				Theme:         "invalid",
				RefreshRate:   time.Second,
				ChartHeight:   10,
				TablePageSize: 20,
			},
			wantErr: true,
		},
		{
			name: "refresh rate too fast",
			ui: UIConfig{
				Theme:         "dark",
				RefreshRate:   50 * time.Millisecond,
				ChartHeight:   10,
				TablePageSize: 20,
			},
			wantErr: true,
		},
		{
			name: "chart height too small",
			ui: UIConfig{
				Theme:         "dark",
				RefreshRate:   time.Second,
				ChartHeight:   2,
				TablePageSize: 20,
			},
			wantErr: true,
		},
		{
			name: "table page size too small",
			ui: UIConfig{
				Theme:         "dark",
				RefreshRate:   time.Second,
				ChartHeight:   10,
				TablePageSize: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateUI(&tt.ui)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_ValidatePerformance(t *testing.T) {
	validator := NewStandardValidator()

	tests := []struct {
		name    string
		perf    PerformanceConfig
		wantErr bool
	}{
		{
			name: "valid config",
			perf: PerformanceConfig{
				WorkerCount: 4,
				BufferSize:  64 * 1024,
				BatchSize:   100,
				MaxMemory:   500 * 1024 * 1024,
				GCInterval:  5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "worker count too small",
			perf: PerformanceConfig{
				WorkerCount: 0,
				BufferSize:  64 * 1024,
				BatchSize:   100,
				MaxMemory:   500 * 1024 * 1024,
				GCInterval:  5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "buffer size too small",
			perf: PerformanceConfig{
				WorkerCount: 4,
				BufferSize:  100,
				BatchSize:   100,
				MaxMemory:   500 * 1024 * 1024,
				GCInterval:  5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "GC interval too short",
			perf: PerformanceConfig{
				WorkerCount: 4,
				BufferSize:  64 * 1024,
				BatchSize:   100,
				MaxMemory:   500 * 1024 * 1024,
				GCInterval:  5 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validatePerformance(&tt.perf)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_ValidateSubscription(t *testing.T) {
	validator := NewStandardValidator()

	tests := []struct {
		name    string
		sub     SubscriptionConfig
		wantErr bool
	}{
		{
			name: "valid config",
			sub: SubscriptionConfig{
				Plan:           "pro",
				WarnThreshold:  0.8,
				AlertThreshold: 0.95,
			},
			wantErr: false,
		},
		{
			name: "invalid plan",
			sub: SubscriptionConfig{
				Plan:           "invalid",
				WarnThreshold:  0.8,
				AlertThreshold: 0.95,
			},
			wantErr: true,
		},
		{
			name: "invalid thresholds - warn >= alert",
			sub: SubscriptionConfig{
				Plan:           "pro",
				WarnThreshold:  0.95,
				AlertThreshold: 0.8,
			},
			wantErr: true,
		},
		{
			name: "threshold out of range",
			sub: SubscriptionConfig{
				Plan:           "pro",
				WarnThreshold:  1.5,
				AlertThreshold: 0.95,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateSubscription(&tt.sub)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStandardValidator_Validate(t *testing.T) {
	validator := NewStandardValidator()

	// Test with valid configuration
	cfg := DefaultConfig()
	err := validator.Validate(cfg)
	assert.NoError(t, err)

	// Test with invalid configuration
	cfg.App.LogLevel = "invalid"
	err = validator.Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "app:")
}
