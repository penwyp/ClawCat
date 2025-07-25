package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/orchestrator"
	"github.com/penwyp/ClawCat/sessions"
)

// TestClaudeMonitorConsistency tests consistency with Claude Monitor behavior
func TestClaudeMonitorConsistency(t *testing.T) {
	// Create test configuration
	cfg := createTestConfig()
	
	// Create test data
	testDataPath := createTestData(t)
	defer os.RemoveAll(testDataPath)
	
	t.Run("DataLoading", func(t *testing.T) {
		testDataLoading(t, testDataPath)
	})
	
	t.Run("SessionAnalysis", func(t *testing.T) {
		testSessionAnalysis(t, testDataPath)
	})
	
	t.Run("BurnRateCalculation", func(t *testing.T) {
		testBurnRateCalculation(t)
	})
	
	t.Run("RealtimeMetrics", func(t *testing.T) {
		testRealtimeMetrics(t, cfg)
	})
	
	t.Run("OrchestrationFlow", func(t *testing.T) {
		testOrchestrationFlow(t, testDataPath, cfg)
	})
	
	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandling(t)
	})
}

// testDataLoading validates that data loading works consistently with Claude Monitor
func testDataLoading(t *testing.T, dataPath string) {
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:   dataPath,
		HoursBack:  nil, // Load all data
		Mode:       models.CostModeAuto,
		IncludeRaw: true,
	}
	
	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		t.Fatalf("Failed to load usage entries: %v", err)
	}
	
	// Validate results
	if len(result.Entries) == 0 {
		t.Error("Expected some usage entries, got none")
	}
	
	// Check that entries are sorted by timestamp
	for i := 1; i < len(result.Entries); i++ {
		if result.Entries[i].Timestamp.Before(result.Entries[i-1].Timestamp) {
			t.Error("Entries are not sorted by timestamp")
			break
		}
	}
	
	// Validate entry data integrity
	for i, entry := range result.Entries {
		if err := entry.Validate(); err != nil {
			t.Errorf("Entry %d failed validation: %v", i, err)
		}
		
		// Check cost calculation
		if entry.CostUSD <= 0 && entry.TotalTokens > 0 {
			t.Errorf("Entry %d has tokens but no cost", i)
		}
	}
	
	t.Logf("Successfully loaded %d entries from %d files in %v",
		len(result.Entries), result.Metadata.FilesProcessed, result.Metadata.LoadDuration)
}

// testSessionAnalysis validates session block creation and analysis
func testSessionAnalysis(t *testing.T, dataPath string) {
	// Load test data
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:   dataPath,
		Mode:       models.CostModeAuto,
		IncludeRaw: true,
	}
	
	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		t.Fatalf("Failed to load test data: %v", err)
	}
	
	// Create session analyzer
	analyzer := sessions.NewSessionAnalyzer(5) // 5-hour sessions
	
	// Transform to blocks
	blocks := analyzer.TransformToBlocks(result.Entries)
	
	// Validate blocks
	if len(blocks) == 0 {
		t.Error("Expected some session blocks, got none")
	}
	
	// Check block properties
	for i, block := range blocks {
		// Validate block structure
		if block.ID == "" {
			t.Errorf("Block %d has empty ID", i)
		}
		
		if block.StartTime.IsZero() || block.EndTime.IsZero() {
			t.Errorf("Block %d has invalid timestamps", i)
		}
		
		if block.EndTime.Before(block.StartTime) {
			t.Errorf("Block %d has end time before start time", i)
		}
		
		// Validate duration (should be 5 hours for non-gap blocks)
		if !block.IsGap {
			expectedDuration := 5 * time.Hour
			actualDuration := block.EndTime.Sub(block.StartTime)
			if actualDuration != expectedDuration {
				t.Errorf("Block %d has duration %v, expected %v", i, actualDuration, expectedDuration)
			}
		}
		
		// Validate token counts consistency
		if block.TokenCounts.TotalTokens() != block.TotalTokens {
			t.Errorf("Block %d has inconsistent token counts: %d vs %d",
				i, block.TokenCounts.TotalTokens(), block.TotalTokens)
		}
		
		// Validate cost consistency
		if block.CostUSD != block.TotalCost {
			t.Errorf("Block %d has inconsistent costs: %f vs %f",
				i, block.CostUSD, block.TotalCost)
		}
	}
	
	// Test limit detection
	if result.RawEntries != nil {
		rawEntries := make([]map[string]interface{}, len(result.RawEntries))
		for i, entry := range result.RawEntries {
			rawEntries[i] = entry
		}
		
		limits := analyzer.DetectLimits(rawEntries)
		t.Logf("Detected %d limit messages", len(limits))
	}
	
	t.Logf("Successfully created %d session blocks", len(blocks))
}

// testBurnRateCalculation validates burn rate calculations
func testBurnRateCalculation(t *testing.T) {
	// Create test session block
	block := createTestSessionBlock()
	
	// Create burn rate calculator
	calculator := calculations.NewBurnRateCalculator()
	
	// Calculate burn rate
	burnRate := calculator.CalculateBurnRate(block)
	if burnRate == nil {
		t.Error("Expected burn rate calculation, got nil")
		return
	}
	
	// Validate burn rate values
	if burnRate.TokensPerMinute <= 0 {
		t.Error("Expected positive tokens per minute")
	}
	
	if burnRate.CostPerHour < 0 {
		t.Error("Expected non-negative cost per hour")
	}
	
	// Test projection
	projection := calculator.ProjectBlockUsage(block)
	if projection == nil {
		t.Error("Expected usage projection, got nil")
		return
	}
	
	// Validate projection values
	if projection.ProjectedTotalTokens < block.TokenCounts.TotalTokens() {
		t.Error("Projected tokens should be >= current tokens")
	}
	
	if projection.ProjectedTotalCost < block.CostUSD {
		t.Error("Projected cost should be >= current cost")
	}
	
	t.Logf("Burn rate: %.2f tokens/min, %.2f $/hour",
		burnRate.TokensPerMinute, burnRate.CostPerHour)
	t.Logf("Projection: %d tokens, $%.4f",
		projection.ProjectedTotalTokens, projection.ProjectedTotalCost)
}

// testRealtimeMetrics validates real-time metrics calculations
func testRealtimeMetrics(t *testing.T, cfg *config.Config) {
	// Create metrics calculator
	calculator := calculations.NewEnhancedMetricsCalculator(cfg)
	defer calculator.Close()
	
	// Create test session blocks
	blocks := []models.SessionBlock{
		createTestSessionBlock(),
		createInactiveSessionBlock(),
	}
	
	// Update calculator with test data
	calculator.UpdateSessionBlocks(blocks)
	
	// Calculate metrics
	metrics := calculator.Calculate()
	
	// Validate metrics
	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}
	
	// Check basic properties
	if metrics.LastUpdated.IsZero() {
		t.Error("Expected last updated timestamp")
	}
	
	if metrics.CalculationTime <= 0 {
		t.Error("Expected positive calculation time")
	}
	
	// Validate confidence level
	if metrics.ConfidenceLevel < 0 || metrics.ConfidenceLevel > 100 {
		t.Errorf("Invalid confidence level: %f", metrics.ConfidenceLevel)
	}
	
	// Validate health status
	validHealthStatuses := []string{"healthy", "warning", "critical"}
	isValidHealth := false
	for _, status := range validHealthStatuses {
		if metrics.HealthStatus == status {
			isValidHealth = true
			break
		}
	}
	if !isValidHealth {
		t.Errorf("Invalid health status: %s", metrics.HealthStatus)
	}
	
	t.Logf("Metrics: %d tokens, $%.4f, health: %s, confidence: %.1f%%",
		metrics.CurrentTokens, metrics.CurrentCost, metrics.HealthStatus, metrics.ConfidenceLevel)
}

// testOrchestrationFlow validates the orchestration system
func testOrchestrationFlow(t *testing.T, dataPath string, cfg *config.Config) {
	// Create orchestrator
	orch := orchestrator.NewMonitoringOrchestrator(
		1*time.Second, // Fast update for testing
		dataPath,
		cfg,
	)
	
	// Test channels for receiving updates
	updateReceived := make(chan bool, 1)
	sessionChangeReceived := make(chan bool, 1)
	
	// Register callbacks
	orch.RegisterUpdateCallback(func(data orchestrator.MonitoringData) {
		// Validate monitoring data
		if len(data.Data.Blocks) == 0 {
			t.Error("Expected some blocks in monitoring data")
		}
		
		select {
		case updateReceived <- true:
		default:
		}
	})
	
	orch.RegisterSessionCallback(func(eventType, sessionID string, sessionData interface{}) {
		select {
		case sessionChangeReceived <- true:
		default:
		}
	})
	
	// Start orchestrator
	if err := orch.Start(); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer orch.Stop()
	
	// Wait for initial data
	if !orch.WaitForInitialData(10 * time.Second) {
		t.Error("Timeout waiting for initial data")
	}
	
	// Test force refresh
	_, err := orch.ForceRefresh()
	if err != nil {
		t.Errorf("Force refresh failed: %v", err)
	}
	
	// Wait for callbacks
	select {
	case <-updateReceived:
		t.Log("Data update callback received")
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for data update callback")
	}
	
	t.Log("Orchestration flow test completed successfully")
}

// testErrorHandling validates error handling and retry mechanisms
func testErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test retry with recoverable error
	attemptCount := 0
	err := errors.RetryWithBackoff(ctx, func() error {
		attemptCount++
		if attemptCount < 3 {
			return os.ErrNotExist // Retryable error
		}
		return nil // Success on 3rd attempt
	}, "test_operation")
	
	if err != nil {
		t.Errorf("Expected retry to succeed, got: %v", err)
	}
	
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
	
	// Test retry with non-retryable error
	attemptCount = 0
	err = errors.RetryWithBackoff(ctx, func() error {
		attemptCount++
		return errors.ErrCircuitBreakerOpen // Non-retryable
	}, "test_operation")
	
	if err == nil {
		t.Error("Expected retry to fail with non-retryable error")
	}
	
	t.Log("Error handling test completed successfully")
}

// Helper functions for creating test data

func createTestConfig() *config.Config {
	return &config.Config{
		Data: config.DataConfig{
			Paths: []string{},
		},
		UI: config.UIConfig{
			RefreshRate: 1 * time.Second,
			Theme:       "dark",
			CompactMode: false,
		},
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
		Debug: config.DebugConfig{
			Enabled: true,
		},
	}
}

func createTestData(t *testing.T) string {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "clawcat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create test JSONL file
	testFile := filepath.Join(tmpDir, "test_conversation.jsonl")
	content := `{"type": "message", "timestamp": "2024-01-01T10:00:00Z", "model": "claude-3-sonnet-20240229", "usage": {"input_tokens": 100, "output_tokens": 50, "cache_creation_tokens": 0, "cache_read_tokens": 0}}
{"type": "message", "timestamp": "2024-01-01T10:05:00Z", "model": "claude-3-sonnet-20240229", "usage": {"input_tokens": 150, "output_tokens": 75, "cache_creation_tokens": 10, "cache_read_tokens": 5}}
{"type": "message", "timestamp": "2024-01-01T15:30:00Z", "model": "claude-3-opus-20240229", "usage": {"input_tokens": 200, "output_tokens": 100, "cache_creation_tokens": 0, "cache_read_tokens": 20}}
`
	
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	return tmpDir
}

func createTestSessionBlock() models.SessionBlock {
	now := time.Now()
	return models.SessionBlock{
		ID:        "test-session-1",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(3 * time.Hour),
		IsActive:  true,
		IsGap:     false,
		TokenCounts: models.TokenCounts{
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 50,
			CacheReadTokens:     25,
		},
		CostUSD:           0.05,
		SentMessagesCount: 10,
		Models:            []string{"claude-3-sonnet-20240229"},
		Entries: []models.UsageEntry{
			{
				Timestamp:   now.Add(-1 * time.Hour),
				Model:       "claude-3-sonnet-20240229",
				InputTokens: 500,
				OutputTokens: 250,
				TotalTokens: 750,
				CostUSD:     0.025,
			},
		},
		// Legacy compatibility
		TotalTokens: 1575,
		TotalCost:   0.05,
	}
}

func createInactiveSessionBlock() models.SessionBlock {
	past := time.Now().Add(-6 * time.Hour)
	return models.SessionBlock{
		ID:        "test-session-2",
		StartTime: past.Add(-5 * time.Hour),
		EndTime:   past,
		IsActive:  false,
		IsGap:     false,
		TokenCounts: models.TokenCounts{
			InputTokens:  500,
			OutputTokens: 250,
		},
		CostUSD:     0.025,
		Models:      []string{"claude-3-haiku-20240307"},
		// Legacy compatibility
		TotalTokens: 750,
		TotalCost:   0.025,
	}
}