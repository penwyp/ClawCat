package calculations

import (
	"sort"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// StatsAggregator provides flexible aggregation for various time periods
type StatsAggregator struct {
	timezone   *time.Location
	calculator *CostCalculator
}

// PeriodStats represents aggregated statistics for a time period
type PeriodStats struct {
	Period              string                `json:"period"`
	StartTime           time.Time             `json:"start_time"`
	EndTime             time.Time             `json:"end_time"`
	InputTokens         int                   `json:"input_tokens"`
	OutputTokens        int                   `json:"output_tokens"`
	CacheCreationTokens int                   `json:"cache_creation_tokens"`
	CacheReadTokens     int                   `json:"cache_read_tokens"`
	TotalTokens         int                   `json:"total_tokens"`
	TotalCost           float64               `json:"total_cost"`
	EntryCount          int                   `json:"entry_count"`
	ModelBreakdown      map[string]ModelStats `json:"model_breakdown"`
	AverageCostPerToken float64               `json:"average_cost_per_token"`
	AverageCostPerEntry float64               `json:"average_cost_per_entry"`
}

// ModelStats represents statistics for a specific model
type ModelStats struct {
	Model               string  `json:"model"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	TotalTokens         int     `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	EntryCount          int     `json:"entry_count"`
	Percentage          float64 `json:"percentage"`
	AverageCostPerToken float64 `json:"average_cost_per_token"`
}

// OverallStats represents overall statistics across all periods
type OverallStats struct {
	TotalTokens        int                   `json:"total_tokens"`
	TotalCost          float64               `json:"total_cost"`
	TotalEntries       int                   `json:"total_entries"`
	UniqueDays         int                   `json:"unique_days"`
	AverageDailyCost   float64               `json:"average_daily_cost"`
	AverageDailyTokens int                   `json:"average_daily_tokens"`
	ModelDistribution  map[string]ModelStats `json:"model_distribution"`
	TimeRange          TimeRange             `json:"time_range"`
	PeakUsageHour      int                   `json:"peak_usage_hour"`
	CacheUtilization   CacheStats            `json:"cache_utilization"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// CacheStats represents cache utilization statistics
type CacheStats struct {
	TotalCacheTokens    int     `json:"total_cache_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
	CacheSavingsPercent float64 `json:"cache_savings_percent"`
	CacheSavingsCost    float64 `json:"cache_savings_cost"`
}

// NewStatsAggregator creates a new stats aggregator
func NewStatsAggregator(tz *time.Location) *StatsAggregator {
	if tz == nil {
		tz = time.UTC
	}
	return &StatsAggregator{
		timezone:   tz,
		calculator: NewCostCalculator(),
	}
}

// AggregateByHour aggregates entries by hour
func (s *StatsAggregator) AggregateByHour(entries []models.UsageEntry) []PeriodStats {
	return s.aggregateByDuration(entries, time.Hour, "hour")
}

// AggregateByDay aggregates entries by day
func (s *StatsAggregator) AggregateByDay(entries []models.UsageEntry) []PeriodStats {
	return s.aggregateByDuration(entries, 24*time.Hour, "day")
}

// AggregateByWeek aggregates entries by week
func (s *StatsAggregator) AggregateByWeek(entries []models.UsageEntry) []PeriodStats {
	return s.aggregateByDuration(entries, 7*24*time.Hour, "week")
}

// AggregateByMonth aggregates entries by month
func (s *StatsAggregator) AggregateByMonth(entries []models.UsageEntry) []PeriodStats {
	if len(entries) == 0 {
		return []PeriodStats{}
	}

	// Group by month
	monthGroups := make(map[string][]models.UsageEntry)
	for _, entry := range entries {
		entryTime := entry.Timestamp.In(s.timezone)
		key := entryTime.Format("2006-01")
		monthGroups[key] = append(monthGroups[key], entry)
	}

	// Convert to PeriodStats
	var results []PeriodStats
	for month, groupEntries := range monthGroups {
		if len(groupEntries) == 0 {
			continue
		}

		// Parse month to get start time
		startTime, _ := time.ParseInLocation("2006-01", month, s.timezone)
		endTime := startTime.AddDate(0, 1, 0).Add(-time.Nanosecond)

		stats := s.calculatePeriodStats(groupEntries, startTime, endTime, "month")
		results = append(results, stats)
	}

	// Sort by start time
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.Before(results[j].StartTime)
	})

	return results
}

// AggregateCustom aggregates entries by custom duration
func (s *StatsAggregator) AggregateCustom(entries []models.UsageEntry, duration time.Duration) []PeriodStats {
	return s.aggregateByDuration(entries, duration, "custom")
}

// GetOverallStats calculates overall statistics
func (s *StatsAggregator) GetOverallStats(entries []models.UsageEntry) OverallStats {
	if len(entries) == 0 {
		return OverallStats{}
	}

	// Sort entries by timestamp
	sortedEntries := make([]models.UsageEntry, len(entries))
	copy(sortedEntries, entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].Timestamp.Before(sortedEntries[j].Timestamp)
	})

	stats := OverallStats{
		ModelDistribution: make(map[string]ModelStats),
		TimeRange: TimeRange{
			Start: sortedEntries[0].Timestamp,
			End:   sortedEntries[len(sortedEntries)-1].Timestamp,
		},
	}
	stats.TimeRange.Duration = stats.TimeRange.End.Sub(stats.TimeRange.Start)

	// Calculate aggregates
	modelCounts := make(map[string]ModelStats)
	hourCounts := make(map[int]int)
	daySet := make(map[string]bool)

	totalCacheCreation := 0
	totalCacheRead := 0

	for _, entry := range sortedEntries {
		// Overall totals
		stats.TotalTokens += entry.TotalTokens
		stats.TotalEntries++

		// Model breakdown
		modelStats := modelCounts[entry.Model]
		modelStats.Model = entry.Model
		modelStats.InputTokens += entry.InputTokens
		modelStats.OutputTokens += entry.OutputTokens
		modelStats.CacheCreationTokens += entry.CacheCreationTokens
		modelStats.CacheReadTokens += entry.CacheReadTokens
		modelStats.TotalTokens += entry.TotalTokens
		modelStats.EntryCount++

		// Calculate cost for this entry
		if costResult, err := s.calculator.Calculate(entry); err == nil {
			stats.TotalCost += costResult.TotalCost
			modelStats.TotalCost += costResult.TotalCost
		}

		modelCounts[entry.Model] = modelStats

		// Hour analysis
		hour := entry.Timestamp.In(s.timezone).Hour()
		hourCounts[hour]++

		// Day tracking
		day := entry.Timestamp.In(s.timezone).Format("2006-01-02")
		daySet[day] = true

		// Cache tracking
		totalCacheCreation += entry.CacheCreationTokens
		totalCacheRead += entry.CacheReadTokens
	}

	// Calculate percentages and averages for model distribution
	for model, modelStats := range modelCounts {
		if stats.TotalTokens > 0 {
			modelStats.Percentage = float64(modelStats.TotalTokens) / float64(stats.TotalTokens) * 100
		}
		if modelStats.TotalTokens > 0 {
			modelStats.AverageCostPerToken = modelStats.TotalCost / float64(modelStats.TotalTokens)
		}
		stats.ModelDistribution[model] = modelStats
	}

	// Find peak usage hour
	maxHourCount := 0
	for hour, count := range hourCounts {
		if count > maxHourCount {
			maxHourCount = count
			stats.PeakUsageHour = hour
		}
	}

	// Calculate daily averages
	stats.UniqueDays = len(daySet)
	if stats.UniqueDays > 0 {
		stats.AverageDailyCost = stats.TotalCost / float64(stats.UniqueDays)
		stats.AverageDailyTokens = stats.TotalTokens / stats.UniqueDays
	}

	// Calculate cache statistics
	stats.CacheUtilization = s.calculateCacheStats(totalCacheCreation, totalCacheRead, stats.TotalTokens, stats.TotalCost)

	return stats
}

// aggregateByDuration is a helper function to aggregate entries by a fixed duration
func (s *StatsAggregator) aggregateByDuration(entries []models.UsageEntry, duration time.Duration, periodType string) []PeriodStats {
	if len(entries) == 0 {
		return []PeriodStats{}
	}

	// Group entries by time periods
	groups := make(map[time.Time][]models.UsageEntry)

	for _, entry := range entries {
		entryTime := entry.Timestamp.In(s.timezone)
		periodStart := s.roundToInterval(entryTime, duration)
		groups[periodStart] = append(groups[periodStart], entry)
	}

	// Convert groups to PeriodStats
	results := make([]PeriodStats, 0, len(groups))
	for startTime, groupEntries := range groups {
		endTime := startTime.Add(duration).Add(-time.Nanosecond)
		stats := s.calculatePeriodStats(groupEntries, startTime, endTime, periodType)
		results = append(results, stats)
	}

	// Sort by start time
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.Before(results[j].StartTime)
	})

	return results
}

// calculatePeriodStats calculates statistics for a group of entries in a period
func (s *StatsAggregator) calculatePeriodStats(entries []models.UsageEntry, startTime, endTime time.Time, periodType string) PeriodStats {
	stats := PeriodStats{
		Period:         periodType,
		StartTime:      startTime,
		EndTime:        endTime,
		EntryCount:     len(entries),
		ModelBreakdown: make(map[string]ModelStats),
	}

	modelCounts := make(map[string]ModelStats)

	// Aggregate all entries
	for _, entry := range entries {
		stats.InputTokens += entry.InputTokens
		stats.OutputTokens += entry.OutputTokens
		stats.CacheCreationTokens += entry.CacheCreationTokens
		stats.CacheReadTokens += entry.CacheReadTokens
		stats.TotalTokens += entry.TotalTokens

		// Model breakdown
		modelStats := modelCounts[entry.Model]
		modelStats.Model = entry.Model
		modelStats.InputTokens += entry.InputTokens
		modelStats.OutputTokens += entry.OutputTokens
		modelStats.CacheCreationTokens += entry.CacheCreationTokens
		modelStats.CacheReadTokens += entry.CacheReadTokens
		modelStats.TotalTokens += entry.TotalTokens
		modelStats.EntryCount++

		// Calculate cost
		if costResult, err := s.calculator.Calculate(entry); err == nil {
			stats.TotalCost += costResult.TotalCost
			modelStats.TotalCost += costResult.TotalCost
		}

		modelCounts[entry.Model] = modelStats
	}

	// Calculate percentages for model breakdown
	for model, modelStats := range modelCounts {
		if stats.TotalTokens > 0 {
			modelStats.Percentage = float64(modelStats.TotalTokens) / float64(stats.TotalTokens) * 100
		}
		if modelStats.TotalTokens > 0 {
			modelStats.AverageCostPerToken = modelStats.TotalCost / float64(modelStats.TotalTokens)
		}
		stats.ModelBreakdown[model] = modelStats
	}

	// Calculate averages for the period
	if stats.TotalTokens > 0 {
		stats.AverageCostPerToken = stats.TotalCost / float64(stats.TotalTokens)
	}
	if stats.EntryCount > 0 {
		stats.AverageCostPerEntry = stats.TotalCost / float64(stats.EntryCount)
	}

	return stats
}

// calculateCacheStats calculates cache utilization statistics
func (s *StatsAggregator) calculateCacheStats(cacheCreation, cacheRead, totalTokens int, totalCost float64) CacheStats {
	stats := CacheStats{
		TotalCacheTokens:    cacheCreation + cacheRead,
		CacheCreationTokens: cacheCreation,
		CacheReadTokens:     cacheRead,
	}

	if stats.TotalCacheTokens > 0 {
		stats.CacheHitRate = float64(cacheRead) / float64(stats.TotalCacheTokens) * 100
	}

	if totalTokens > 0 {
		stats.CacheSavingsPercent = float64(stats.TotalCacheTokens) / float64(totalTokens) * 100
	}

	// Estimate cache savings (cache read tokens are much cheaper than regular tokens)
	// This is a simplified calculation - in reality it would depend on specific pricing
	if cacheRead > 0 {
		// Assume cache reads save approximately 80-90% of the cost
		estimatedSavings := float64(cacheRead) * 0.85 // Rough estimate
		stats.CacheSavingsCost = estimatedSavings * (totalCost / float64(totalTokens))
	}

	return stats
}

// roundToInterval rounds a time to the nearest interval boundary
func (s *StatsAggregator) roundToInterval(t time.Time, interval time.Duration) time.Time {
	switch interval {
	case time.Hour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case 24 * time.Hour: // Day
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case 7 * 24 * time.Hour: // Week
		// Round to Monday
		days := int(t.Weekday())
		if days == 0 { // Sunday
			days = 7
		}
		days-- // Monday = 0
		return time.Date(t.Year(), t.Month(), t.Day()-days, 0, 0, 0, 0, t.Location())
	default:
		// For custom intervals, round down to the nearest interval
		unix := t.Unix()
		intervalSeconds := int64(interval.Seconds())
		roundedUnix := (unix / intervalSeconds) * intervalSeconds
		return time.Unix(roundedUnix, 0).In(t.Location())
	}
}

// GetTopModels returns the top N models by usage or cost
func (s *StatsAggregator) GetTopModels(entries []models.UsageEntry, n int, sortBy string) []ModelStats {
	overall := s.GetOverallStats(entries)

	models := make([]ModelStats, 0, len(overall.ModelDistribution))
	for _, stats := range overall.ModelDistribution {
		models = append(models, stats)
	}

	// Sort based on criteria
	switch sortBy {
	case "tokens":
		sort.Slice(models, func(i, j int) bool {
			return models[i].TotalTokens > models[j].TotalTokens
		})
	case "cost":
		sort.Slice(models, func(i, j int) bool {
			return models[i].TotalCost > models[j].TotalCost
		})
	case "entries":
		sort.Slice(models, func(i, j int) bool {
			return models[i].EntryCount > models[j].EntryCount
		})
	default:
		// Default to tokens
		sort.Slice(models, func(i, j int) bool {
			return models[i].TotalTokens > models[j].TotalTokens
		})
	}

	// Return top N
	if n > len(models) {
		n = len(models)
	}
	return models[:n]
}

// ComparePeriods compares statistics between two time periods
func (s *StatsAggregator) ComparePeriods(entries1, entries2 []models.UsageEntry) map[string]interface{} {
	stats1 := s.GetOverallStats(entries1)
	stats2 := s.GetOverallStats(entries2)

	comparison := make(map[string]interface{})

	// Token comparison
	comparison["tokens"] = map[string]interface{}{
		"period1":    stats1.TotalTokens,
		"period2":    stats2.TotalTokens,
		"change":     stats2.TotalTokens - stats1.TotalTokens,
		"percentage": s.calculatePercentageChange(float64(stats1.TotalTokens), float64(stats2.TotalTokens)),
	}

	// Cost comparison
	comparison["cost"] = map[string]interface{}{
		"period1":    stats1.TotalCost,
		"period2":    stats2.TotalCost,
		"change":     stats2.TotalCost - stats1.TotalCost,
		"percentage": s.calculatePercentageChange(stats1.TotalCost, stats2.TotalCost),
	}

	// Entry count comparison
	comparison["entries"] = map[string]interface{}{
		"period1":    stats1.TotalEntries,
		"period2":    stats2.TotalEntries,
		"change":     stats2.TotalEntries - stats1.TotalEntries,
		"percentage": s.calculatePercentageChange(float64(stats1.TotalEntries), float64(stats2.TotalEntries)),
	}

	return comparison
}

// calculatePercentageChange calculates percentage change between two values
func (s *StatsAggregator) calculatePercentageChange(old, new float64) float64 {
	if old == 0 {
		if new == 0 {
			return 0
		}
		return 100 // Infinite increase, cap at 100%
	}
	return ((new - old) / old) * 100
}
