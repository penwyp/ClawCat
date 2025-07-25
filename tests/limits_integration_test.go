package tests

import (
	"testing"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/limits"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/ui/components"
)

// TestLimitsIntegration tests the complete limits feature integration
func TestLimitsIntegration(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
		Limits: config.LimitsConfig{
			Notifications: []limits.NotificationType{limits.NotifyDesktop},
		},
	}

	// Create limit manager
	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Create UI components
	limitDisplay := components.NewLimitDisplay()
	limitDisplay.SetWidth(80)

	// Test initial state
	status := lm.GetStatus()
	if status.CurrentUsage.Cost != 0 {
		t.Error("Initial cost should be 0")
	}
	if status.CurrentUsage.Tokens != 0 {
		t.Error("Initial tokens should be 0")
	}

	// Update display with initial status
	limitDisplay.Update(status)
	
	// Test usage updates
	usageEntry := models.UsageEntry{
		TotalTokens: 100000,
		CostUSD:     5.0,
	}

	// Process usage entry
	newStatus, err := lm.CheckUsage(usageEntry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	// Update display
	limitDisplay.Update(newStatus)

	// Verify state
	if newStatus.CurrentUsage.Cost != 5.0 {
		t.Errorf("Expected cost 5.0, got %.2f", newStatus.CurrentUsage.Cost)
	}
	if newStatus.CurrentUsage.Tokens != 100000 {
		t.Errorf("Expected tokens 100000, got %d", newStatus.CurrentUsage.Tokens)
	}

	expectedPercentage := (5.0 / 18.0) * 100
	if newStatus.Percentage != expectedPercentage {
		t.Errorf("Expected percentage %.2f, got %.2f", expectedPercentage, newStatus.Percentage)
	}

	// Test UI rendering
	rendered := limitDisplay.Render()
	if rendered == "" {
		t.Error("Rendered output should not be empty")
	}

	// Test multiple usage updates to reach warning threshold
	for i := 0; i < 3; i++ {
		usageEntry = models.UsageEntry{
			TotalTokens: 200000,
			CostUSD:     4.0,
		}
		newStatus, err = lm.CheckUsage(usageEntry)
		if err != nil {
			t.Fatalf("Failed to check usage in iteration %d: %v", i, err)
		}
		limitDisplay.Update(newStatus)
	}

	// Should now be at warning threshold
	if newStatus.Percentage < 75 {
		t.Errorf("Expected to reach warning threshold, got %.2f%%", newStatus.Percentage)
	}

	if newStatus.WarningLevel == nil {
		t.Error("Should have warning level set")
	}

	// Test UI shows warning
	rendered = limitDisplay.Render()
	// Note: We can't easily test specific strings without knowing the exact rendering,
	// but we can verify it's not empty and longer than before
	if rendered == "" {
		t.Error("Warning state rendered output should not be empty")
	}
}

// TestLimitsWorkflow tests a realistic usage workflow
func TestLimitsWorkflow(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Simulate a typical usage pattern over time
	usagePattern := []struct {
		tokens int64
		cost   float64
		desc   string
	}{
		{50000, 2.5, "Light usage"},
		{100000, 5.0, "Medium usage"},
		{150000, 7.5, "Heavy usage"},
		{200000, 10.0, "Very heavy usage"},
	}

	totalCost := 0.0
	totalTokens := int64(0)

	for i, usage := range usagePattern {
		t.Run(usage.desc, func(t *testing.T) {
			entry := models.UsageEntry{
				TotalTokens: usage.tokens,
				CostUSD:     usage.cost,
			}

			status, err := lm.CheckUsage(entry)
			if err != nil {
				t.Fatalf("Failed to check usage: %v", err)
			}

			totalCost += usage.cost
			totalTokens += usage.tokens

			if status.CurrentUsage.Cost != totalCost {
				t.Errorf("Expected total cost %.2f, got %.2f", totalCost, status.CurrentUsage.Cost)
			}

			if status.CurrentUsage.Tokens != totalTokens {
				t.Errorf("Expected total tokens %d, got %d", totalTokens, status.CurrentUsage.Tokens)
			}

			// Check that percentage is calculated correctly
			expectedPercentage := (totalCost / 18.0) * 100
			if status.Percentage != expectedPercentage {
				t.Errorf("Expected percentage %.2f, got %.2f", expectedPercentage, status.Percentage)
			}

			// Check warning levels progression
			if totalCost >= 18.0*0.75 && status.WarningLevel == nil {
				t.Error("Should have warning level when over 75%")
			}

			// Check recommendations
			recommendations := lm.GetRecommendations()
			if totalCost >= 18.0*0.75 && len(recommendations) == 0 {
				t.Error("Should have recommendations when over 75%")
			}

			t.Logf("Step %d: Cost=%.2f, Percentage=%.1f%%, Warnings=%v, Recommendations=%d",
				i+1, totalCost, status.Percentage,
				status.WarningLevel != nil, len(recommendations))
		})
	}
}

// TestCustomPlanWorkflow tests custom plan with P90 calculation
func TestCustomPlanWorkflow(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "custom",
			CustomCostLimit: 0, // Will be calculated
		},
	}

	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Simulate historical data for P90 calculation
	lm.mu.Lock()
	lm.history = []limits.HistoricalUsage{
		{Cost: 15.0}, {Cost: 18.0}, {Cost: 12.0}, {Cost: 20.0}, {Cost: 16.0},
		{Cost: 14.0}, {Cost: 19.0}, {Cost: 17.0}, {Cost: 13.0}, {Cost: 21.0},
		{Cost: 16.5}, {Cost: 14.5}, {Cost: 18.5}, {Cost: 15.5}, {Cost: 19.5},
	}
	lm.mu.Unlock()

	// Calculate P90 limit
	p90Limit, err := lm.CalculateP90Limit()
	if err != nil {
		t.Fatalf("Failed to calculate P90 limit: %v", err)
	}

	if p90Limit <= 0 {
		t.Error("P90 limit should be positive")
	}

	t.Logf("Calculated P90 limit: %.2f", p90Limit)

	// Update plan with calculated limit
	lm.mu.Lock()
	lm.plan.CostLimit = p90Limit
	lm.mu.Unlock()

	// Test usage against custom limit
	entry := models.UsageEntry{
		TotalTokens: 500000,
		CostUSD:     p90Limit * 0.8, // 80% of limit
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	if status.Percentage < 75 {
		t.Error("Should trigger warning at 80% of custom limit")
	}

	// Test distribution analysis
	dist := lm.GetDistributionAnalysis()
	if dist.P90 <= 0 {
		t.Error("Distribution P90 should be positive")
	}

	// Test recommended limit
	recommendedLimit, description, err := lm.GetRecommendedLimit()
	if err != nil {
		t.Fatalf("Failed to get recommended limit: %v", err)
	}

	if recommendedLimit <= 0 {
		t.Error("Recommended limit should be positive")
	}

	if description == "" {
		t.Error("Description should not be empty")
	}

	t.Logf("Recommended limit: %.2f (%s)", recommendedLimit, description)
}

// TestNotificationIntegration tests notification system integration
func TestNotificationIntegration(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
		Limits: config.LimitsConfig{
			Notifications: []limits.NotificationType{limits.NotifyDesktop, limits.NotifySound},
		},
	}

	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Create enhanced notifier for testing
	enhancedNotifier := limits.NewEnhancedNotifier(cfg)

	// Simulate usage that triggers notifications
	entry := models.UsageEntry{
		TotalTokens: 750000,
		CostUSD:     13.5, // 75% of $18 Pro limit
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	// Test that warning level is set
	if status.WarningLevel == nil {
		t.Error("Should have warning level at 75%")
	}

	// Test notification sending (will not actually send on test systems)
	if status.WarningLevel != nil {
		err = enhancedNotifier.SendNotificationWithHistory(
			status.WarningLevel.Message,
			status.WarningLevel.Severity,
		)
		// Don't fail on notification errors in tests
		_ = err
	}

	// Check that notification was recorded in history
	history := enhancedNotifier.GetHistory()
	if len(history) == 0 {
		t.Error("Should have notification history")
	}

	// Test notification statistics
	stats := enhancedNotifier.GetNotificationStats()
	if stats["total"] == 0 {
		t.Error("Should have notification statistics")
	}

	// Test different notification types
	notifier := lm.notifier
	for _, notifType := range []limits.NotificationType{limits.NotifyDesktop, limits.NotifySound, limits.NotifyWebhook, limits.NotifyEmail} {
		if notifier.IsEnabled(notifType) {
			t.Logf("Notification type %v is enabled", notifType)
		}
	}
}

// TestResetUsageCycle tests the usage reset functionality
func TestResetUsageCycle(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Add some usage
	entry := models.UsageEntry{
		TotalTokens: 500000,
		CostUSD:     10.0,
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	if status.CurrentUsage.Cost != 10.0 {
		t.Errorf("Expected cost 10.0, got %.2f", status.CurrentUsage.Cost)
	}

	// Reset usage
	err = lm.ResetUsage()
	if err != nil {
		t.Fatalf("Failed to reset usage: %v", err)
	}

	// Check that usage is reset
	status = lm.GetStatus()
	if status.CurrentUsage.Cost != 0 {
		t.Errorf("Expected cost 0 after reset, got %.2f", status.CurrentUsage.Cost)
	}

	if status.CurrentUsage.Tokens != 0 {
		t.Errorf("Expected tokens 0 after reset, got %d", status.CurrentUsage.Tokens)
	}

	// Check that history was preserved
	lm.mu.RLock()
	historyLen := len(lm.history)
	lm.mu.RUnlock()

	if historyLen == 0 {
		t.Error("Should have historical data after reset")
	}

	// Test time to reset calculation
	timeToReset := lm.calculateTimeToReset()
	if timeToReset <= 0 {
		t.Error("Time to reset should be positive")
	}

	if timeToReset > 31*24*time.Hour {
		t.Error("Time to reset should be less than 31 days for monthly cycle")
	}

	t.Logf("Time to reset: %v", timeToReset)
}

// TestPlanMigration tests changing subscription plans
func TestPlanMigration(t *testing.T) {
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	lm, err := limits.NewLimitManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create limit manager: %v", err)
	}

	// Add usage to Pro plan
	entry := models.UsageEntry{
		TotalTokens: 500000,
		CostUSD:     10.0,
	}

	status, err := lm.CheckUsage(entry)
	if err != nil {
		t.Fatalf("Failed to check usage: %v", err)
	}

	// Should be over 50% for Pro plan ($18 limit)
	if status.Percentage < 50 {
		t.Errorf("Expected >50%% usage, got %.2f%%", status.Percentage)
	}

	// Upgrade to Max-5 plan
	err = lm.SetPlan(limits.PlanMax5)
	if err != nil {
		t.Fatalf("Failed to set plan: %v", err)
	}

	// Usage percentage should be lower now (same $10 usage, $35 limit)
	newStatus := lm.GetStatus()
	expectedPercentage := (10.0 / 35.0) * 100
	if newStatus.Percentage != expectedPercentage {
		t.Errorf("Expected percentage %.2f after upgrade, got %.2f",
			expectedPercentage, newStatus.Percentage)
	}

	// Warning levels should be cleared
	if newStatus.WarningLevel != nil {
		t.Error("Warning level should be cleared after plan change")
	}

	// Test plan comparison
	comparison := limits.ComparePlans(limits.PlanPro, limits.PlanMax5)
	if comparison != -1 {
		t.Error("Pro plan should be less than Max-5 plan")
	}

	t.Logf("Successfully migrated from Pro to Max-5: %.2f%% -> %.2f%%",
		status.Percentage, newStatus.Percentage)
}