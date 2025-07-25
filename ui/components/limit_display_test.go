package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/limits"
)

func TestNewLimitDisplay(t *testing.T) {
	ld := NewLimitDisplay()
	
	if ld == nil {
		t.Fatal("LimitDisplay should not be nil")
	}
	
	if ld.status != nil {
		t.Error("Initial status should be nil")
	}
	
	if ld.expanded {
		t.Error("Should not be expanded by default")
	}
	
	if ld.width != 0 {
		t.Error("Initial width should be 0")
	}
}

func TestLimitDisplay_Update(t *testing.T) {
	ld := NewLimitDisplay()
	
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{
			Name:      "Pro",
			Type:      limits.PlanPro,
			CostLimit: 18.0,
		},
		CurrentUsage: limits.Usage{
			Cost:   9.0,
			Tokens: 500000,
		},
		Percentage: 50.0,
	}
	
	ld.Update(status)
	
	if ld.status != status {
		t.Error("Status should be updated")
	}
}

func TestLimitDisplay_SetWidth(t *testing.T) {
	ld := NewLimitDisplay()
	
	ld.SetWidth(100)
	if ld.width != 100 {
		t.Errorf("Expected width 100, got %d", ld.width)
	}
}

func TestLimitDisplay_SetExpanded(t *testing.T) {
	ld := NewLimitDisplay()
	
	ld.SetExpanded(true)
	if !ld.expanded {
		t.Error("Should be expanded")
	}
	
	ld.SetExpanded(false)
	if ld.expanded {
		t.Error("Should not be expanded")
	}
}

func TestLimitDisplay_Render(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Test with no status
	result := ld.Render()
	if !strings.Contains(result, "Loading") {
		t.Error("Should show loading message when status is nil")
	}
	
	// Test with status
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{
			Name:      "Pro",
			Type:      limits.PlanPro,
			CostLimit: 18.0,
		},
		CurrentUsage: limits.Usage{
			Cost:   9.0,
			Tokens: 500000,
		},
		Percentage: 50.0,
		TimeToReset: 24 * time.Hour,
	}
	
	ld.Update(status)
	ld.SetWidth(80)
	
	// Test compact view
	ld.SetExpanded(false)
	compactResult := ld.Render()
	if compactResult == "" {
		t.Error("Compact render should not be empty")
	}
	
	// Test expanded view
	ld.SetExpanded(true)
	expandedResult := ld.Render()
	if expandedResult == "" {
		t.Error("Expanded render should not be empty")
	}
	
	// Expanded should be longer than compact
	if len(expandedResult) <= len(compactResult) {
		t.Error("Expanded view should be longer than compact view")
	}
}

func TestLimitDisplay_RenderWithWarning(t *testing.T) {
	ld := NewLimitDisplay()
	ld.SetWidth(80)
	ld.SetExpanded(true)
	
	warningLevel := &limits.WarningLevel{
		Threshold: 90.0,
		Message:   "90% limit reached",
		Severity:  limits.SeverityWarning,
	}
	
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{
			Name:      "Pro",
			Type:      limits.PlanPro,
			CostLimit: 18.0,
		},
		CurrentUsage: limits.Usage{
			Cost:   16.2, // 90% of 18
			Tokens: 900000,
		},
		Percentage:   90.0,
		WarningLevel: warningLevel,
		TimeToReset:  24 * time.Hour,
	}
	
	ld.Update(status)
	result := ld.Render()
	
	if !strings.Contains(result, "90%") {
		t.Error("Should contain percentage in warning")
	}
	
	if !strings.Contains(result, "⚠️") {
		t.Error("Should contain warning icon")
	}
}

func TestLimitDisplay_RenderWithRecommendations(t *testing.T) {
	ld := NewLimitDisplay()
	ld.SetWidth(80)
	ld.SetExpanded(true)
	
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{
			Name:      "Pro",
			Type:      limits.PlanPro,
			CostLimit: 18.0,
		},
		CurrentUsage: limits.Usage{
			Cost:   17.1, // 95% of 18
			Tokens: 950000,
		},
		Percentage: 95.0,
		Recommendations: []string{
			"Consider upgrading your plan",
			"Review your usage patterns",
		},
		TimeToReset: 24 * time.Hour,
	}
	
	ld.Update(status)
	result := ld.Render()
	
	if !strings.Contains(result, "Recommendations") {
		t.Error("Should contain recommendations section")
	}
	
	if !strings.Contains(result, "upgrading") {
		t.Error("Should contain recommendation text")
	}
}

func TestLimitDisplay_GetProgressColor(t *testing.T) {
	ld := NewLimitDisplay()
	
	tests := []struct {
		percentage   float64
		expectedHue  string // We'll check if it contains certain color indicators
	}{
		{25.0, "00FF00"},  // Green
		{60.0, "FFD700"},  // Gold
		{80.0, "FFA500"},  // Orange
		{92.0, "FF4500"},  // Orange-red
		{98.0, "FF0000"},  // Red
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f%%", tt.percentage), func(t *testing.T) {
			color := ld.getProgressColor(tt.percentage)
			colorStr := string(color)
			if !strings.Contains(colorStr, tt.expectedHue) {
				t.Errorf("Expected color to contain %s for %.1f%%, got %s", 
					tt.expectedHue, tt.percentage, colorStr)
			}
		})
	}
}

func TestLimitDisplay_FormatDuration(t *testing.T) {
	ld := NewLimitDisplay()
	
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Minute, "30m"},
		{2 * time.Hour, "2h 0m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25 * time.Hour, "1d 1h"},
		{50 * time.Hour, "2d 2h"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := ld.formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %v, want %v", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestLimitDisplay_FormatNumber(t *testing.T) {
	ld := NewLimitDisplay()
	
	tests := []struct {
		number   int64
		expected string
	}{
		{500, "500"},
		{1500, "1.5K"},
		{1000, "1.0K"},
		{1500000, "1.5M"},
		{2000000, "2.0M"},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.number), func(t *testing.T) {
			result := ld.formatNumber(tt.number)
			if result != tt.expected {
				t.Errorf("formatNumber(%d) = %v, want %v", tt.number, result, tt.expected)
			}
		})
	}
}

func TestLimitDisplay_GetStatus(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return nil initially
	if ld.GetStatus() != nil {
		t.Error("Should return nil when no status set")
	}
	
	// Should return status after update
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{Name: "Pro"},
	}
	ld.Update(status)
	
	if ld.GetStatus() != status {
		t.Error("Should return the updated status")
	}
}

func TestLimitDisplay_IsOverLimit(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return false when no status
	if ld.IsOverLimit() {
		t.Error("Should return false when no status")
	}
	
	// Test under limit
	status := &limits.LimitStatus{Percentage: 95.0}
	ld.Update(status)
	if ld.IsOverLimit() {
		t.Error("Should return false when under 100%")
	}
	
	// Test over limit
	status.Percentage = 105.0
	ld.Update(status)
	if !ld.IsOverLimit() {
		t.Error("Should return true when over 100%")
	}
	
	// Test exactly at limit
	status.Percentage = 100.0
	ld.Update(status)
	if !ld.IsOverLimit() {
		t.Error("Should return true when exactly at 100%")
	}
}

func TestLimitDisplay_IsNearLimit(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return false when no status
	if ld.IsNearLimit(90.0) {
		t.Error("Should return false when no status")
	}
	
	status := &limits.LimitStatus{Percentage: 85.0}
	ld.Update(status)
	
	// Test below threshold
	if ld.IsNearLimit(90.0) {
		t.Error("Should return false when below threshold")
	}
	
	// Test above threshold
	if !ld.IsNearLimit(80.0) {
		t.Error("Should return true when above threshold")
	}
	
	// Test exactly at threshold
	if !ld.IsNearLimit(85.0) {
		t.Error("Should return true when at threshold")
	}
}

func TestLimitDisplay_GetUsagePercentage(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return 0 when no status
	if ld.GetUsagePercentage() != 0 {
		t.Error("Should return 0 when no status")
	}
	
	status := &limits.LimitStatus{Percentage: 75.5}
	ld.Update(status)
	
	if ld.GetUsagePercentage() != 75.5 {
		t.Errorf("Expected 75.5, got %.1f", ld.GetUsagePercentage())
	}
}

func TestLimitDisplay_GetRemainingBudget(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return 0 when no status
	if ld.GetRemainingBudget() != 0 {
		t.Error("Should return 0 when no status")
	}
	
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{CostLimit: 18.0},
		CurrentUsage: limits.Usage{Cost: 12.0},
	}
	ld.Update(status)
	
	expected := 6.0
	if ld.GetRemainingBudget() != expected {
		t.Errorf("Expected remaining budget %.1f, got %.1f", expected, ld.GetRemainingBudget())
	}
	
	// Test over budget
	status.CurrentUsage.Cost = 20.0
	ld.Update(status)
	if ld.GetRemainingBudget() != 0 {
		t.Error("Should return 0 when over budget")
	}
}

func TestLimitDisplay_GetRemainingTokens(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Should return 0 when no status
	if ld.GetRemainingTokens() != 0 {
		t.Error("Should return 0 when no status")
	}
	
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{TokenLimit: 1000000},
		CurrentUsage: limits.Usage{Tokens: 600000},
	}
	ld.Update(status)
	
	expected := int64(400000)
	if ld.GetRemainingTokens() != expected {
		t.Errorf("Expected remaining tokens %d, got %d", expected, ld.GetRemainingTokens())
	}
	
	// Test over limit
	status.CurrentUsage.Tokens = 1200000
	ld.Update(status)
	if ld.GetRemainingTokens() != 0 {
		t.Error("Should return 0 when over token limit")
	}
	
	// Test with no token limit
	status.Plan.TokenLimit = 0
	ld.Update(status)
	if ld.GetRemainingTokens() != 0 {
		t.Error("Should return 0 when no token limit")
	}
}

func TestLimitDisplay_RenderQuickStatus(t *testing.T) {
	ld := NewLimitDisplay()
	
	// Test with no status
	result := ld.RenderQuickStatus()
	if !strings.Contains(result, "No limit data") {
		t.Error("Should show no data message when status is nil")
	}
	
	// Test with status
	status := &limits.LimitStatus{
		Plan: limits.SubscriptionPlan{
			Name:      "Pro",
			CostLimit: 18.0,
		},
		CurrentUsage: limits.Usage{Cost: 13.5},
		Percentage:   75.0,
	}
	ld.Update(status)
	
	result = ld.RenderQuickStatus()
	if !strings.Contains(result, "Pro") {
		t.Error("Should contain plan name")
	}
	if !strings.Contains(result, "75.0%") {
		t.Error("Should contain percentage")
	}
	if !strings.Contains(result, "$4.50") {
		t.Error("Should contain remaining budget")
	}
}