package cache

import (
	"fmt"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// ExampleUsage demonstrates how to use the enhanced cache system
func ExampleUsage() error {
	// Create enhanced store configuration
	config := EnhancedStoreConfig{
		BadgerConfig: BadgerConfig{
			DBPath:         "/tmp/clawcat_cache_demo",
			MaxMemoryUsage: 128 * 1024 * 1024, // 128MB
			DefaultTTL:     24 * time.Hour,     // 24 hours
			LogLevel:       "WARNING",
		},
		EnableMetrics:   true,
		AutoCleanup:     true,
		CleanupInterval: time.Hour,
	}

	// Create enhanced store
	store, err := NewEnhancedStore(config)
	if err != nil {
		return fmt.Errorf("failed to create enhanced store: %w", err)
	}
	defer store.Close()

	// Example usage entries (simulating Claude usage data)
	entries := []models.UsageEntry{
		{
			Timestamp:           time.Now().Add(-25 * time.Hour),
			Model:               "claude-3-sonnet-20240229",
			InputTokens:         1500,
			OutputTokens:        800,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.045,
			SessionID:           "session-001",
		},
		{
			Timestamp:           time.Now().Add(-24 * time.Hour),
			Model:               "claude-3-haiku-20240307",
			InputTokens:         2000,
			OutputTokens:        1200,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.012,
			SessionID:           "session-002",
		},
		{
			Timestamp:           time.Now().Add(-23 * time.Hour),
			Model:               "claude-3-sonnet-20240229",
			InputTokens:         3000,
			OutputTokens:        2000,
			CacheCreationTokens: 500,
			CacheReadTokens:     0,
			CostUSD:             0.089,
			SessionID:           "session-001",
		},
		{
			Timestamp:           time.Now().Add(-1 * time.Hour),
			Model:               "claude-3-opus-20240229",
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 0,
			CacheReadTokens:     200,
			CostUSD:             0.078,
			SessionID:           "session-003",
		},
	}

	// Process the usage data
	fmt.Println("Processing usage entries...")
	if err := store.ProcessFile("example_usage_file.jsonl", entries); err != nil {
		return fmt.Errorf("failed to process entries: %w", err)
	}

	// Query examples
	fmt.Println("\n=== Query Examples ===")

	// 1. Get models list
	models, err := store.GetModelsList()
	if err != nil {
		return err
	}
	fmt.Printf("Available models: %v\n", models)

	// 2. Get daily aggregation for yesterday
	yesterday := time.Now().Add(-24 * time.Hour)
	dailyAgg, err := store.GetDailyAggregation(yesterday)
	if err != nil {
		fmt.Printf("No daily aggregation found for %s: %v\n", yesterday.Format("2006-01-02"), err)
	} else {
		fmt.Printf("Daily aggregation for %s:\n", dailyAgg.Date.Format("2006-01-02"))
		fmt.Printf("  Total entries: %d\n", dailyAgg.TotalStats.EntryCount)
		fmt.Printf("  Total cost: $%.4f\n", dailyAgg.TotalStats.TotalCost)
		fmt.Printf("  Total tokens: %d\n", dailyAgg.TotalStats.TotalTokens)
		fmt.Printf("  Models used: %d\n", len(dailyAgg.Models))
		for model, stats := range dailyAgg.Models {
			fmt.Printf("    %s: %d entries, $%.4f, %d tokens\n", 
				model, stats.EntryCount, stats.TotalCost, stats.TotalTokens)
		}
	}

	// 3. Get usage for specific model in last 7 days
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	modelStats, err := store.GetModelUsageInRange("claude-3-sonnet-20240229", sevenDaysAgo, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("\nClaude-3-Sonnet usage in last 7 days:\n")
	fmt.Printf("  Entries: %d\n", modelStats.EntryCount)
	fmt.Printf("  Cost: $%.4f\n", modelStats.TotalCost)
	fmt.Printf("  Input tokens: %d\n", modelStats.InputTokens)
	fmt.Printf("  Output tokens: %d\n", modelStats.OutputTokens)
	fmt.Printf("  Cache tokens: %d\n", modelStats.CacheCreationTokens+modelStats.CacheReadTokens)

	// 4. Get total usage in last 30 days
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	totalStats, err := store.GetTotalUsageInRange(thirtyDaysAgo, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("\nTotal usage in last 30 days:\n")
	fmt.Printf("  Total entries: %d\n", totalStats.EntryCount)
	fmt.Printf("  Total cost: $%.4f\n", totalStats.TotalCost)
	fmt.Printf("  Total tokens: %d\n", totalStats.TotalTokens)
	fmt.Printf("  Unique sessions: %d\n", len(totalStats.Sessions))

	// 5. Get top models by cost
	topModels, err := store.GetTopModels(3, thirtyDaysAgo, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("\nTop 3 models by cost:\n")
	for i, ranking := range topModels {
		fmt.Printf("  %d. %s: $%.4f (%d tokens, %d entries)\n", 
			i+1, ranking.Model, ranking.TotalCost, ranking.TotalTokens, ranking.EntryCount)
	}

	// 6. Get store statistics
	stats, err := store.Stats()
	if err != nil {
		return err
	}
	fmt.Printf("\nStore Statistics:\n")
	fmt.Printf("  Database size: %d bytes\n", stats.BadgerStats.TotalSize)
	fmt.Printf("  Number of keys: %d\n", stats.BadgerStats.NumKeys)
	fmt.Printf("  Files processed: %d\n", stats.ProcessorMetrics.FilesProcessed)
	fmt.Printf("  Entries processed: %d\n", stats.ProcessorMetrics.EntriesProcessed)
	fmt.Printf("  Hourly aggregations: %d\n", stats.HourlyAggregations)
	fmt.Printf("  Daily aggregations: %d\n", stats.DailyAggregations)
	fmt.Printf("  Total models: %d\n", stats.TotalModels)

	return nil
}

// BenchmarkQueries demonstrates the performance benefits of pre-aggregated data
func BenchmarkQueries(store *EnhancedStore) error {
	fmt.Println("\n=== Performance Benchmark ===")
	
	// Simulate querying monthly data (30 days)
	start := time.Now().Add(-30 * 24 * time.Hour)
	end := time.Now()
	
	// Time the daily range query
	startTime := time.Now()
	dailyAggs, err := store.GetDailyRange(start, end)
	if err != nil {
		return err
	}
	dailyQueryTime := time.Since(startTime)
	
	fmt.Printf("Daily aggregations query:\n")
	fmt.Printf("  Time: %v\n", dailyQueryTime)
	fmt.Printf("  Records returned: %d\n", len(dailyAggs))
	fmt.Printf("  Average time per record: %v\n", dailyQueryTime/time.Duration(len(dailyAggs)))
	
	// Calculate total stats from daily aggregations
	startTime = time.Now()
	totalStats := &ModelStats{}
	for _, dailyAgg := range dailyAggs {
		if dailyAgg.TotalStats != nil {
			totalStats.EntryCount += dailyAgg.TotalStats.EntryCount
			totalStats.TotalCost += dailyAgg.TotalStats.TotalCost
			totalStats.TotalTokens += dailyAgg.TotalStats.TotalTokens
		}
	}
	aggregationTime := time.Since(startTime)
	
	fmt.Printf("Secondary aggregation:\n")
	fmt.Printf("  Time: %v\n", aggregationTime)
	fmt.Printf("  Total entries aggregated: %d\n", totalStats.EntryCount)
	fmt.Printf("  Total cost: $%.4f\n", totalStats.TotalCost)
	
	fmt.Printf("Total query time: %v\n", dailyQueryTime+aggregationTime)
	
	return nil
}