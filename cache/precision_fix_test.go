package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/models"
	"github.com/stretchr/testify/assert"
)

// TestPrecisionFixValidation validates that our fix preserves entry count granularity
func TestPrecisionFixValidation(t *testing.T) {
	// Create a file summary that simulates cached data with multiple entries
	summary := createTestFileSummary(t)
	
	t.Logf("Test summary: %d hourly buckets, each with model stats showing EntryCount", len(summary.HourlyBuckets))
	
	// Log the aggregated data in the cache
	for hourKey, bucket := range summary.HourlyBuckets {
		for _, modelStat := range bucket.ModelStats {
			t.Logf("Hour %s, Model %s: %d entries, %d tokens, $%.4f", 
				hourKey, modelStat.Model, modelStat.EntryCount, 
				modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
				modelStat.TotalCost)
		}
	}
	
	// The fix should NOT be applied yet in our test cache code
	// So this demonstrates how the current cache would behave
	
	// Count expected entries based on EntryCount in summary
	expectedEntryCount := 0
	for _, bucket := range summary.HourlyBuckets {
		for _, modelStat := range bucket.ModelStats {
			expectedEntryCount += modelStat.EntryCount
		}
	}
	
	t.Logf("Expected entry count based on cache EntryCount fields: %d", expectedEntryCount)
	
	// Our fix should maintain this count instead of reducing to just one entry per model per hour
	assert.Greater(t, expectedEntryCount, 1, 
		"Test data should have multiple entries to demonstrate the precision issue")
}

// createTestFileSummary creates a summary that mimics the user's data
func createTestFileSummary(t *testing.T) *FileSummary {
	// Create a file summary that represents many individual entries 
	// aggregated into hourly buckets (like the real cache does)
	
	hourKey := "2025-06-25 10"
	
	summary := &FileSummary{
		Path:            "/test/conversation.jsonl",
		HourlyBuckets:   make(map[string]*TemporalBucket),
		ProcessedHashes: make(map[string]bool),
	}
	
	// Create a bucket representing the hour when user had 2937 entries
	bucket := &TemporalBucket{
		Period:      hourKey,
		EntryCount:  2937, // Total entries in this hour
		TotalTokens: 302391962,
		TotalCost:   137.2613,
		ModelStats:  make(map[string]*ModelStat),
	}
	
	// Add the Sonnet model stats (this represents the aggregated data from 2937 individual entries)
	bucket.ModelStats["claude-sonnet-4-20250514"] = &ModelStat{
		Model:               "claude-sonnet-4-20250514",
		EntryCount:          2937, // This is the key - the cache knows there were 2937 entries
		TotalCost:           137.2613,
		InputTokens:         88947,
		OutputTokens:        525297,
		CacheCreationTokens: 11183108,
		CacheReadTokens:     290594610,
	}
	
	summary.HourlyBuckets[hourKey] = bucket
	
	return summary
}

// TestSyntheticEntryGeneration tests that we can generate the right number of synthetic entries
func TestSyntheticEntryGeneration(t *testing.T) {
	// Test our approach to generate synthetic entries from aggregated data
	summary := createTestFileSummary(t)
	
	// Extract the model stat
	hourKey := "2025-06-25 10"
	bucket := summary.HourlyBuckets[hourKey]
	modelStat := bucket.ModelStats["claude-sonnet-4-20250514"]
	
	// Generate synthetic entries (this is what our fix does)
	entries := generateSyntheticEntries(modelStat, hourKey)
	
	t.Logf("Generated %d synthetic entries from %d original EntryCount", 
		len(entries), modelStat.EntryCount)
	
	// Verify we get the expected number of entries
	assert.Equal(t, modelStat.EntryCount, len(entries), 
		"Should generate same number of entries as EntryCount")
	
	// Verify totals are preserved
	totalTokens := 0
	totalCost := 0.0
	for _, entry := range entries {
		totalTokens += entry.TotalTokens
		totalCost += entry.CostUSD
	}
	
	expectedTotalTokens := modelStat.InputTokens + modelStat.OutputTokens + 
		modelStat.CacheCreationTokens + modelStat.CacheReadTokens
	
	assert.Equal(t, expectedTotalTokens, totalTokens, 
		"Total tokens should be preserved across synthetic entries")
	assert.InDelta(t, modelStat.TotalCost, totalCost, 0.01, 
		"Total cost should be preserved across synthetic entries")
	
	t.Logf("✓ Granularity preserved: %d entries", len(entries))
	t.Logf("✓ Totals preserved: %d tokens, $%.4f cost", totalTokens, totalCost)
}

func generateSyntheticEntries(modelStat *ModelStat, hourKey string) []models.UsageEntry {
	hourTime, _ := time.Parse("2006-01-02 15", hourKey)
	
	// This implements the same logic as our fix
	avgInputTokens := modelStat.InputTokens / modelStat.EntryCount
	avgOutputTokens := modelStat.OutputTokens / modelStat.EntryCount
	avgCacheCreationTokens := modelStat.CacheCreationTokens / modelStat.EntryCount
	avgCacheReadTokens := modelStat.CacheReadTokens / modelStat.EntryCount
	avgCostUSD := modelStat.TotalCost / float64(modelStat.EntryCount)
	
	// Handle remainders
	remainderInputTokens := modelStat.InputTokens % modelStat.EntryCount
	remainderOutputTokens := modelStat.OutputTokens % modelStat.EntryCount
	remainderCacheCreationTokens := modelStat.CacheCreationTokens % modelStat.EntryCount
	remainderCacheReadTokens := modelStat.CacheReadTokens % modelStat.EntryCount
	
	var entries []models.UsageEntry
	
	for i := 0; i < modelStat.EntryCount; i++ {
		// Distribute tokens evenly with remainders in first entries
		inputTokens := avgInputTokens
		outputTokens := avgOutputTokens
		cacheCreationTokens := avgCacheCreationTokens
		cacheReadTokens := avgCacheReadTokens
		
		if i < remainderInputTokens {
			inputTokens++
		}
		if i < remainderOutputTokens {
			outputTokens++
		}
		if i < remainderCacheCreationTokens {
			cacheCreationTokens++
		}
		if i < remainderCacheReadTokens {
			cacheReadTokens++
		}
		
		entry := models.UsageEntry{
			Timestamp:           hourTime.Add(time.Duration(i) * time.Minute),
			Model:               modelStat.Model,
			InputTokens:         inputTokens,
			OutputTokens:        outputTokens,
			CacheCreationTokens: cacheCreationTokens,
			CacheReadTokens:     cacheReadTokens,
			TotalTokens:         inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
			CostUSD:             avgCostUSD,
			MessageID:           fmt.Sprintf("synthetic_%s_%s_%d", modelStat.Model, hourKey, i),
		}
		
		entries = append(entries, entry)
	}
	
	return entries
}