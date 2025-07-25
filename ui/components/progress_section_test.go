package components

import (
	"strings"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/calculations"
)

func TestNewProgressSection(t *testing.T) {
	width := 100
	ps := NewProgressSection(width)

	if ps == nil {
		t.Fatal("NewProgressSection returned nil")
	}

	if ps.width != width {
		t.Errorf("Expected width %d, got %d", width, ps.width)
	}
}

func TestProgressSection_Update(t *testing.T) {
	ps := NewProgressSection(100)

	// Create test metrics
	metrics := &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-2 * time.Hour),
		CurrentTokens: 75000,
		CurrentCost:   12.50,
		BurnRate:      50.0,
	}

	limits := Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	// Update progress section
	ps.Update(metrics, limits)

	// Verify progress bars were created
	if ps.TokenProgress == nil {
		t.Error("TokenProgress should not be nil after update")
	}

	if ps.CostProgress == nil {
		t.Error("CostProgress should not be nil after update")
	}

	if ps.TimeProgress == nil {
		t.Error("TimeProgress should not be nil after update")
	}

	// Verify token progress values
	if ps.TokenProgress.Current != float64(metrics.CurrentTokens) {
		t.Errorf("Expected token current %f, got %f", float64(metrics.CurrentTokens), ps.TokenProgress.Current)
	}

	if ps.TokenProgress.Max != float64(limits.TokenLimit) {
		t.Errorf("Expected token max %f, got %f", float64(limits.TokenLimit), ps.TokenProgress.Max)
	}

	// Verify cost progress values
	if ps.CostProgress.Current != metrics.CurrentCost {
		t.Errorf("Expected cost current %f, got %f", metrics.CurrentCost, ps.CostProgress.Current)
	}

	if ps.CostProgress.Max != limits.CostLimit {
		t.Errorf("Expected cost max %f, got %f", limits.CostLimit, ps.CostProgress.Max)
	}
}

func TestProgressSection_Render(t *testing.T) {
	ps := NewProgressSection(100)

	// Test render without data
	result := ps.Render()
	if !strings.Contains(result, "Progress Overview") {
		t.Errorf("Expected 'Progress Overview' in render output, got: %s", result)
	}

	// Add test data
	metrics := &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-1 * time.Hour),
		CurrentTokens: 50000,
		CurrentCost:   9.00,
		BurnRate:      25.0,
	}

	limits := Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	ps.Update(metrics, limits)

	// Test render with data
	result = ps.Render()

	// Should contain progress overview title
	if !strings.Contains(result, "Progress Overview") {
		t.Error("Expected result to contain 'Progress Overview'")
	}

	// Should contain progress bars
	if !strings.Contains(result, "Token Usage") {
		t.Error("Expected result to contain 'Token Usage'")
	}

	if !strings.Contains(result, "Cost Usage") {
		t.Error("Expected result to contain 'Cost Usage'")
	}

	if !strings.Contains(result, "Time Elapsed") {
		t.Error("Expected result to contain 'Time Elapsed'")
	}
}

func TestProgressSection_GetSummary(t *testing.T) {
	ps := NewProgressSection(100)

	// Test without data
	summary := ps.GetSummary()
	if !strings.Contains(summary, "No progress data") {
		t.Error("Expected 'No progress data' message when no data")
	}

	// Add test data
	metrics := &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-1 * time.Hour),
		CurrentTokens: 50000,
		CurrentCost:   9.00,
	}

	limits := Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	ps.Update(metrics, limits)

	// Test with data
	summary = ps.GetSummary()

	// Should contain percentages
	if !strings.Contains(summary, "%") {
		t.Error("Expected summary to contain percentage values")
	}

	// Should contain tokens, cost, and time
	if !strings.Contains(summary, "Tokens") {
		t.Error("Expected summary to contain 'Tokens'")
	}

	if !strings.Contains(summary, "Cost") {
		t.Error("Expected summary to contain 'Cost'")
	}

	if !strings.Contains(summary, "Time") {
		t.Error("Expected summary to contain 'Time'")
	}
}

func TestProgressSection_HasCriticalStatus(t *testing.T) {
	ps := NewProgressSection(100)

	// Test without critical status
	metrics := &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-30 * time.Minute),
		CurrentTokens: 25000,
		CurrentCost:   4.50,
	}

	limits := Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	ps.Update(metrics, limits)

	if ps.HasCriticalStatus() {
		t.Error("Should not have critical status with low usage")
	}

	// Test with critical token usage
	metrics.CurrentTokens = 95000
	ps.Update(metrics, limits)

	if !ps.HasCriticalStatus() {
		t.Error("Should have critical status with high token usage")
	}

	// Test with critical cost usage
	metrics.CurrentTokens = 25000
	metrics.CurrentCost = 16.50
	ps.Update(metrics, limits)

	if !ps.HasCriticalStatus() {
		t.Error("Should have critical status with high cost usage")
	}
}

func TestProgressSection_GetWorstStatus(t *testing.T) {
	ps := NewProgressSection(100)

	tests := []struct {
		name       string
		tokenUsage float64
		costUsage  float64
		expected   string
	}{
		{
			name:       "normal usage",
			tokenUsage: 25.0,
			costUsage:  25.0,
			expected:   "normal",
		},
		{
			name:       "moderate usage",
			tokenUsage: 60.0,
			costUsage:  25.0,
			expected:   "moderate",
		},
		{
			name:       "warning usage",
			tokenUsage: 25.0,
			costUsage:  80.0,
			expected:   "warning",
		},
		{
			name:       "critical usage",
			tokenUsage: 95.0,
			costUsage:  25.0,
			expected:   "critical",
		},
	}

	limits := Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &calculations.RealtimeMetrics{
				SessionStart:  time.Now().Add(-1 * time.Hour),
				CurrentTokens: int(float64(limits.TokenLimit) * tt.tokenUsage / 100),
				CurrentCost:   limits.CostLimit * tt.costUsage / 100,
			}

			ps.Update(metrics, limits)
			status := ps.GetWorstStatus()

			if status != tt.expected {
				t.Errorf("Expected status %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestProgressSection_SetWidth(t *testing.T) {
	ps := NewProgressSection(100)

	newWidth := 150
	ps.SetWidth(newWidth)

	if ps.width != newWidth {
		t.Errorf("Expected width %d, got %d", newWidth, ps.width)
	}
}

func TestProgressSection_SetHeight(t *testing.T) {
	ps := NewProgressSection(100)

	newHeight := 25
	ps.SetHeight(newHeight)

	if ps.height != newHeight {
		t.Errorf("Expected height %d, got %d", newHeight, ps.height)
	}
}

func TestCalculateBarWidth(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		expected int
	}{
		{"small width", 50, 20},   // Below reserved space, should return minimum
		{"normal width", 100, 60}, // Normal case
		{"large width", 200, 60},  // Above maximum, should be capped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := NewProgressSection(tt.width)
			barWidth := ps.calculateBarWidth()

			if barWidth < 20 {
				t.Errorf("Bar width should not be less than 20, got %d", barWidth)
			}

			if barWidth > 60 {
				t.Errorf("Bar width should not be more than 60, got %d", barWidth)
			}
		})
	}
}

func TestFormatProgressDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"minutes only", 45 * time.Minute, "45m"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},
		{"hours only", 3 * time.Hour, "3h 0m"},
		{"zero duration", 0, "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatProgressDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Test helper to create test metrics
func createTestMetrics(tokenUsage, costUsage float64, sessionDuration time.Duration) *calculations.RealtimeMetrics {
	return &calculations.RealtimeMetrics{
		SessionStart:  time.Now().Add(-sessionDuration),
		CurrentTokens: int(100000 * tokenUsage / 100),
		CurrentCost:   18.00 * costUsage / 100,
		BurnRate:      25.0,
	}
}

func BenchmarkProgressSection_Update(b *testing.B) {
	ps := NewProgressSection(100)
	metrics := createTestMetrics(50, 50, time.Hour)
	limits := Limits{TokenLimit: 100000, CostLimit: 18.00}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Update(metrics, limits)
	}
}

func BenchmarkProgressSection_Render(b *testing.B) {
	ps := NewProgressSection(100)
	metrics := createTestMetrics(50, 50, time.Hour)
	limits := Limits{TokenLimit: 100000, CostLimit: 18.00}
	ps.Update(metrics, limits)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ps.Render()
	}
}
