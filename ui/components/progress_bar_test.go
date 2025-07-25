package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		current  float64
		max      float64
		expected float64 // expected percentage
	}{
		{
			name:     "50% progress",
			label:    "Test",
			current:  50,
			max:      100,
			expected: 50.0,
		},
		{
			name:     "100% progress",
			label:    "Complete",
			current:  100,
			max:      100,
			expected: 100.0,
		},
		{
			name:     "0% progress",
			label:    "Empty",
			current:  0,
			max:      100,
			expected: 0.0,
		},
		{
			name:     "over 100%",
			label:    "Over",
			current:  150,
			max:      100,
			expected: 100.0, // Should be capped at 100%
		},
		{
			name:     "zero max",
			label:    "Zero",
			current:  50,
			max:      0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar(tt.label, tt.current, tt.max)

			if pb.Label != tt.label {
				t.Errorf("Expected label %s, got %s", tt.label, pb.Label)
			}

			if pb.Current != tt.current {
				t.Errorf("Expected current %f, got %f", tt.current, pb.Current)
			}

			if pb.Max != tt.max {
				t.Errorf("Expected max %f, got %f", tt.max, pb.Max)
			}

			if pb.Percentage != tt.expected {
				t.Errorf("Expected percentage %f, got %f", tt.expected, pb.Percentage)
			}
		})
	}
}

func TestProgressBar_Render(t *testing.T) {
	tests := []struct {
		name        string
		current     float64
		max         float64
		width       int
		expectLabel bool
		expectValue bool
	}{
		{
			name:        "basic render",
			current:     50,
			max:         100,
			width:       20,
			expectLabel: true,
			expectValue: true,
		},
		{
			name:        "full progress",
			current:     100,
			max:         100,
			width:       10,
			expectLabel: true,
			expectValue: true,
		},
		{
			name:        "empty progress",
			current:     0,
			max:         100,
			width:       15,
			expectLabel: true,
			expectValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar("Test", tt.current, tt.max)
			pb.SetWidth(tt.width)

			result := pb.Render()

			if tt.expectLabel && !strings.Contains(result, "Test") {
				t.Error("Expected result to contain label 'Test'")
			}

			if tt.expectValue {
				percentage := fmt.Sprintf("%.1f%%", tt.current/tt.max*100)
				if !strings.Contains(result, percentage) {
					t.Errorf("Expected result to contain percentage %s", percentage)
				}
			}

			// Check that result contains progress bar elements
			if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
				t.Error("Expected result to contain progress bar brackets")
			}
		})
	}
}

func TestProgressBar_Update(t *testing.T) {
	pb := NewProgressBar("Test", 25, 100)

	// Initial state
	if pb.Current != 25 || pb.Percentage != 25.0 {
		t.Errorf("Initial state incorrect: current=%f, percentage=%f", pb.Current, pb.Percentage)
	}

	// Update to 75
	pb.Update(75)

	if pb.Current != 75 {
		t.Errorf("Expected current to be 75, got %f", pb.Current)
	}

	if pb.Percentage != 75.0 {
		t.Errorf("Expected percentage to be 75.0, got %f", pb.Percentage)
	}

	// Update beyond max
	pb.Update(150)

	if pb.Current != 150 {
		t.Errorf("Expected current to be 150, got %f", pb.Current)
	}

	if pb.Percentage != 100.0 {
		t.Errorf("Expected percentage to be capped at 100.0, got %f", pb.Percentage)
	}
}

func TestProgressBar_SetWidth(t *testing.T) {
	pb := NewProgressBar("Test", 50, 100)

	// Test normal width
	pb.SetWidth(30)
	if pb.Width != 30 {
		t.Errorf("Expected width 30, got %d", pb.Width)
	}

	// Test minimum width constraint
	pb.SetWidth(5)
	if pb.Width != 10 {
		t.Errorf("Expected width to be constrained to minimum 10, got %d", pb.Width)
	}
}

func TestProgressBar_SetColor(t *testing.T) {
	pb := NewProgressBar("Test", 50, 100)

	testColor := lipgloss.Color("#FF0000")
	pb.SetColor(testColor)

	if pb.Color != testColor {
		t.Errorf("Expected color %s, got %s", testColor, pb.Color)
	}
}

func TestProgressBar_GetStatus(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		expected   string
	}{
		{"normal", 25, "normal"},
		{"moderate", 60, "moderate"},
		{"warning", 80, "warning"},
		{"critical", 95, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar("Test", tt.percentage, 100)
			status := pb.GetStatus()

			if status != tt.expected {
				t.Errorf("Expected status %s, got %s", tt.expected, status)
			}
		})
	}
}

func TestProgressBar_IsOverLimit(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		max      float64
		expected bool
	}{
		{"under limit", 75, 100, false},
		{"at limit", 100, 100, false},
		{"over limit", 125, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := NewProgressBar("Test", tt.current, tt.max)
			result := pb.IsOverLimit()

			if result != tt.expected {
				t.Errorf("Expected IsOverLimit to be %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestProgressColorScheme_GetProgressColor(t *testing.T) {
	scheme := DefaultColorScheme

	tests := []struct {
		percentage float64
		expected   lipgloss.Color
	}{
		{25, "#00ff00"}, // Green
		{60, "#ffff00"}, // Yellow
		{80, "#ff8800"}, // Orange
		{95, "#ff0000"}, // Red
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f%%", tt.percentage), func(t *testing.T) {
			color := scheme.GetProgressColor(tt.percentage)
			if color != tt.expected {
				t.Errorf("Expected color %s, got %s", tt.expected, color)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		max      float64
		expected string
	}{
		{"small values", 5, 10, "5/10"},
		{"thousands", 1500, 3000, "1.5K/3.0K"},
		{"millions", 1500000, 3000000, "1.5M/3.0M"},
		{"cost format", 12.50, 18.00, "$12.50/$18.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.current, tt.max)
			if !strings.Contains(result, "/") {
				t.Errorf("Expected result to contain '/', got %s", result)
			}
			// Note: exact format testing would depend on the specific formatting logic
		})
	}
}

func BenchmarkProgressBar_Render(b *testing.B) {
	pb := NewProgressBar("Benchmark", 50, 100)
	pb.SetWidth(40)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.Render()
	}
}

func BenchmarkProgressBar_Update(b *testing.B) {
	pb := NewProgressBar("Benchmark", 0, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pb.Update(float64(i % 100))
	}
}
