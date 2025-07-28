package fileio

import (
	"crypto/md5"
	"fmt"
	"os"
	"time"

	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
)

// createEntriesFromSummary creates entries from a cached summary
func createEntriesFromSummary(summary *cache.FileSummary, cutoffTime *time.Time) []models.UsageEntry {
	var entries []models.UsageEntry

	// Use hourly buckets if available for better temporal granularity
	if len(summary.HourlyBuckets) > 0 {
		// Create entries from hourly buckets
		for hourKey, hourBucket := range summary.HourlyBuckets {
			// Parse the hour timestamp
			hourTime, err := time.Parse("2006-01-02 15", hourKey)
			if err != nil {
				logging.LogWarnf("Failed to parse hour key %s: %v", hourKey, err)
				continue
			}

			// Skip if before cutoff time
			if cutoffTime != nil && hourTime.Before(*cutoffTime) {
				continue
			}

			// Create entries for each model in this hour
			for _, modelStat := range hourBucket.ModelStats {
				if modelStat.EntryCount > 0 {
					// Create individual synthetic entries to preserve granularity
					// Calculate average values per entry
					avgInputTokens := modelStat.InputTokens / modelStat.EntryCount
					avgOutputTokens := modelStat.OutputTokens / modelStat.EntryCount
					avgCacheCreationTokens := modelStat.CacheCreationTokens / modelStat.EntryCount
					avgCacheReadTokens := modelStat.CacheReadTokens / modelStat.EntryCount
					avgCostUSD := modelStat.TotalCost / float64(modelStat.EntryCount)

					// Handle remainders to ensure totals match exactly
					remainderInputTokens := modelStat.InputTokens % modelStat.EntryCount
					remainderOutputTokens := modelStat.OutputTokens % modelStat.EntryCount
					remainderCacheCreationTokens := modelStat.CacheCreationTokens % modelStat.EntryCount
					remainderCacheReadTokens := modelStat.CacheReadTokens % modelStat.EntryCount

					for i := 0; i < modelStat.EntryCount; i++ {
						// Distribute tokens evenly, with remainders in the first entries
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
						}

						entry.NormalizeModel()
						entry.Project = extractProjectFromPath(summary.Path)
						entries = append(entries, entry)
					}
				}
			}
		}
	} else if len(summary.DailyBuckets) > 0 {
		// Fallback to daily buckets if hourly not available
		for dayKey, dayBucket := range summary.DailyBuckets {
			// Parse the day timestamp
			dayTime, err := time.Parse("2006-01-02", dayKey)
			if err != nil {
				logging.LogWarnf("Failed to parse day key %s: %v", dayKey, err)
				continue
			}

			// Skip if before cutoff time
			if cutoffTime != nil && dayTime.Before(*cutoffTime) {
				continue
			}

			// Create entries for each model in this day
			for _, modelStat := range dayBucket.ModelStats {
				if modelStat.EntryCount > 0 {
					// Similar logic as hourly buckets
					avgInputTokens := modelStat.InputTokens / modelStat.EntryCount
					avgOutputTokens := modelStat.OutputTokens / modelStat.EntryCount
					avgCacheCreationTokens := modelStat.CacheCreationTokens / modelStat.EntryCount
					avgCacheReadTokens := modelStat.CacheReadTokens / modelStat.EntryCount
					avgCostUSD := modelStat.TotalCost / float64(modelStat.EntryCount)

					remainderInputTokens := modelStat.InputTokens % modelStat.EntryCount
					remainderOutputTokens := modelStat.OutputTokens % modelStat.EntryCount
					remainderCacheCreationTokens := modelStat.CacheCreationTokens % modelStat.EntryCount
					remainderCacheReadTokens := modelStat.CacheReadTokens % modelStat.EntryCount

					for i := 0; i < modelStat.EntryCount; i++ {
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
							Timestamp:           dayTime.Add(time.Duration(i) * time.Hour),
							Model:               modelStat.Model,
							InputTokens:         inputTokens,
							OutputTokens:        outputTokens,
							CacheCreationTokens: cacheCreationTokens,
							CacheReadTokens:     cacheReadTokens,
							TotalTokens:         inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
							CostUSD:             avgCostUSD,
						}

						entry.NormalizeModel()
						entry.Project = extractProjectFromPath(summary.Path)
						entries = append(entries, entry)
					}
				}
			}
		}
	} else {
		// Fallback to old behavior if no temporal buckets (for backward compatibility)
		for modelName, modelStat := range summary.ModelStats {
			if modelStat.EntryCount > 0 {
				// Create a single aggregated entry per model
				entry := models.UsageEntry{
					Timestamp:           summary.ProcessedAt,
					Model:               modelName,
					InputTokens:         modelStat.InputTokens,
					OutputTokens:        modelStat.OutputTokens,
					CacheCreationTokens: modelStat.CacheCreationTokens,
					CacheReadTokens:     modelStat.CacheReadTokens,
					TotalTokens:         modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
					CostUSD:             modelStat.TotalCost,
				}

				entry.NormalizeModel()
				entry.Project = extractProjectFromPath(summary.Path)
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

// createSummaryFromEntries creates a FileSummary from processed entries
func createSummaryFromEntries(absPath, filePath string, entries []models.UsageEntry, fileInfo os.FileInfo) *cache.FileSummary {
	summary := &cache.FileSummary{
		Path:          filePath,
		AbsolutePath:  absPath,
		ModTime:       fileInfo.ModTime(),
		FileSize:      fileInfo.Size(),
		EntryCount:    len(entries),
		ProcessedAt:   time.Now(),
		ModelStats:    make(map[string]cache.ModelStat),
		HourlyBuckets: make(map[string]*cache.TemporalBucket),
		DailyBuckets:  make(map[string]*cache.TemporalBucket),
	}

	// Calculate checksum (simple approach based on file mod time and size)
	summary.Checksum = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s_%d_%d",
		absPath, fileInfo.ModTime().Unix(), fileInfo.Size()))))

	// Process entries to create statistics
	var totalCost float64
	var totalTokens int
	var startTime, endTime time.Time

	for i, entry := range entries {
		// Track time range
		if i == 0 || entry.Timestamp.Before(startTime) {
			startTime = entry.Timestamp
		}
		if i == 0 || entry.Timestamp.After(endTime) {
			endTime = entry.Timestamp
		}

		// Update totals
		totalCost += entry.CostUSD
		totalTokens += entry.TotalTokens

		// Update model stats
		modelStat, exists := summary.ModelStats[entry.Model]
		if !exists {
			modelStat = cache.ModelStat{
				Model: entry.Model,
			}
		}
		modelStat.EntryCount++
		modelStat.TotalCost += entry.CostUSD
		modelStat.InputTokens += entry.InputTokens
		modelStat.OutputTokens += entry.OutputTokens
		modelStat.CacheCreationTokens += entry.CacheCreationTokens
		modelStat.CacheReadTokens += entry.CacheReadTokens
		summary.ModelStats[entry.Model] = modelStat

		// Update hourly bucket
		hourKey := entry.Timestamp.Format("2006-01-02 15")
		hourBucket, exists := summary.HourlyBuckets[hourKey]
		if !exists {
			hourBucket = &cache.TemporalBucket{
				Period:     hourKey,
				ModelStats: make(map[string]*cache.ModelStat),
			}
			summary.HourlyBuckets[hourKey] = hourBucket
		}
		hourBucket.EntryCount++
		hourBucket.TotalCost += entry.CostUSD
		hourBucket.TotalTokens += entry.TotalTokens

		// Update model stats within hour bucket
		hourModelStat, exists := hourBucket.ModelStats[entry.Model]
		if !exists {
			hourModelStat = &cache.ModelStat{
				Model: entry.Model,
			}
			hourBucket.ModelStats[entry.Model] = hourModelStat
		}
		hourModelStat.EntryCount++
		hourModelStat.TotalCost += entry.CostUSD
		hourModelStat.InputTokens += entry.InputTokens
		hourModelStat.OutputTokens += entry.OutputTokens
		hourModelStat.CacheCreationTokens += entry.CacheCreationTokens
		hourModelStat.CacheReadTokens += entry.CacheReadTokens

		// Update daily bucket
		dayKey := entry.Timestamp.Format("2006-01-02")
		dayBucket, exists := summary.DailyBuckets[dayKey]
		if !exists {
			dayBucket = &cache.TemporalBucket{
				Period:     dayKey,
				ModelStats: make(map[string]*cache.ModelStat),
			}
			summary.DailyBuckets[dayKey] = dayBucket
		}
		dayBucket.EntryCount++
		dayBucket.TotalCost += entry.CostUSD
		dayBucket.TotalTokens += entry.TotalTokens

		// Update model stats within day bucket
		dayModelStat, exists := dayBucket.ModelStats[entry.Model]
		if !exists {
			dayModelStat = &cache.ModelStat{
				Model: entry.Model,
			}
			dayBucket.ModelStats[entry.Model] = dayModelStat
		}
		dayModelStat.EntryCount++
		dayModelStat.TotalCost += entry.CostUSD
		dayModelStat.InputTokens += entry.InputTokens
		dayModelStat.OutputTokens += entry.OutputTokens
		dayModelStat.CacheCreationTokens += entry.CacheCreationTokens
		dayModelStat.CacheReadTokens += entry.CacheReadTokens
	}

	summary.TotalCost = totalCost
	summary.TotalTokens = totalTokens

	return summary
}

// createEmptySummaryForFile creates a minimal FileSummary for files without assistant messages
func createEmptySummaryForFile(absPath, filePath string) *cache.FileSummary {
	fileInfo, _ := os.Stat(filePath)
	summary := &cache.FileSummary{
		Path:                   filePath,
		AbsolutePath:           absPath,
		ModTime:                fileInfo.ModTime(),
		FileSize:               fileInfo.Size(),
		EntryCount:             0,
		ProcessedAt:            time.Now(),
		ModelStats:             make(map[string]cache.ModelStat),
		HourlyBuckets:          make(map[string]*cache.TemporalBucket),
		DailyBuckets:           make(map[string]*cache.TemporalBucket),
		HasNoAssistantMessages: true,
	}

	// Calculate checksum
	summary.Checksum = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s_%d_%d",
		absPath, fileInfo.ModTime().Unix(), fileInfo.Size()))))

	return summary
}