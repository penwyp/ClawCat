package ui

import (
	"testing"
	"time"

	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/sessions"
	"github.com/stretchr/testify/assert"
)

func TestNewModel(t *testing.T) {
	config := DefaultConfig
	model := NewModel(config)

	assert.Equal(t, config, model.config)
	assert.Equal(t, ViewMonitor, model.view)
	assert.True(t, model.ready)
	assert.True(t, model.loading)
	assert.NotNil(t, model.keys)
	assert.NotNil(t, model.styles)
	assert.NotNil(t, model.spinner)
	assert.NotNil(t, model.monitor)
	assert.NotNil(t, model.analytics)
}

func TestModel_SwitchView(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Test valid view switches
	model.SwitchView(ViewAnalytics)
	assert.Equal(t, ViewAnalytics, model.view)

	model.SwitchView(ViewMonitor)
	assert.Equal(t, ViewMonitor, model.view)
}

func TestModel_NextView(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Start at Monitor (0)
	assert.Equal(t, ViewMonitor, model.view)

	// Next should be Analytics (1)
	model.NextView()
	assert.Equal(t, ViewAnalytics, model.view)

	// Next should wrap back to Monitor (0)
	model.NextView()
	assert.Equal(t, ViewMonitor, model.view)
}

func TestModel_PrevView(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Start at Monitor (0)
	assert.Equal(t, ViewMonitor, model.view)

	// Previous should wrap to Analytics (1)
	model.PrevView()
	assert.Equal(t, ViewAnalytics, model.view)

	// Previous should wrap back to Monitor (0)
	model.PrevView()
	assert.Equal(t, ViewMonitor, model.view)
}

func TestModel_GetCurrentView(t *testing.T) {
	model := NewModel(DefaultConfig)

	testCases := []struct {
		view     ViewType
		expected string
	}{
		{ViewMonitor, "Monitor"},
		{ViewAnalytics, "Analytics"},
	}

	for _, tc := range testCases {
		model.SwitchView(tc.view)
		assert.Equal(t, tc.expected, model.GetCurrentView())
	}
}

func TestModel_Resize(t *testing.T) {
	model := NewModel(DefaultConfig)

	width, height := 100, 50
	model.Resize(width, height)

	assert.Equal(t, width, model.width)
	assert.Equal(t, height, model.height)
}

func TestModel_SetReady(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Initially ready (changed to avoid stuck loading)
	assert.True(t, model.IsReady())

	// Set not ready
	model.SetReady(false)
	assert.False(t, model.IsReady())

	// Set ready again
	model.SetReady(true)
	assert.True(t, model.IsReady())
}

func TestModel_SetLoading(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Initially loading
	assert.True(t, model.IsLoading())

	// Set not loading
	model.SetLoading(false)
	assert.False(t, model.IsLoading())

	// Set loading
	model.SetLoading(true)
	assert.True(t, model.IsLoading())
}

func TestModel_UpdateSessions(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Create test sessions
	testSessions := []*sessions.Session{
		{
			ID:        "test-1",
			StartTime: time.Now().Add(-time.Hour),
			EndTime:   time.Now().Add(-30 * time.Minute),
			Entries: []models.UsageEntry{
				{
					Timestamp:    time.Now().Add(-time.Hour),
					Model:        "claude-3-sonnet",
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
					CostUSD:      0.75,
				},
			},
		},
		{
			ID:        "test-2",
			StartTime: time.Now().Add(-30 * time.Minute),
			Entries: []models.UsageEntry{
				{
					Timestamp:    time.Now().Add(-30 * time.Minute),
					Model:        "claude-3-haiku",
					InputTokens:  200,
					OutputTokens: 100,
					TotalTokens:  300,
					CostUSD:      0.15,
				},
			},
		},
	}

	model.UpdateSessions(testSessions)

	assert.Equal(t, testSessions, model.GetSessions())
	assert.False(t, model.IsLoading())

	// Check that statistics were updated
	stats := model.GetStats()
	assert.Equal(t, 2, stats.SessionCount)
	assert.Equal(t, int64(450), stats.TotalTokens) // 150 + 300
	assert.Equal(t, 0.90, stats.TotalCost)         // 0.75 + 0.15
}

func TestModel_UpdateEntries(t *testing.T) {
	model := NewModel(DefaultConfig)

	testEntries := []models.UsageEntry{
		{
			Model:        "claude-3-sonnet",
			InputTokens:  500,
			OutputTokens: 250,
			TotalTokens:  750,
			CostUSD:      3.75,
		},
		{
			Model:        "claude-3-haiku",
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			CostUSD:      0.75,
		},
	}

	model.UpdateEntries(testEntries)

	assert.Equal(t, testEntries, model.GetEntries())
}

func TestStatistics_Calculation(t *testing.T) {
	model := NewModel(DefaultConfig)

	// Create a session with known values for testing
	now := time.Now()
	testSessions := []*sessions.Session{
		{
			ID:        "test-session",
			StartTime: now.Add(-time.Hour),
			EndTime:   now,
			Entries: []models.UsageEntry{
				{
					Model:       "claude-3-sonnet",
					TotalTokens: 1000,
					CostUSD:     5.0,
				},
				{
					Model:       "claude-3-haiku",
					TotalTokens: 500,
					CostUSD:     1.0,
				},
			},
		},
	}

	model.UpdateSessions(testSessions)
	stats := model.GetStats()

	assert.Equal(t, 1, stats.SessionCount)
	assert.Equal(t, int64(1500), stats.TotalTokens)
	assert.Equal(t, 6.0, stats.TotalCost)
	assert.Equal(t, 6.0, stats.AverageCost) // 6.0 / 1 session
}
