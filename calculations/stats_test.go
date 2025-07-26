package calculations

import (
	"testing"
	"time"

	"github.com/penwyp/claudecat/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStatsAggregator(t *testing.T) {
	tz := time.UTC
	aggregator := NewStatsAggregator(tz)

	require.NotNil(t, aggregator)
	assert.Equal(t, tz, aggregator.timezone)
	assert.NotNil(t, aggregator.calculator)
}

func TestNewStatsAggregator_NilTimezone(t *testing.T) {
	aggregator := NewStatsAggregator(nil)

	require.NotNil(t, aggregator)
	assert.Equal(t, time.UTC, aggregator.timezone)
}

func createTestEntries() []models.UsageEntry {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	return []models.UsageEntry{
		{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    baseTime,
		},
		{
			Model:        models.ModelSonnet,
			InputTokens:  2000,
			OutputTokens: 1000,
			TotalTokens:  3000,
			Timestamp:    baseTime.Add(30 * time.Minute),
		},
		{
			Model:        models.ModelHaiku,
			InputTokens:  3000,
			OutputTokens: 1500,
			TotalTokens:  4500,
			Timestamp:    baseTime.Add(2 * time.Hour),
		},
		{
			Model:               models.ModelOpus,
			InputTokens:         1500,
			OutputTokens:        750,
			CacheCreationTokens: 200,
			CacheReadTokens:     100,
			TotalTokens:         2550,
			Timestamp:           baseTime.Add(25 * time.Hour), // Next day
		},
	}
}

func TestStatsAggregator_AggregateByHour(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	stats := aggregator.AggregateByHour(entries)

	// Should have 3 hours (0, 2, and 25th hour which is next day)
	assert.Len(t, stats, 3)

	// First hour should have 2 entries (Sonnet models)
	firstHour := stats[0]
	assert.Equal(t, "hour", firstHour.Period)
	assert.Equal(t, 2, firstHour.EntryCount)
	assert.Equal(t, 3000, firstHour.InputTokens)  // 1000 + 2000
	assert.Equal(t, 1500, firstHour.OutputTokens) // 500 + 1000
	assert.Equal(t, 4500, firstHour.TotalTokens)  // 1500 + 3000

	// Model breakdown should have only Sonnet
	assert.Len(t, firstHour.ModelBreakdown, 1)
	sonnetStats, exists := firstHour.ModelBreakdown[models.ModelSonnet]
	assert.True(t, exists)
	assert.Equal(t, 4500, sonnetStats.TotalTokens)
	assert.Equal(t, 2, sonnetStats.EntryCount)
	assert.InDelta(t, 100.0, sonnetStats.Percentage, 0.1) // Should be 100% in this hour
}

func TestStatsAggregator_AggregateByDay(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	stats := aggregator.AggregateByDay(entries)

	// Should have 2 days
	assert.Len(t, stats, 2)

	// First day should have 3 entries
	firstDay := stats[0]
	assert.Equal(t, "day", firstDay.Period)
	assert.Equal(t, 3, firstDay.EntryCount)
	assert.Equal(t, 6000, firstDay.InputTokens)  // 1000 + 2000 + 3000
	assert.Equal(t, 3000, firstDay.OutputTokens) // 500 + 1000 + 1500
	assert.Equal(t, 9000, firstDay.TotalTokens)  // 1500 + 3000 + 4500

	// Should have both Sonnet and Haiku
	assert.Len(t, firstDay.ModelBreakdown, 2)

	// Second day should have 1 entry (Opus)
	secondDay := stats[1]
	assert.Equal(t, 1, secondDay.EntryCount)
	assert.Equal(t, 1500, secondDay.InputTokens)
	assert.Equal(t, 200, secondDay.CacheCreationTokens)
	assert.Equal(t, 100, secondDay.CacheReadTokens)
	assert.Len(t, secondDay.ModelBreakdown, 1)

	opusStats, exists := secondDay.ModelBreakdown[models.ModelOpus]
	assert.True(t, exists)
	assert.Equal(t, 2550, opusStats.TotalTokens)
}

func TestStatsAggregator_AggregateByWeek(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	stats := aggregator.AggregateByWeek(entries)

	// All entries should be in the same week
	assert.Len(t, stats, 1)

	weekStats := stats[0]
	assert.Equal(t, "week", weekStats.Period)
	assert.Equal(t, 4, weekStats.EntryCount)
	assert.Equal(t, 7500, weekStats.InputTokens)  // Sum of all input tokens
	assert.Equal(t, 3750, weekStats.OutputTokens) // Sum of all output tokens
	assert.Equal(t, 11550, weekStats.TotalTokens) // Sum of all total tokens

	// Should have all 3 models
	assert.Len(t, weekStats.ModelBreakdown, 3)
}

func TestStatsAggregator_AggregateByMonth(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	stats := aggregator.AggregateByMonth(entries)

	// All entries are in January 2024
	assert.Len(t, stats, 1)

	monthStats := stats[0]
	assert.Equal(t, "month", monthStats.Period)
	assert.Equal(t, 4, monthStats.EntryCount)
	assert.Equal(t, 7500, monthStats.InputTokens)
	assert.Equal(t, 3750, monthStats.OutputTokens)
	assert.Equal(t, 11550, monthStats.TotalTokens)
}

func TestStatsAggregator_AggregateCustom(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	// Aggregate by 90 minutes
	stats := aggregator.AggregateCustom(entries, 90*time.Minute)

	// Should group first two entries together, third entry separate, fourth entry separate
	assert.Len(t, stats, 3)

	firstPeriod := stats[0]
	assert.Equal(t, "custom", firstPeriod.Period)
	assert.Equal(t, 2, firstPeriod.EntryCount) // First two Sonnet entries
}

func TestStatsAggregator_GetOverallStats(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	stats := aggregator.GetOverallStats(entries)

	// Check overall totals
	assert.Equal(t, 11550, stats.TotalTokens)
	assert.Equal(t, 4, stats.TotalEntries)
	assert.Equal(t, 2, stats.UniqueDays) // Entries span 2 days
	assert.Greater(t, stats.TotalCost, 0.0)

	// Check model distribution
	assert.Len(t, stats.ModelDistribution, 3)

	// Check time range
	expectedStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.Add(25 * time.Hour)
	assert.Equal(t, expectedStart, stats.TimeRange.Start)
	assert.Equal(t, expectedEnd, stats.TimeRange.End)
	assert.Equal(t, 25*time.Hour, stats.TimeRange.Duration)

	// Check cache utilization
	assert.Equal(t, 300, stats.CacheUtilization.TotalCacheTokens) // 200 + 100
	assert.Equal(t, 200, stats.CacheUtilization.CacheCreationTokens)
	assert.Equal(t, 100, stats.CacheUtilization.CacheReadTokens)
	assert.InDelta(t, 33.33, stats.CacheUtilization.CacheHitRate, 0.1) // 100/300 * 100
}

func TestStatsAggregator_GetOverallStats_EmptyEntries(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)

	stats := aggregator.GetOverallStats([]models.UsageEntry{})

	assert.Equal(t, 0, stats.TotalTokens)
	assert.Equal(t, 0, stats.TotalEntries)
	assert.Equal(t, 0, stats.UniqueDays)
	assert.Equal(t, 0.0, stats.TotalCost)
	assert.Equal(t, 0, len(stats.ModelDistribution))
}

func TestStatsAggregator_GetTopModels(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	// Test top models by tokens
	topByTokens := aggregator.GetTopModels(entries, 2, "tokens")
	assert.Len(t, topByTokens, 2)

	// Haiku should be first (4500 tokens), then Sonnet (4500 total from 2 entries)
	// Actually, let's check the logic - Sonnet has 1500 + 3000 = 4500 total
	// Haiku has 4500 from one entry
	// Opus has 2550 from one entry
	// So it should be Sonnet and Haiku at the top

	// Test top models by cost
	topByCost := aggregator.GetTopModels(entries, 3, "cost")
	assert.Len(t, topByCost, 3) // Should return all 3 since we asked for 3

	// Test top models by entries
	topByEntries := aggregator.GetTopModels(entries, 2, "entries")
	assert.Len(t, topByEntries, 2)
	// Sonnet should be first (2 entries), then Haiku and Opus (1 entry each)
	assert.Equal(t, models.ModelSonnet, topByEntries[0].Model)
	assert.Equal(t, 2, topByEntries[0].EntryCount)
}

func TestStatsAggregator_ComparePeriods(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)

	// Create two sets of entries for comparison
	entries1 := []models.UsageEntry{
		{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	entries2 := []models.UsageEntry{
		{
			Model:        models.ModelSonnet,
			InputTokens:  2000,
			OutputTokens: 1000,
			TotalTokens:  3000,
			Timestamp:    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	comparison := aggregator.ComparePeriods(entries1, entries2)

	// Check structure
	assert.Contains(t, comparison, "tokens")
	assert.Contains(t, comparison, "cost")
	assert.Contains(t, comparison, "entries")

	// Check token comparison
	tokenComp := comparison["tokens"].(map[string]interface{})
	assert.Equal(t, 1500, tokenComp["period1"])
	assert.Equal(t, 3000, tokenComp["period2"])
	assert.Equal(t, 1500, tokenComp["change"])
	assert.InDelta(t, 100.0, tokenComp["percentage"], 0.1) // 100% increase
}

func TestStatsAggregator_CalculatePercentageChange(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)

	tests := []struct {
		old      float64
		new      float64
		expected float64
	}{
		{100, 150, 50.0},  // 50% increase
		{200, 100, -50.0}, // 50% decrease
		{100, 100, 0.0},   // No change
		{0, 100, 100.0},   // From zero (capped at 100%)
		{0, 0, 0.0},       // Both zero
	}

	for _, tt := range tests {
		result := aggregator.calculatePercentageChange(tt.old, tt.new)
		assert.InDelta(t, tt.expected, result, 0.1, "Old: %f, New: %f", tt.old, tt.new)
	}
}

func TestStatsAggregator_RoundToInterval(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)
	testTime := time.Date(2024, 1, 15, 14, 35, 42, 123456789, time.UTC)

	tests := []struct {
		interval time.Duration
		expected time.Time
	}{
		{
			time.Hour,
			time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			24 * time.Hour, // Day
			time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			7 * 24 * time.Hour,                           // Week (rounds to Monday)
			time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // 2024-01-15 was a Monday
		},
		{
			30 * time.Minute, // Custom interval
			time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		result := aggregator.roundToInterval(testTime, tt.interval)
		assert.Equal(t, tt.expected, result, "Interval: %v", tt.interval)
	}
}

func TestStatsAggregator_CalculateCacheStats(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)

	cacheStats := aggregator.calculateCacheStats(200, 100, 1000, 10.0)

	assert.Equal(t, 300, cacheStats.TotalCacheTokens)
	assert.Equal(t, 200, cacheStats.CacheCreationTokens)
	assert.Equal(t, 100, cacheStats.CacheReadTokens)
	assert.InDelta(t, 33.33, cacheStats.CacheHitRate, 0.1)       // 100/300 * 100
	assert.InDelta(t, 30.0, cacheStats.CacheSavingsPercent, 0.1) // 300/1000 * 100
	assert.Greater(t, cacheStats.CacheSavingsCost, 0.0)
}

func TestStatsAggregator_EmptyEntries(t *testing.T) {
	aggregator := NewStatsAggregator(time.UTC)

	// Test all aggregation methods with empty entries
	assert.Empty(t, aggregator.AggregateByHour([]models.UsageEntry{}))
	assert.Empty(t, aggregator.AggregateByDay([]models.UsageEntry{}))
	assert.Empty(t, aggregator.AggregateByWeek([]models.UsageEntry{}))
	assert.Empty(t, aggregator.AggregateByMonth([]models.UsageEntry{}))
	assert.Empty(t, aggregator.AggregateCustom([]models.UsageEntry{}, time.Hour))

	topModels := aggregator.GetTopModels([]models.UsageEntry{}, 5, "tokens")
	assert.Empty(t, topModels)
}

func TestStatsAggregator_TimezoneHandling(t *testing.T) {
	// Test with different timezone
	ny, _ := time.LoadLocation("America/New_York")
	aggregator := NewStatsAggregator(ny)

	// Create entry at midnight UTC (which is 7 PM previous day in NY during winter)
	entries := []models.UsageEntry{
		{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), // Jan 2 00:00 UTC
		},
	}

	dayStats := aggregator.AggregateByDay(entries)
	require.Len(t, dayStats, 1)

	// Should be rounded to Jan 1 in NY timezone
	expectedStart := time.Date(2024, 1, 1, 0, 0, 0, 0, ny)
	assert.Equal(t, expectedStart, dayStats[0].StartTime)
}

func BenchmarkStatsAggregator_AggregateByHour(b *testing.B) {
	aggregator := NewStatsAggregator(time.UTC)

	// Create 1000 entries spread across 24 hours
	entries := make([]models.UsageEntry, 1000)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 1000; i++ {
		entries[i] = models.UsageEntry{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    baseTime.Add(time.Duration(i) * time.Minute),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aggregator.AggregateByHour(entries)
	}
}

func BenchmarkStatsAggregator_GetOverallStats(b *testing.B) {
	aggregator := NewStatsAggregator(time.UTC)
	entries := createTestEntries()

	// Create more entries for a realistic benchmark
	for i := 0; i < 100; i++ {
		entries = append(entries, models.UsageEntry{
			Model:        models.ModelSonnet,
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			Timestamp:    time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = aggregator.GetOverallStats(entries)
	}
}
