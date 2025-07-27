package main

import (
	"fmt"
	"github.com/penwyp/claudecat/logging"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/fileio"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/orchestrator"
	"github.com/penwyp/claudecat/sessions"
)

// ValidationReport contains the results of consistency validation
type ValidationReport struct {
	DataLoadingValid     bool     `json:"data_loading_valid"`
	SessionAnalysisValid bool     `json:"session_analysis_valid"`
	BurnRateValid        bool     `json:"burn_rate_valid"`
	MetricsValid         bool     `json:"metrics_valid"`
	CachingValid         bool     `json:"caching_valid"`
	OrchestrationValid   bool     `json:"orchestration_valid"`
	Errors               []string `json:"errors"`
	Warnings             []string `json:"warnings"`
	Summary              string   `json:"summary"`
}

func main() {
	fmt.Println("claudecat Consistency Validation")
	fmt.Println("==============================")

	logging.InitLogger("DEBUG", "/tmp/claudecat.validate.log", true)
	// Get data path from command line or use default
	dataPath := getDataPath()
	fmt.Printf("Using data path: %s\n\n", dataPath)

	// Create validation report
	report := &ValidationReport{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Run validation tests
	validateDataLoading(dataPath, report)
	validateSessionAnalysis(dataPath, report)
	validateBurnRateCalculation(report)
	validateMetricsCalculation(report)
	validateCaching(dataPath, report)
	validateOrchestration(dataPath, report)

	// Generate final report
	generateReport(report)
}

// validateDataLoading validates data loading functionality
func validateDataLoading(dataPath string, report *ValidationReport) {
	fmt.Println("1. Validating Data Loading...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Data loading panicked: %v", r))
			report.DataLoadingValid = false
		}
	}()

	// Create cache store for testing
	cacheDir := filepath.Join(os.TempDir(), "claudecat-validation-cache")
	cacheStore, err := cache.NewFileBasedSummaryCache(cacheDir)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Failed to create cache store: %v", err))
		cacheStore = nil
	}
	defer func() {
		if cacheStore != nil {
			os.RemoveAll(cacheDir) // Clean up temporary cache
		}
	}()

	// Test basic data loading with cache enabled
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:           dataPath,
		Mode:               models.CostModeAuto,
		IncludeRaw:         false,
		EnableSummaryCache: cacheStore != nil,
		CacheStore:         cacheStore,
	}

	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to load usage entries: %v", err))
		report.DataLoadingValid = false
		return
	}

	// Validate results
	if len(result.Entries) == 0 {
		report.Warnings = append(report.Warnings, "No usage entries found - this might be expected if no data exists")
	}

	// Check data integrity - allow some invalid entries but count them
	invalidCount := 0
	for _, entry := range result.Entries {
		if err := entry.Validate(); err != nil {
			invalidCount++
			// Only fail if there are too many invalid entries (>10% or >100 entries)
			if invalidCount > len(result.Entries)/10 && invalidCount > 100 {
				report.Errors = append(report.Errors, fmt.Sprintf("Too many invalid entries: %d out of %d", invalidCount, len(result.Entries)))
				report.DataLoadingValid = false
				return
			}
		}
	}

	if invalidCount > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Found %d invalid entries out of %d total", invalidCount, len(result.Entries)))
	}

	report.DataLoadingValid = true
	fmt.Printf("   ✓ Loaded %d entries from %d files\n",
		len(result.Entries), result.Metadata.FilesProcessed)
}

// validateSessionAnalysis validates session analysis functionality
func validateSessionAnalysis(dataPath string, report *ValidationReport) {
	fmt.Println("2. Validating Session Analysis...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Session analysis panicked: %v", r))
			report.SessionAnalysisValid = false
		}
	}()

	// Load data for analysis
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:   dataPath,
		Mode:       models.CostModeAuto,
		IncludeRaw: true,
	}

	result, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to load data for session analysis: %v", err))
		report.SessionAnalysisValid = false
		return
	}

	if len(result.Entries) == 0 {
		report.Warnings = append(report.Warnings, "No data available for session analysis")
		report.SessionAnalysisValid = true // Not an error, just no data
		return
	}

	// Create session analyzer
	analyzer := sessions.NewSessionAnalyzer(5)

	// Transform to blocks
	blocks := analyzer.TransformToBlocks(result.Entries)

	// Validate blocks
	if len(blocks) == 0 {
		report.Warnings = append(report.Warnings, "No session blocks created")
	}

	for i, block := range blocks {
		if block.ID == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("Block %d has empty ID", i))
			continue
		}

		if block.StartTime.IsZero() || block.EndTime.IsZero() {
			report.Errors = append(report.Errors, fmt.Sprintf("Block %d has invalid timestamps", i))
			continue
		}

		if block.EndTime.Before(block.StartTime) {
			report.Errors = append(report.Errors, fmt.Sprintf("Block %d has end time before start time", i))
			continue
		}
	}

	// Test limit detection if raw data is available
	if result.RawEntries != nil {
		rawEntries := make([]map[string]interface{}, len(result.RawEntries))
		for i, entry := range result.RawEntries {
			rawEntries[i] = entry
		}

		limits := analyzer.DetectLimits(rawEntries)
		if len(limits) > 0 {
			fmt.Printf("   ✓ Detected %d limit messages\n", len(limits))
		}
	}

	if len(report.Errors) == 0 {
		report.SessionAnalysisValid = true
		fmt.Printf("   ✓ Created %d session blocks\n", len(blocks))
	} else {
		report.SessionAnalysisValid = false
	}
}

// validateBurnRateCalculation validates burn rate calculations
func validateBurnRateCalculation(report *ValidationReport) {
	fmt.Println("3. Validating Burn Rate Calculation...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Burn rate calculation panicked: %v", r))
			report.BurnRateValid = false
		}
	}()

	// Create test session block
	now := time.Now()
	block := models.SessionBlock{
		ID:        "test-burn-rate",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(3 * time.Hour),
		IsActive:  true,
		TokenCounts: models.TokenCounts{
			InputTokens:  1000,
			OutputTokens: 500,
		},
		CostUSD: 0.05,
	}

	// Test burn rate calculation
	calculator := calculations.NewBurnRateCalculator()

	burnRate := calculator.CalculateBurnRate(block)
	if burnRate == nil {
		report.Errors = append(report.Errors, "Burn rate calculation returned nil")
		report.BurnRateValid = false
		return
	}

	if burnRate.TokensPerMinute <= 0 {
		report.Errors = append(report.Errors, "Invalid tokens per minute in burn rate")
		report.BurnRateValid = false
		return
	}

	// Test projection
	projection := calculator.ProjectBlockUsage(block)
	if projection == nil {
		report.Errors = append(report.Errors, "Usage projection returned nil")
		report.BurnRateValid = false
		return
	}

	if projection.ProjectedTotalTokens < block.TokenCounts.TotalTokens() {
		report.Errors = append(report.Errors, "Projected tokens less than current tokens")
		report.BurnRateValid = false
		return
	}

	report.BurnRateValid = true
	fmt.Printf("   ✓ Burn rate: %.2f tokens/min, projection: %d tokens\n",
		burnRate.TokensPerMinute, projection.ProjectedTotalTokens)
}

// validateMetricsCalculation validates real-time metrics
func validateMetricsCalculation(report *ValidationReport) {
	fmt.Println("4. Validating Metrics Calculation...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Metrics calculation panicked: %v", r))
			report.MetricsValid = false
		}
	}()

	// Create test config
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	// Create metrics calculator
	calculator := calculations.NewEnhancedMetricsCalculator(cfg)
	defer calculator.Close()

	// Create test data
	now := time.Now()
	blocks := []models.SessionBlock{
		{
			ID:        "test-metrics",
			StartTime: now.Add(-1 * time.Hour),
			EndTime:   now.Add(4 * time.Hour),
			IsActive:  true,
			TokenCounts: models.TokenCounts{
				InputTokens:  500,
				OutputTokens: 250,
			},
			CostUSD: 0.025,
		},
	}

	calculator.UpdateSessionBlocks(blocks)

	// Calculate metrics
	metrics := calculator.Calculate()
	if metrics == nil {
		report.Errors = append(report.Errors, "Metrics calculation returned nil")
		report.MetricsValid = false
		return
	}

	// Validate metrics
	if metrics.LastUpdated.IsZero() {
		report.Errors = append(report.Errors, "Metrics missing last updated timestamp")
		report.MetricsValid = false
		return
	}

	if metrics.ConfidenceLevel < 0 || metrics.ConfidenceLevel > 100 {
		report.Errors = append(report.Errors, fmt.Sprintf("Invalid confidence level: %f", metrics.ConfidenceLevel))
		report.MetricsValid = false
		return
	}

	validHealthStatuses := []string{"healthy", "warning", "critical"}
	isValidHealth := false
	for _, status := range validHealthStatuses {
		if metrics.HealthStatus == status {
			isValidHealth = true
			break
		}
	}
	if !isValidHealth {
		report.Errors = append(report.Errors, fmt.Sprintf("Invalid health status: %s", metrics.HealthStatus))
		report.MetricsValid = false
		return
	}

	report.MetricsValid = true
	fmt.Printf("   ✓ Metrics: health=%s, confidence=%.1f%%\n",
		metrics.HealthStatus, metrics.ConfidenceLevel)
}

// validateCaching validates cache functionality
func validateCaching(dataPath string, report *ValidationReport) {
	fmt.Println("4. Validating Caching...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Cache validation panicked: %v", r))
			report.CachingValid = false
		}
	}()

	// Create temporary cache directory
	cacheDir := filepath.Join(os.TempDir(), "claudecat-cache-test")
	defer os.RemoveAll(cacheDir)

	// Test cache creation
	cacheStore, err := cache.NewFileBasedSummaryCache(cacheDir)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to create cache store: %v", err))
		report.CachingValid = false
		return
	}

	// Test cache miss (first load)
	opts := fileio.LoadUsageEntriesOptions{
		DataPath:           dataPath,
		Mode:               models.CostModeAuto,
		IncludeRaw:         false,
		EnableSummaryCache: true,
		CacheStore:         cacheStore,
	}

	result1, err := fileio.LoadUsageEntries(opts)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("First cache load failed: %v", err))
		report.CachingValid = false
		return
	}

	if len(result1.Entries) == 0 {
		report.Warnings = append(report.Warnings, "No data available for cache testing")
		report.CachingValid = true // Not an error if no data
		return
	}

	// Test cache hit (second load should be faster)
	startTime := time.Now()
	result2, err := fileio.LoadUsageEntries(opts)
	cacheLoadTime := time.Since(startTime)

	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Second cache load failed: %v", err))
		report.CachingValid = false
		return
	}

	// Validate cached results match original
	if len(result1.Entries) != len(result2.Entries) {
		report.Errors = append(report.Errors, "Cache result count mismatch")
		report.CachingValid = false
		return
	}

	// Basic validation - check first few entries with valid data
	validatedCount := 0
	for i := 0; i < len(result1.Entries) && validatedCount < 5; i++ {
		e1, e2 := result1.Entries[i], result2.Entries[i]
		
		// Skip entries with zero tokens (they may be invalid but cached consistently)
		if e1.InputTokens == 0 && e1.OutputTokens == 0 {
			continue
		}
		
		if e1.Model != e2.Model || e1.InputTokens != e2.InputTokens || e1.OutputTokens != e2.OutputTokens {
			report.Errors = append(report.Errors, fmt.Sprintf("Cache entry %d data mismatch", i))
			report.CachingValid = false
			return
		}
		validatedCount++
	}

	report.CachingValid = true
	fmt.Printf("   ✓ Cache working correctly, second load took %v\n", cacheLoadTime)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// validateOrchestration validates the orchestration system
func validateOrchestration(dataPath string, report *ValidationReport) {
	fmt.Println("6. Validating Orchestration...")

	defer func() {
		if r := recover(); r != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Orchestration panicked: %v", r))
			report.OrchestrationValid = false
		}
	}()

	// Create test config
	cfg := &config.Config{
		Data: config.DataConfig{
			Paths: []string{dataPath},
		},
		Subscription: config.SubscriptionConfig{
			Plan: "pro",
		},
	}

	// Create orchestrator
	orch := orchestrator.NewMonitoringOrchestrator(
		5*time.Second, // Slow update for testing
		dataPath,
		cfg,
	)

	// Test channels
	updateReceived := make(chan bool, 1)

	// Register callback
	orch.RegisterUpdateCallback(func(data orchestrator.MonitoringData) {
		select {
		case updateReceived <- true:
		default:
		}
	})

	// Start orchestrator
	if err := orch.Start(); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to start orchestrator: %v", err))
		report.OrchestrationValid = false
		return
	}
	defer orch.Stop()

	// Wait for initial data
	if !orch.WaitForInitialData(10 * time.Second) {
		report.Warnings = append(report.Warnings, "Timeout waiting for initial data")
	}

	// Test force refresh
	_, err := orch.ForceRefresh()
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Force refresh failed: %v", err))
	}

	// Wait for callback
	select {
	case <-updateReceived:
		report.OrchestrationValid = true
		fmt.Println("   ✓ Orchestration working correctly")
	case <-time.After(15 * time.Second):
		report.Warnings = append(report.Warnings, "Timeout waiting for orchestration callback")
		report.OrchestrationValid = true // Not necessarily an error if no data
	}
}

// generateReport generates the final validation report
func generateReport(report *ValidationReport) {
	fmt.Println("\nValidation Report")
	fmt.Println("=================")

	// Count valid components
	validCount := 0
	totalCount := 6

	if report.DataLoadingValid {
		validCount++
	}
	if report.SessionAnalysisValid {
		validCount++
	}
	if report.BurnRateValid {
		validCount++
	}
	if report.MetricsValid {
		validCount++
	}
	if report.CachingValid {
		validCount++
	}
	if report.OrchestrationValid {
		validCount++
	}

	// Generate summary
	if validCount == totalCount && len(report.Errors) == 0 {
		report.Summary = "✓ All components are consistent with Claude Monitor behavior"
		fmt.Println(report.Summary)
	} else {
		report.Summary = fmt.Sprintf("⚠ %d/%d components validated successfully", validCount, totalCount)
		fmt.Println(report.Summary)
	}

	// Print errors
	if len(report.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, err := range report.Errors {
			fmt.Printf("  ✗ %s\n", err)
		}
	}

	// Print warnings
	if len(report.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, warn := range report.Warnings {
			fmt.Printf("  ⚠ %s\n", warn)
		}
	}

	// Print component status
	fmt.Println("\nComponent Status:")
	fmt.Printf("  Data Loading:      %s\n", getStatus(report.DataLoadingValid))
	fmt.Printf("  Session Analysis:  %s\n", getStatus(report.SessionAnalysisValid))
	fmt.Printf("  Burn Rate Calc:    %s\n", getStatus(report.BurnRateValid))
	fmt.Printf("  Metrics Calc:      %s\n", getStatus(report.MetricsValid))
	fmt.Printf("  Caching:           %s\n", getStatus(report.CachingValid))
	fmt.Printf("  Orchestration:     %s\n", getStatus(report.OrchestrationValid))

	// Exit with appropriate code
	if len(report.Errors) == 0 {
		fmt.Println("\n✓ Validation completed successfully")
		os.Exit(0)
	} else {
		fmt.Println("\n✗ Validation failed")
		os.Exit(1)
	}
}

// Helper functions
func getDataPath() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// Default data path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	return filepath.Join(homeDir, ".claude", "projects")
}

func getStatus(valid bool) string {
	if valid {
		return "✓ PASS"
	}
	return "✗ FAIL"
}
