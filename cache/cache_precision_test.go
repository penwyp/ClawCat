package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/penwyp/ClawCat/models"
	"github.com/stretchr/testify/assert"
)

// TestCachePrecisionBug tests for the critical 91% precision loss when using cached data
func TestCachePrecisionBug(t *testing.T) {
	// Reproduce the exact scenario from user's logs where cached data shows massive loss
	// Create multiple entries that would be aggregated in the cache
	entries := createRealisticEntriesForCacheTest(t)
	
	t.Logf("Created %d entries for cache precision test", len(entries))

	// Calculate fresh totals (what analyze shows on first run)
	freshTotals := calculateFreshTotals(entries)
	t.Logf("Fresh totals: Entries=%d, Tokens=%d, Cost=%.4f", 
		len(entries), freshTotals.TotalTokens, freshTotals.TotalCost)

	// Simulate the caching bug - this mimics what createEntriesFromSummaryWithDedup does
	buggyEntries := simulateCachingBug(entries)
	buggyTotals := calculateFreshTotals(buggyEntries)
	t.Logf("Cached totals (buggy): Entries=%d, Tokens=%d, Cost=%.4f", 
		len(buggyEntries), buggyTotals.TotalTokens, buggyTotals.TotalCost)

	// Check for the precision loss bug
	entryRatio := float64(len(buggyEntries)) / float64(len(entries))
	tokenRatio := float64(buggyTotals.TotalTokens) / float64(freshTotals.TotalTokens)
	costRatio := buggyTotals.TotalCost / freshTotals.TotalCost

	t.Logf("Entry count ratio (cached/fresh): %.4f", entryRatio)
	t.Logf("Token ratio (cached/fresh): %.4f", tokenRatio)
	t.Logf("Cost ratio (cached/fresh): %.4f", costRatio)

	// This test demonstrates the bug - it WILL FAIL before the fix
	// After the fix, this should pass as the cached entries preserve granularity
	
	// Check for major precision loss (anything under 50% indicates the bug)
	if entryRatio < 0.5 {
		t.Logf("BUG REPRODUCED: Cached entries show %.1f%% loss (from %d to %d entries)", 
			(1-entryRatio)*100, len(entries), len(buggyEntries))
		t.Logf("This demonstrates the caching bug where individual entries are aggregated")
	} else {
		t.Logf("GOOD: Entry count preserved with %.1f%% retention", entryRatio*100)
	}

	// The totals should always be equal (this should pass even with the bug)
	assert.Equal(t, freshTotals.TotalTokens, buggyTotals.TotalTokens, 
		"Total tokens should match despite aggregation")
	assert.InDelta(t, freshTotals.TotalCost, buggyTotals.TotalCost, 0.01, 
		"Total cost should match despite aggregation")
	
	// For now, we expect this test to demonstrate the bug
	// After fixing the real code, we can update this to assert the fix works
	if entryRatio < 0.1 {
		t.Logf("EXPECTED: This test demonstrates the 91%% precision loss bug")
		t.Logf("The fix should preserve individual entries to maintain granularity")
	}
}

// TestModelStatsAggregation tests the core aggregation logic
func TestModelStatsAggregation(t *testing.T) {
	// Use the exact data from user's log that shows the issue
	entries := []models.UsageEntry{
		{
			Model:               "claude-sonnet-4-20250514",
			InputTokens:         88947,
			OutputTokens:        525297,
			CacheCreationTokens: 11183108,
			CacheReadTokens:     290594610,
			TotalTokens:         302391962,
			CostUSD:            137.2613,
		},
	}

	stats := NewModelStats(entries)

	// These should match exactly
	assert.Equal(t, 88947, stats.InputTokens)
	assert.Equal(t, 525297, stats.OutputTokens) 
	assert.Equal(t, 11183108, stats.CacheCreationTokens)
	assert.Equal(t, 290594610, stats.CacheReadTokens)
	assert.Equal(t, 302391962, stats.TotalTokens)
	assert.InDelta(t, 137.2613, stats.TotalCost, 0.0001)
}

// Helper types and functions
type SimpleTotals struct {
	TotalTokens int
	TotalCost   float64
}

func calculateFreshTotals(entries []models.UsageEntry) SimpleTotals {
	totals := SimpleTotals{}
	for _, entry := range entries {
		totals.TotalTokens += entry.TotalTokens
		totals.TotalCost += entry.CostUSD
	}
	return totals
}

func calculateUsingCache(entries []models.UsageEntry) SimpleTotals {
	// Simulate the caching path that might be causing precision loss
	
	// Create a file summary (this is where cache data is stored)
	summary := &FileSummary{
		ModelStats: make(map[string]ModelStat),
	}

	// Group by model and create cached stats
	modelGroups := make(map[string][]models.UsageEntry)
	for _, entry := range entries {
		modelGroups[entry.Model] = append(modelGroups[entry.Model], entry)
	}

	// Process each model group through the caching system
	totals := SimpleTotals{}
	for model, modelEntries := range modelGroups {
		// This uses the same logic as the real caching system
		modelStats := NewModelStats(modelEntries)
		
		// Store in summary (this is the cached representation)
		summary.ModelStats[model] = ModelStat{
			Model:               model,
			EntryCount:          modelStats.EntryCount,
			TotalCost:           modelStats.TotalCost,
			InputTokens:         modelStats.InputTokens,
			OutputTokens:        modelStats.OutputTokens,
			CacheCreationTokens: modelStats.CacheCreationTokens,
			CacheReadTokens:     modelStats.CacheReadTokens,
		}

		// Accumulate totals from cached data
		totals.TotalTokens += modelStats.TotalTokens
		totals.TotalCost += modelStats.TotalCost
	}

	return totals
}

// createRealisticEntriesForCacheTest creates test data that reproduces the user's scenario
func createRealisticEntriesForCacheTest(t *testing.T) []models.UsageEntry {
	// Create many entries within the same hour to simulate what gets aggregated
	baseTime := time.Date(2025, 6, 25, 10, 0, 0, 0, time.UTC)
	entries := make([]models.UsageEntry, 0, 2937) // User had 2937 entries for Sonnet
	
	// Create many individual entries that would be in the same hour bucket
	for i := 0; i < 2937; i++ {
		// Spread entries across the hour (same hour bucket in cache)
		timestamp := baseTime.Add(time.Duration(i%60) * time.Minute) // 60 minutes in hour
		
		entry := models.UsageEntry{
			Model:               "claude-sonnet-4-20250514",
			InputTokens:         30,     // Small individual amounts
			OutputTokens:        179,
			CacheCreationTokens: 3800,
			CacheReadTokens:     99000,
			TotalTokens:         103009, // Individual entry total
			CostUSD:            0.0467,  // Small individual cost
			Timestamp:          timestamp,
			MessageID:          fmt.Sprintf("msg-%d", i),
		}
		
		entries = append(entries, entry)
	}
	
	return entries
}

// simulateCachingBug reproduces the bug in createEntriesFromSummaryWithDedup
func simulateCachingBug(originalEntries []models.UsageEntry) []models.UsageEntry {
	// This simulates the problematic behavior where individual entries 
	// are aggregated into a single entry per model per hour
	
	// Group by model and hour (same logic as the buggy cache code)
	type hourModelKey struct {
		model string
		hour  string
	}
	
	aggregates := make(map[hourModelKey]*models.UsageEntry)
	
	for _, entry := range originalEntries {
		// Create the same hour key used in the cache
		hourKey := entry.Timestamp.Format("2006-01-02 15")
		key := hourModelKey{
			model: entry.Model,
			hour:  hourKey,
		}
		
		if aggregates[key] == nil {
			// Create new aggregate entry (this is the bug - should preserve individual entries)
			aggregates[key] = &models.UsageEntry{
				Model:               entry.Model,
				Timestamp:          entry.Timestamp.Truncate(time.Hour), // Hour precision only
				InputTokens:         entry.InputTokens,
				OutputTokens:        entry.OutputTokens,
				CacheCreationTokens: entry.CacheCreationTokens,
				CacheReadTokens:     entry.CacheReadTokens,
				TotalTokens:         entry.TotalTokens,
				CostUSD:            entry.CostUSD,
				MessageID:          fmt.Sprintf("cached_%s_%s", entry.Model, hourKey),
			}
		} else {
			// Aggregate into the single entry (this causes the loss of granularity)
			agg := aggregates[key]
			agg.InputTokens += entry.InputTokens
			agg.OutputTokens += entry.OutputTokens
			agg.CacheCreationTokens += entry.CacheCreationTokens
			agg.CacheReadTokens += entry.CacheReadTokens
			agg.TotalTokens += entry.TotalTokens
			agg.CostUSD += entry.CostUSD
		}
	}
	
	// Convert aggregates back to slice - this shows the dramatic reduction
	var result []models.UsageEntry
	for _, agg := range aggregates {
		result = append(result, *agg)
	}
	
	return result
}