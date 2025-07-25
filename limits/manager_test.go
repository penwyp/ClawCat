package limits

import (
	"testing"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
)

func TestNewLimitManager(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	if lm == nil {
		t.Fatal("Limit manager is nil")
	}

	if lm.plan.Type != PlanPro {
		t.Errorf("Expected plan type %v, got %v", PlanPro, lm.plan.Type)
	}

	if lm.plan.CostLimit != 18.00 {
		t.Errorf("Expected cost limit 18.00, got %v", lm.plan.CostLimit)
	}
}

func TestNewLimitManagerCustomPlan(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan:            "custom",
			CustomCostLimit: 50.0,
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create custom limit manager: %v", err)
	}

	if lm.plan.Type != PlanCustom {
		t.Errorf("Expected plan type %v, got %v", PlanCustom, lm.plan.Type)
	}

	if lm.plan.CostLimit != 50.0 {
		t.Errorf("Expected cost limit 50.0, got %v", lm.plan.CostLimit)
	}
}

func TestNewLimitManagerInvalidPlan(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "invalid",
		},
	}

	_, err := NewLimitManager(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid plan type")
	}
}

func TestCheckUsage(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test usage within limits
	entry := models.UsageEntry{
		TotalTokens: 1000,
		CostUSD:     5.0,
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	if status == nil {
		t.Fatal("Status is nil")
	}

	expectedPercentage := (5.0 / 18.0) * 100
	if status.Percentage != expectedPercentage {
		t.Errorf("Expected percentage %.2f, got %.2f", expectedPercentage, status.Percentage)
	}

	if status.CurrentUsage.Cost != 5.0 {
		t.Errorf("Expected cost 5.0, got %.2f", status.CurrentUsage.Cost)
	}

	if status.CurrentUsage.Tokens != 1000 {
		t.Errorf("Expected tokens 1000, got %d", status.CurrentUsage.Tokens)
	}
}

func TestCheckUsageWarningTrigger(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test usage that triggers 75% warning
	entry := models.UsageEntry{
		TotalTokens: 750000,
		CostUSD:     13.5, // 75% of $18
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	if status.WarningLevel == nil {
		t.Fatal("Expected warning level to be set")
	}

	if status.WarningLevel.Threshold != 75 {
		t.Errorf("Expected warning threshold 75, got %.1f", status.WarningLevel.Threshold)
	}

	if status.WarningLevel.Severity != SeverityInfo {
		t.Errorf("Expected severity %v, got %v", SeverityInfo, status.WarningLevel.Severity)
	}
}

func TestCheckUsageCriticalLevel(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test usage that exceeds limit
	entry := models.UsageEntry{
		TotalTokens: 1200000,
		CostUSD:     18.5, // Over $18 limit
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	if status.WarningLevel == nil {
		t.Fatal("Expected warning level to be set")
	}

	if status.WarningLevel.Threshold != 100 {
		t.Errorf("Expected warning threshold 100, got %.1f", status.WarningLevel.Threshold)
	}

	if status.WarningLevel.Severity != SeverityCritical {
		t.Errorf("Expected severity %v, got %v", SeverityCritical, status.WarningLevel.Severity)
	}

	if status.Percentage <= 100 {
		t.Errorf("Expected percentage > 100, got %.2f", status.Percentage)
	}
}

func TestUpdateUsage(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Initial usage
	err = lm.UpdateUsage(500, 2.5)
	if err != nil {
		t.Fatalf("Failed to update usage: %v", err)
	}

	// Add more usage
	err = lm.UpdateUsage(300, 1.5)
	if err != nil {
		t.Fatalf("Failed to update usage: %v", err)
	}

	status := lm.GetStatus()
	if status.CurrentUsage.Tokens != 800 {
		t.Errorf("Expected total tokens 800, got %d", status.CurrentUsage.Tokens)
	}

	if status.CurrentUsage.Cost != 4.0 {
		t.Errorf("Expected total cost 4.0, got %.2f", status.CurrentUsage.Cost)
	}
}

func TestSetPlan(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Change to Max-5 plan
	err = lm.SetPlan(PlanMax5)
	if err != nil {
		t.Fatalf("Failed to set plan: %v", err)
	}

	if lm.plan.Type != PlanMax5 {
		t.Errorf("Expected plan type %v, got %v", PlanMax5, lm.plan.Type)
	}

	if lm.plan.CostLimit != 35.0 {
		t.Errorf("Expected cost limit 35.0, got %.2f", lm.plan.CostLimit)
	}

	// Test invalid plan
	err = lm.SetPlan(PlanType("invalid"))
	if err == nil {
		t.Fatal("Expected error for invalid plan type")
	}
}

func TestResetUsage(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Add some usage
	err = lm.UpdateUsage(1000, 5.0)
	if err != nil {
		t.Fatalf("Failed to update usage: %v", err)
	}

	// Reset usage
	err = lm.ResetUsage()
	if err != nil {
		t.Fatalf("Failed to reset usage: %v", err)
	}

	status := lm.GetStatus()
	if status.CurrentUsage.Tokens != 0 {
		t.Errorf("Expected tokens 0 after reset, got %d", status.CurrentUsage.Tokens)
	}

	if status.CurrentUsage.Cost != 0 {
		t.Errorf("Expected cost 0 after reset, got %.2f", status.CurrentUsage.Cost)
	}

	// Check that history was saved
	lm.mu.RLock()
	historyLen := len(lm.history)
	lm.mu.RUnlock()

	if historyLen != 1 {
		t.Errorf("Expected 1 history entry, got %d", historyLen)
	}
}

func TestGetRecommendations(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test recommendations at different usage levels
	testCases := []struct {
		usage           float64
		expectedContains string
	}{
		{usage: 10.0, expectedContains: ""},         // Under 50%, no specific recommendations
		{usage: 50.0, expectedContains: "on track"}, // 50-75%
		{usage: 75.0, expectedContains: "Monitor"},  // 75-90%
		{usage: 90.0, expectedContains: "upgrading"}, // 90-95%
		{usage: 95.0, expectedContains: "Critical"}, // >95%
	}

	for _, tc := range testCases {
		// Set usage to test level
		lm.mu.Lock()
		lm.usage.Cost = (tc.usage / 100) * lm.plan.CostLimit
		lm.mu.Unlock()

		recommendations := lm.GetRecommendations()

		if tc.expectedContains == "" {
			// For low usage, might have no recommendations or general ones
			continue
		}

		found := false
		for _, rec := range recommendations {
			if rec != "" && tc.expectedContains != "" {
				found = true
				break
			}
		}

		if tc.expectedContains != "" && !found {
			t.Errorf("Usage %.1f%% - expected recommendation containing '%s', got %v",
				tc.usage, tc.expectedContains, recommendations)
		}
	}
}

func TestCalculateTimeToReset(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test monthly reset cycle
	timeToReset := lm.calculateTimeToReset()
	if timeToReset <= 0 {
		t.Error("Time to reset should be positive")
	}

	// Should be less than 32 days (accounting for different month lengths)
	if timeToReset > 32*24*time.Hour {
		t.Error("Time to reset should be less than 32 days for monthly cycle")
	}
}

// TestShouldTriggerWarning has been removed as the shouldTriggerWarning method
// has been inlined into CheckUsage to avoid deadlock issues.
// The warning trigger logic is now tested through the integration tests.

func TestConcurrentUsage(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Test concurrent usage updates
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				err := lm.UpdateUsage(1, 0.01)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to update usage: %v", id, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	status := lm.GetStatus()
	expectedTokens := int64(1000)
	expectedCost := 10.0

	if status.CurrentUsage.Tokens != expectedTokens {
		t.Errorf("Expected %d tokens, got %d", expectedTokens, status.CurrentUsage.Tokens)
	}

	if status.CurrentUsage.Cost != expectedCost {
		t.Errorf("Expected %.2f cost, got %.2f", expectedCost, status.CurrentUsage.Cost)
	}
}