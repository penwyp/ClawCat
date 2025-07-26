package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedStoreIntegration(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := "/tmp/clawcat_test_" + time.Now().Format("20060102_150405")
	defer os.RemoveAll(tmpDir)

	// Create enhanced store
	config := EnhancedStoreConfig{
		BadgerConfig: BadgerConfig{
			DBPath:         tmpDir,
			MaxMemoryUsage: 64 * 1024 * 1024, // 64MB for test
			DefaultTTL:     time.Hour,
			LogLevel:       "ERROR", // Only errors during test
		},
		EnableMetrics: true,
		AutoCleanup:   false, // Disable for test
	}

	store, err := NewEnhancedStore(config)
	require.NoError(t, err, "Failed to create enhanced store")
	defer store.Close()

	// Test data - simulate usage entries from different time periods
	baseTime := time.Now().Truncate(time.Hour)
	entries := []models.UsageEntry{
		{
			Timestamp:           baseTime.Add(-25 * time.Hour), // Yesterday
			Model:               "claude-3-sonnet-20240229",
			InputTokens:         1500,
			OutputTokens:        800,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.045,
			SessionID:           "session-001",
		},
		{
			Timestamp:           baseTime.Add(-24 * time.Hour), // Yesterday
			Model:               "claude-3-haiku-20240307",
			InputTokens:         2000,
			OutputTokens:        1200,
			CacheCreationTokens: 0,
			CacheReadTokens:     0,
			CostUSD:             0.012,
			SessionID:           "session-002",
		},
		{
			Timestamp:           baseTime.Add(-23 * time.Hour), // Yesterday
			Model:               "claude-3-sonnet-20240229",
			InputTokens:         3000,
			OutputTokens:        2000,
			CacheCreationTokens: 500,
			CacheReadTokens:     0,
			CostUSD:             0.089,
			SessionID:           "session-001",
		},
		{
			Timestamp:           baseTime.Add(-1 * time.Hour), // Today
			Model:               "claude-3-opus-20240229",
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 0,
			CacheReadTokens:     200,
			CostUSD:             0.078,
			SessionID:           "session-003",
		},
	}

	// Test processing file
	err = store.ProcessFile("test_usage.jsonl", entries)
	require.NoError(t, err, "Failed to process usage entries")

	// Test getting models list
	models, err := store.GetModelsList()
	require.NoError(t, err, "Failed to get models list")
	assert.Len(t, models, 3, "Expected 3 unique models")

	expectedModels := map[string]bool{
		"claude-3-sonnet-20240229": true,
		"claude-3-haiku-20240307":  true,
		"claude-3-opus-20240229":   true,
	}
	for _, model := range models {
		assert.True(t, expectedModels[model], "Unexpected model: %s", model)
	}

	// Test getting daily aggregation
	yesterday := baseTime.Add(-24 * time.Hour).Truncate(24 * time.Hour)
	dailyAgg, err := store.GetDailyAggregation(yesterday)
	require.NoError(t, err, "Failed to get daily aggregation")

	assert.Equal(t, 3, dailyAgg.TotalStats.EntryCount, "Expected 3 entries for yesterday")
	assert.InDelta(t, 0.146, dailyAgg.TotalStats.TotalCost, 0.001, "Total cost mismatch")
	assert.Equal(t, 2, len(dailyAgg.Models), "Expected 2 models for yesterday")

	// Verify model-specific stats
	sonnetStats := dailyAgg.Models["claude-3-sonnet-20240229"]
	require.NotNil(t, sonnetStats, "Expected Sonnet stats")
	assert.Equal(t, 2, sonnetStats.EntryCount, "Expected 2 Sonnet entries")
	assert.InDelta(t, 0.134, sonnetStats.TotalCost, 0.001, "Sonnet cost mismatch")

	haikuStats := dailyAgg.Models["claude-3-haiku-20240307"]
	require.NotNil(t, haikuStats, "Expected Haiku stats")
	assert.Equal(t, 1, haikuStats.EntryCount, "Expected 1 Haiku entry")
	assert.InDelta(t, 0.012, haikuStats.TotalCost, 0.001, "Haiku cost mismatch")

	// Test getting today's data
	today := baseTime.Truncate(24 * time.Hour)
	todayAgg, err := store.GetDailyAggregation(today)
	require.NoError(t, err, "Failed to get today's aggregation")
	assert.Equal(t, 1, todayAgg.TotalStats.EntryCount, "Expected 1 entry for today")

	// Test date range query
	startDate := yesterday
	endDate := today
	dailyRange, err := store.GetDailyRange(startDate, endDate)
	require.NoError(t, err, "Failed to get daily range")
	assert.Len(t, dailyRange, 2, "Expected 2 days of data")

	// Test model usage in range
	modelUsage, err := store.GetModelUsageInRange("claude-3-sonnet-20240229", startDate, endDate.Add(24*time.Hour))
	require.NoError(t, err, "Failed to get model usage in range")
	assert.Equal(t, 2, modelUsage.EntryCount, "Expected 2 Sonnet entries in range")
	assert.InDelta(t, 0.134, modelUsage.TotalCost, 0.001, "Sonnet range cost mismatch")

	// Test total usage in range
	totalUsage, err := store.GetTotalUsageInRange(startDate, endDate.Add(24*time.Hour))
	require.NoError(t, err, "Failed to get total usage in range")
	assert.Equal(t, 4, totalUsage.EntryCount, "Expected 4 total entries in range")
	assert.InDelta(t, 0.224, totalUsage.TotalCost, 0.001, "Total range cost mismatch")

	// Test model comparison
	modelsToCompare := []string{"claude-3-sonnet-20240229", "claude-3-haiku-20240307", "claude-3-opus-20240229"}
	comparison, err := store.GetModelComparison(modelsToCompare, startDate, endDate.Add(24*time.Hour))
	require.NoError(t, err, "Failed to get model comparison")
	assert.Len(t, comparison, 3, "Expected comparison for 3 models")

	// Test top models
	topModels, err := store.GetTopModels(3, startDate, endDate.Add(24*time.Hour))
	require.NoError(t, err, "Failed to get top models")
	assert.Len(t, topModels, 3, "Expected top 3 models")
	
	// Verify ranking (Sonnet should be #1 by cost)
	assert.Equal(t, "claude-3-sonnet-20240229", topModels[0].Model, "Expected Sonnet to be top model")
	assert.InDelta(t, 0.134, topModels[0].TotalCost, 0.001, "Top model cost mismatch")

	// Test store statistics
	stats, err := store.Stats()
	require.NoError(t, err, "Failed to get store stats")
	assert.Equal(t, 3, stats.TotalModels, "Expected 3 models in stats")
	assert.Equal(t, int64(4), stats.ProcessorMetrics.EntriesProcessed, "Expected 4 processed entries")
	assert.Greater(t, stats.BadgerStats.NumKeys, int64(0), "Expected some keys in database")

	// Test store health
	assert.True(t, store.IsHealthy(), "Store should be healthy")
}

func TestBadgerCacheBasicOperations(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := "/tmp/badger_test_" + time.Now().Format("20060102_150405")
	defer os.RemoveAll(tmpDir)

	config := BadgerConfig{
		DBPath:         tmpDir,
		MaxMemoryUsage: 32 * 1024 * 1024, // 32MB for test
		DefaultTTL:     time.Minute,
		LogLevel:       "ERROR",
	}

	cache, err := NewBadgerCache(config)
	require.NoError(t, err, "Failed to create BadgerDB cache")
	defer cache.Close()

	// Test basic set/get operations
	testKey := "test:key:1"
	testValue := map[string]interface{}{
		"name":  "test",
		"value": 42,
		"time":  time.Now().Unix(),
	}

	err = cache.Set(testKey, testValue)
	require.NoError(t, err, "Failed to set value")

	retrievedValue, exists := cache.Get(testKey)
	require.True(t, exists, "Key should exist")
	
	// Convert back to map for comparison
	valueMap, ok := retrievedValue.(map[string]interface{})
	require.True(t, ok, "Retrieved value should be a map")
	assert.Equal(t, "test", valueMap["name"], "Name field mismatch")
	assert.Equal(t, float64(42), valueMap["value"], "Value field mismatch") // JSON numbers are float64

	// Test TTL functionality
	shortTTLKey := "test:ttl:key"
	err = cache.SetWithTTL(shortTTLKey, "temporary", 100*time.Millisecond)
	require.NoError(t, err, "Failed to set value with TTL")

	// Should exist immediately
	_, exists = cache.Get(shortTTLKey)
	assert.True(t, exists, "Key should exist immediately after set")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)
	_, exists = cache.Get(shortTTLKey)
	assert.False(t, exists, "Key should not exist after TTL expiry")

	// Test delete operation
	err = cache.Delete(testKey)
	require.NoError(t, err, "Failed to delete key")

	_, exists = cache.Get(testKey)
	assert.False(t, exists, "Key should not exist after deletion")

	// Test GetByPrefix
	prefixTests := map[string]string{
		"prefix:test:1": "value1",
		"prefix:test:2": "value2",
		"prefix:demo:3": "value3",
		"other:test:4":  "value4",
	}

	for key, value := range prefixTests {
		err = cache.Set(key, value)
		require.NoError(t, err, "Failed to set prefix test value")
	}

	prefixResults, err := cache.GetByPrefix("prefix:test:")
	require.NoError(t, err, "Failed to get by prefix")
	assert.Len(t, prefixResults, 2, "Expected 2 results for prefix 'prefix:test:'")

	// Test cache statistics
	stats := cache.GetStats()
	assert.Greater(t, stats.NumKeys, int64(0), "Should have some keys")
	assert.Greater(t, stats.TotalSize, int64(0), "Should have non-zero size")
}

func BenchmarkEnhancedStore(b *testing.B) {
	// Create temporary directory for benchmark database
	tmpDir := "/tmp/clawcat_bench_" + time.Now().Format("20060102_150405")
	defer os.RemoveAll(tmpDir)

	config := EnhancedStoreConfig{
		BadgerConfig: BadgerConfig{
			DBPath:         tmpDir,
			MaxMemoryUsage: 128 * 1024 * 1024, // 128MB
			DefaultTTL:     time.Hour,
			LogLevel:       "ERROR",
		},
		EnableMetrics: false, // Disable for benchmark
		AutoCleanup:   false,
	}

	store, err := NewEnhancedStore(config)
	require.NoError(b, err, "Failed to create enhanced store")
	defer store.Close()

	// Generate test data
	baseTime := time.Now().Truncate(time.Hour)
	entries := make([]models.UsageEntry, 1000)
	modelNames := []string{"claude-3-sonnet-20240229", "claude-3-haiku-20240307", "claude-3-opus-20240229"}

	for i := 0; i < 1000; i++ {
		entries[i] = models.UsageEntry{
			Timestamp:           baseTime.Add(-time.Duration(i) * time.Hour),
			Model:               modelNames[i%len(modelNames)],
			InputTokens:         1000 + i,
			OutputTokens:        500 + i/2,
			CacheCreationTokens: i / 10,
			CacheReadTokens:     i / 20,
			CostUSD:             float64(i) * 0.001,
			SessionID:           "session-" + fmt.Sprintf("%03d", i%10),
		}
	}

	// Process data once
	err = store.ProcessFile("benchmark_data.jsonl", entries)
	require.NoError(b, err, "Failed to process benchmark data")

	b.ResetTimer()

	// Benchmark daily range queries
	b.Run("DailyRangeQuery", func(b *testing.B) {
		start := baseTime.Add(-30 * 24 * time.Hour)
		end := baseTime
		
		for i := 0; i < b.N; i++ {
			_, err := store.GetDailyRange(start, end)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark model usage queries
	b.Run("ModelUsageQuery", func(b *testing.B) {
		start := baseTime.Add(-7 * 24 * time.Hour)
		end := baseTime
		
		for i := 0; i < b.N; i++ {
			_, err := store.GetModelUsageInRange("claude-3-sonnet-20240229", start, end)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Benchmark top models query
	b.Run("TopModelsQuery", func(b *testing.B) {
		start := baseTime.Add(-30 * 24 * time.Hour)
		end := baseTime
		
		for i := 0; i < b.N; i++ {
			_, err := store.GetTopModels(5, start, end)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}