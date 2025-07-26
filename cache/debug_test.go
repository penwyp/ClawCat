package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/models"
)

func TestDebugBasicCache(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := "/tmp/debug_test_" + time.Now().Format("20060102_150405")
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Creating cache in: %s\n", tmpDir)

	config := BadgerConfig{
		DBPath:         tmpDir,
		MaxMemoryUsage: 32 * 1024 * 1024, // 32MB for test
		DefaultTTL:     time.Hour,         // Long TTL for debugging
		LogLevel:       "ERROR",
	}

	cache, err := NewBadgerCache(config)
	if err != nil {
		t.Fatalf("Failed to create BadgerDB cache: %v", err)
	}
	defer cache.Close()

	// Test simple string value
	testKey := "debug:simple"
	testValue := "hello world"

	fmt.Printf("Setting key: %s, value: %s\n", testKey, testValue)
	err = cache.Set(testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	fmt.Printf("Getting key: %s\n", testKey)
	retrievedValue, exists := cache.Get(testKey)
	if !exists {
		t.Fatalf("Key should exist but doesn't")
	}

	fmt.Printf("Retrieved value: %v (type: %T)\n", retrievedValue, retrievedValue)
	
	if retrievedValue != testValue {
		t.Fatalf("Value mismatch: expected %v, got %v", testValue, retrievedValue)
	}

	// Check cache stats
	stats := cache.GetStats()
	fmt.Printf("Cache stats: NumKeys=%d, TotalSize=%d\n", stats.NumKeys, stats.TotalSize)
}

func TestDebugEnhancedStore(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := "/tmp/debug_enhanced_" + time.Now().Format("20060102_150405")
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Creating enhanced store in: %s\n", tmpDir)

	config := EnhancedStoreConfig{
		BadgerConfig: BadgerConfig{
			DBPath:         tmpDir,
			MaxMemoryUsage: 64 * 1024 * 1024, // 64MB for test
			DefaultTTL:     time.Hour,
			LogLevel:       "ERROR",
		},
		EnableMetrics: true,
		AutoCleanup:   false,
	}

	store, err := NewEnhancedStore(config)
	if err != nil {
		t.Fatalf("Failed to create enhanced store: %v", err)
	}
	defer store.Close()

	// Create simple test data
	baseTime := time.Now().Truncate(time.Hour)
	fmt.Printf("Base time: %s\n", baseTime.Format("2006-01-02 15:04:05"))

	entries := []models.UsageEntry{
		{
			Timestamp:           baseTime.Add(-1 * time.Hour),
			Model:               "claude-3-sonnet-20240229",
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.05,
			SessionID:           "session-001",
		},
		{
			Timestamp:           baseTime.Add(-1 * time.Hour),
			Model:               "claude-3-haiku-20240307",
			InputTokens:         800,
			OutputTokens:        400,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.02,
			SessionID:           "session-002",
		},
	}

	fmt.Printf("Processing %d entries\n", len(entries))
	err = store.ProcessFile("debug_test.jsonl", entries)
	if err != nil {
		t.Fatalf("Failed to process entries: %v", err)
	}

	// Check models list
	models, err := store.GetModelsList()
	if err != nil {
		t.Fatalf("Failed to get models list: %v", err)
	}
	fmt.Printf("Models found: %v\n", models)

	// Check hourly aggregation
	hourTimestamp := baseTime.Add(-1 * time.Hour)
	fmt.Printf("Checking hourly aggregation for: %s\n", hourTimestamp.Format("2006-01-02 15:04:05"))
	
	hourlyAgg, err := store.GetHourlyAggregation(hourTimestamp)
	if err != nil {
		fmt.Printf("Hourly aggregation error: %v\n", err)
	} else {
		fmt.Printf("Hourly aggregation found: %d models, %d total entries\n", 
			len(hourlyAgg.Models), hourlyAgg.TotalStats.EntryCount)
		for model, stats := range hourlyAgg.Models {
			fmt.Printf("  %s: %d entries, $%.4f\n", model, stats.EntryCount, stats.TotalCost)
		}
	}

	// Check daily aggregation
	dayTimestamp := baseTime.Add(-1 * time.Hour).Truncate(24 * time.Hour)
	fmt.Printf("Checking daily aggregation for: %s\n", dayTimestamp.Format("2006-01-02"))
	
	dailyAgg, err := store.GetDailyAggregation(dayTimestamp)
	if err != nil {
		fmt.Printf("Daily aggregation error: %v\n", err)
	} else {
		fmt.Printf("Daily aggregation found: %d models, %d total entries\n", 
			len(dailyAgg.Models), dailyAgg.TotalStats.EntryCount)
	}

	// Check store stats
	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	fmt.Printf("Store stats: %+v\n", stats)
}