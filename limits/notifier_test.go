package limits

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/config"
)

func TestNewNotifier(t *testing.T) {
	// Test with nil config
	notifier := NewNotifier(nil)
	if notifier == nil {
		t.Fatal("Notifier should not be nil")
	}

	// Should default to desktop notifications
	if len(notifier.enabledTypes) != 1 {
		t.Errorf("Expected 1 enabled type by default, got %d", len(notifier.enabledTypes))
	}

	if notifier.enabledTypes[0] != config.NotifyDesktop {
		t.Errorf("Expected default type to be NotifyDesktop, got %v", notifier.enabledTypes[0])
	}

	// Test with config
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			Notifications: []config.NotificationType{config.NotifyDesktop, config.NotifySound, config.NotifyWebhook},
		},
	}

	notifier = NewNotifier(cfg)
	if len(notifier.enabledTypes) != 3 {
		t.Errorf("Expected 3 enabled types, got %d", len(notifier.enabledTypes))
	}
}

func TestNotifier_SendNotification(t *testing.T) {
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			Notifications: []config.NotificationType{config.NotifyDesktop},
		},
	}

	notifier := NewNotifier(cfg)

	// Test sending notification
	err := notifier.SendNotification("Test message", SeverityInfo)
	// Note: This might fail on systems without proper notification setup
	// but we test that the function doesn't panic
	_ = err // Ignore error for test purposes
}

func TestNotifier_SetEnabledTypes(t *testing.T) {
	notifier := NewNotifier(nil)

	newTypes := []config.NotificationType{config.NotifyDesktop, config.NotifySound, config.NotifyEmail}
	notifier.SetEnabledTypes(newTypes)

	if len(notifier.enabledTypes) != len(newTypes) {
		t.Errorf("Expected %d enabled types, got %d", len(newTypes), len(notifier.enabledTypes))
	}

	for i, expectedType := range newTypes {
		if notifier.enabledTypes[i] != expectedType {
			t.Errorf("Expected type %v at index %d, got %v", expectedType, i, notifier.enabledTypes[i])
		}
	}
}

func TestNotifier_IsEnabled(t *testing.T) {
	notifier := NewNotifier(nil)
	notifier.SetEnabledTypes([]config.NotificationType{config.NotifyDesktop, config.NotifySound})

	if !notifier.IsEnabled(config.NotifyDesktop) {
		t.Error("NotifyDesktop should be enabled")
	}

	if !notifier.IsEnabled(config.NotifySound) {
		t.Error("NotifySound should be enabled")
	}

	if notifier.IsEnabled(config.NotifyEmail) {
		t.Error("NotifyEmail should not be enabled")
	}

	if notifier.IsEnabled(config.NotifyWebhook) {
		t.Error("NotifyWebhook should not be enabled")
	}
}

func TestNotifier_TestNotification(t *testing.T) {
	notifier := NewNotifier(nil)

	// Test notification - might fail on some systems but shouldn't panic
	err := notifier.TestNotification()
	_ = err // Ignore error for test purposes
}

func TestNotifier_GetIconForSeverity(t *testing.T) {
	notifier := NewNotifier(nil)

	tests := []struct {
		severity     Severity
		expectedIcon string
	}{
		{SeverityInfo, "dialog-information"},
		{SeverityWarning, "dialog-warning"},
		{SeverityError, "dialog-error"},
		{SeverityCritical, "dialog-error"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			icon := notifier.getIconForSeverity(tt.severity)
			if icon != tt.expectedIcon {
				t.Errorf("Expected icon %v for severity %v, got %v", tt.expectedIcon, tt.severity, icon)
			}
		})
	}
}

func TestNotifier_CapitalizeFirst(t *testing.T) {
	notifier := NewNotifier(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "Hello"},
		{"HELLO", "HELLO"},
		{"hELLO", "HELLO"},
		{"a", "A"},
		{"123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := notifier.capitalizeFirst(tt.input)
			if result != tt.expected {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEnhancedNotifier(t *testing.T) {
	cfg := &config.Config{
		Limits: config.LimitsConfig{
			Notifications: []config.NotificationType{config.NotifyDesktop},
		},
	}

	enhancedNotifier := NewEnhancedNotifier(cfg)
	if enhancedNotifier == nil {
		t.Fatal("Enhanced notifier should not be nil")
	}

	if enhancedNotifier.Notifier == nil {
		t.Fatal("Base notifier should not be nil")
	}

	if len(enhancedNotifier.history) != 0 {
		t.Error("Initial history should be empty")
	}

	if enhancedNotifier.maxHistory != 100 {
		t.Errorf("Expected max history 100, got %d", enhancedNotifier.maxHistory)
	}
}

func TestEnhancedNotifier_SendNotificationWithHistory(t *testing.T) {
	enhancedNotifier := NewEnhancedNotifier(nil)

	// Send a notification
	err := enhancedNotifier.SendNotificationWithHistory("Test message", SeverityInfo)
	_ = err // Ignore potential system-specific errors

	// Check that history was recorded
	if len(enhancedNotifier.history) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(enhancedNotifier.history))
	}

	if len(enhancedNotifier.history) > 0 {
		record := enhancedNotifier.history[0]
		if record.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got %v", record.Message)
		}

		if record.Severity != SeverityInfo {
			t.Errorf("Expected severity %v, got %v", SeverityInfo, record.Severity)
		}

		if time.Since(record.Timestamp) > time.Second {
			t.Error("Timestamp should be recent")
		}
	}
}

func TestEnhancedNotifier_AddToHistory(t *testing.T) {
	enhancedNotifier := NewEnhancedNotifier(nil)
	enhancedNotifier.maxHistory = 3 // Set small limit for testing

	// Add notifications beyond max history
	for i := 0; i < 5; i++ {
		record := NotificationHistory{
			Message:   fmt.Sprintf("Message %d", i),
			Severity:  SeverityInfo,
			Timestamp: time.Now(),
			Success:   true,
		}
		enhancedNotifier.addToHistory(record)
	}

	// Should only keep the last 3
	if len(enhancedNotifier.history) != 3 {
		t.Errorf("Expected 3 history entries (max), got %d", len(enhancedNotifier.history))
	}

	// Should keep the most recent ones
	expectedMessages := []string{"Message 2", "Message 3", "Message 4"}
	for i, expectedMsg := range expectedMessages {
		if enhancedNotifier.history[i].Message != expectedMsg {
			t.Errorf("Expected message %v at index %d, got %v", expectedMsg, i, enhancedNotifier.history[i].Message)
		}
	}
}

func TestEnhancedNotifier_GetHistory(t *testing.T) {
	enhancedNotifier := NewEnhancedNotifier(nil)

	// Add some test history
	records := []NotificationHistory{
		{Message: "Msg 1", Severity: SeverityInfo, Timestamp: time.Now(), Success: true},
		{Message: "Msg 2", Severity: SeverityWarning, Timestamp: time.Now(), Success: false, Error: "Test error"},
	}

	for _, record := range records {
		enhancedNotifier.addToHistory(record)
	}

	history := enhancedNotifier.GetHistory()
	if len(history) != len(records) {
		t.Errorf("Expected %d history entries, got %d", len(records), len(history))
	}

	for i, expectedRecord := range records {
		if history[i].Message != expectedRecord.Message {
			t.Errorf("Expected message %v at index %d, got %v", expectedRecord.Message, i, history[i].Message)
		}
	}
}

func TestEnhancedNotifier_GetRecentFailures(t *testing.T) {
	enhancedNotifier := NewEnhancedNotifier(nil)

	// Add mixed success/failure records
	now := time.Now()
	records := []NotificationHistory{
		{Message: "Success 1", Success: true, Timestamp: now.Add(-1 * time.Hour)},
		{Message: "Failure 1", Success: false, Timestamp: now.Add(-30 * time.Minute), Error: "Error 1"},
		{Message: "Success 2", Success: true, Timestamp: now.Add(-20 * time.Minute)},
		{Message: "Failure 2", Success: false, Timestamp: now.Add(-10 * time.Minute), Error: "Error 2"},
		{Message: "Old Failure", Success: false, Timestamp: now.Add(-2 * time.Hour), Error: "Old error"},
	}

	for _, record := range records {
		enhancedNotifier.addToHistory(record)
	}

	// Get failures since 45 minutes ago
	since := now.Add(-45 * time.Minute)
	failures := enhancedNotifier.GetRecentFailures(since)

	// Should get only the 2 recent failures (within 45 minutes)
	expectedFailures := 2
	if len(failures) != expectedFailures {
		t.Errorf("Expected %d recent failures, got %d", expectedFailures, len(failures))
	}

	// Check that all returned records are failures
	for i, failure := range failures {
		if failure.Success {
			t.Errorf("Record %d should be a failure", i)
		}
		if failure.Timestamp.Before(since) {
			t.Errorf("Record %d timestamp should be after 'since' time", i)
		}
	}
}

func TestEnhancedNotifier_GetNotificationStats(t *testing.T) {
	enhancedNotifier := NewEnhancedNotifier(nil)

	// Test with no history
	stats := enhancedNotifier.GetNotificationStats()
	if stats["total"] != 0 {
		t.Errorf("Expected total 0 for empty history, got %v", stats["total"])
	}
	if stats["success_rate"] != 0.0 {
		t.Errorf("Expected success_rate 0.0 for empty history, got %v", stats["success_rate"])
	}

	// Add test data
	now := time.Now()
	records := []NotificationHistory{
		{Success: true, Severity: SeverityInfo, Timestamp: now.Add(-1 * time.Minute)},
		{Success: true, Severity: SeverityWarning, Timestamp: now.Add(-2 * time.Minute)},
		{Success: false, Severity: SeverityError, Timestamp: now.Add(-3 * time.Minute)},
		{Success: true, Severity: SeverityInfo, Timestamp: now.Add(-4 * time.Minute)},
		{Success: false, Severity: SeverityCritical, Timestamp: now.Add(-5 * time.Minute)},
	}

	for _, record := range records {
		enhancedNotifier.addToHistory(record)
	}

	stats = enhancedNotifier.GetNotificationStats()

	// Check totals
	if stats["total"] != 5 {
		t.Errorf("Expected total 5, got %v", stats["total"])
	}
	if stats["successful"] != 3 {
		t.Errorf("Expected successful 3, got %v", stats["successful"])
	}
	if stats["failed"] != 2 {
		t.Errorf("Expected failed 2, got %v", stats["failed"])
	}

	// Check success rate
	expectedSuccessRate := float64(3) / float64(5) * 100
	if stats["success_rate"] != expectedSuccessRate {
		t.Errorf("Expected success_rate %.2f, got %v", expectedSuccessRate, stats["success_rate"])
	}

	// Check severity breakdown
	bySeverity, ok := stats["by_severity"].(map[Severity]int)
	if !ok {
		t.Fatal("by_severity should be a map[Severity]int")
	}

	expectedSeverityCounts := map[Severity]int{
		SeverityInfo:     2,
		SeverityWarning:  1,
		SeverityError:    1,
		SeverityCritical: 1,
	}

	for severity, expectedCount := range expectedSeverityCounts {
		if bySeverity[severity] != expectedCount {
			t.Errorf("Expected %d notifications with severity %v, got %d",
				expectedCount, severity, bySeverity[severity])
		}
	}

	// Check last notification timestamp
	lastNotification, ok := stats["last_notification"].(time.Time)
	if !ok {
		t.Fatal("last_notification should be a time.Time")
	}

	// Should be the most recent one (first in our list)
	expectedLast := records[0].Timestamp
	if !lastNotification.Equal(expectedLast) {
		t.Errorf("Expected last notification time %v, got %v", expectedLast, lastNotification)
	}
}
