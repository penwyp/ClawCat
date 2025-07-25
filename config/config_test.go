package config

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test App config
	assert.Equal(t, "ClawCat", cfg.App.Name)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, "Local", cfg.App.Timezone)

	// Test Data config
	assert.True(t, cfg.Data.AutoDiscover)
	assert.Equal(t, 100*time.Millisecond, cfg.Data.WatchInterval)
	assert.Equal(t, int64(100*1024*1024), cfg.Data.MaxFileSize)
	assert.True(t, cfg.Data.CacheEnabled)
	assert.Equal(t, 50, cfg.Data.CacheSize)

	// Test UI config
	assert.Equal(t, "dark", cfg.UI.Theme)
	assert.Equal(t, time.Second, cfg.UI.RefreshRate)
	assert.False(t, cfg.UI.CompactMode)
	assert.True(t, cfg.UI.ShowSpinner)
	assert.Equal(t, 10, cfg.UI.ChartHeight)
	assert.Equal(t, 20, cfg.UI.TablePageSize)

	// Test Performance config
	assert.Equal(t, runtime.NumCPU(), cfg.Performance.WorkerCount)
	assert.Equal(t, 64*1024, cfg.Performance.BufferSize)
	assert.Equal(t, 100, cfg.Performance.BatchSize)
	assert.Equal(t, int64(500*1024*1024), cfg.Performance.MaxMemory)
	assert.Equal(t, 5*time.Minute, cfg.Performance.GCInterval)

	// Test Subscription config
	assert.Equal(t, "pro", cfg.Subscription.Plan)
	assert.Equal(t, 0.80, cfg.Subscription.WarnThreshold)
	assert.Equal(t, 0.95, cfg.Subscription.AlertThreshold)

	// Test Debug config
	assert.False(t, cfg.Debug.Enabled)
}

func TestMinimalConfig(t *testing.T) {
	cfg := MinimalConfig()

	// Should have minimal settings
	assert.False(t, cfg.Data.CacheEnabled)
	assert.Equal(t, 1, cfg.Performance.WorkerCount)
	assert.Equal(t, 1024, cfg.Performance.BufferSize)
	assert.Equal(t, 10, cfg.Performance.BatchSize)
	assert.True(t, cfg.UI.CompactMode)
	assert.False(t, cfg.UI.ShowSpinner)
}

func TestDevelopmentConfig(t *testing.T) {
	cfg := DevelopmentConfig()

	// Should have development settings
	assert.Equal(t, "debug", cfg.App.LogLevel)
	assert.True(t, cfg.Debug.Enabled)
	assert.Equal(t, 500*time.Millisecond, cfg.UI.RefreshRate)
	assert.Equal(t, time.Minute, cfg.Performance.GCInterval)
}

func TestProductionConfig(t *testing.T) {
	cfg := ProductionConfig()

	// Should have production settings
	assert.Equal(t, "warn", cfg.App.LogLevel)
	assert.False(t, cfg.Debug.Enabled)
	assert.Equal(t, runtime.NumCPU()*2, cfg.Performance.WorkerCount)
	assert.Equal(t, int64(1024*1024*1024), cfg.Performance.MaxMemory)
	assert.Equal(t, 10*time.Minute, cfg.Performance.GCInterval)
}

func TestConfigPaths(t *testing.T) {
	paths := ConfigPaths()

	expectedPaths := []string{
		"./clawcat.yaml",
		"$HOME/.config/clawcat/config.yaml",
		"$HOME/.clawcat/config.yaml",
		"/etc/clawcat/config.yaml",
	}

	assert.Equal(t, expectedPaths, paths)
}

func TestFormat(t *testing.T) {
	assert.Equal(t, 0, int(FormatYAML))
	assert.Equal(t, 1, int(FormatJSON))
	assert.Equal(t, 2, int(FormatTOML))
}
