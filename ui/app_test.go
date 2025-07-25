package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewApp(t *testing.T) {
	config := DefaultConfig
	app := NewApp(config)
	
	assert.NotNil(t, app)
	assert.Equal(t, config, app.config)
	assert.NotNil(t, app.program)
	assert.NotNil(t, app.ctx)
	assert.NotNil(t, app.cancel)
}

func TestApp_GetConfig(t *testing.T) {
	config := Config{
		RefreshRate:   2 * time.Second,
		Theme:         "light",
		ShowSpinner:   false,
		CompactMode:   true,
		ChartHeight:   8,
		TablePageSize: 15,
	}
	
	app := NewApp(config)
	result := app.GetConfig()
	
	assert.Equal(t, config, result)
}

func TestApp_IsRunning(t *testing.T) {
	app := NewApp(DefaultConfig)
	
	// App should be "running" initially (context not cancelled)
	assert.True(t, app.IsRunning())
	
	// Stop the app
	err := app.Stop()
	assert.NoError(t, err)
	
	// App should no longer be running
	assert.False(t, app.IsRunning())
}

func TestApp_UpdateConfig(t *testing.T) {
	app := NewApp(DefaultConfig)
	
	// Stop the app to prevent hanging on Send
	app.Stop()
	
	newConfig := Config{
		RefreshRate:   3 * time.Second,
		Theme:         "dark",
		ShowSpinner:   true,
		CompactMode:   false,
		ChartHeight:   12,
		TablePageSize: 25,
	}
	
	app.UpdateConfig(newConfig)
	
	assert.Equal(t, newConfig, app.config)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig
	
	assert.Equal(t, time.Second, config.RefreshRate)
	assert.Equal(t, "dark", config.Theme)
	assert.True(t, config.ShowSpinner)
	assert.False(t, config.CompactMode)
	assert.Equal(t, 10, config.ChartHeight)
	assert.Equal(t, 20, config.TablePageSize)
}