package fileio

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
)

// findJSONLFiles discovers all JSONL files in the given path
func findJSONLFiles(dataPath string) ([]string, error) {
	return DiscoverFiles(dataPath)
}

// hasAssistantMessages checks if a file contains assistant messages
func hasAssistantMessages(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Check first 50 lines for assistant messages
	for scanner.Scan() && lineCount < 50 {
		line := scanner.Text()
		lineCount++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var data map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &data); err != nil {
			continue
		}

		// Check if this is an assistant message with usage data
		if typeStr, ok := data["type"].(string); ok && typeStr == "assistant" {
			if message, ok := data["message"].(map[string]interface{}); ok {
				if usage, ok := message["usage"].(map[string]interface{}); ok {
					// Check if usage has any tokens
					for _, field := range []string{"input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
						if val, ok := usage[field]; ok {
							if tokens, ok := val.(float64); ok && tokens > 0 {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// extractUsageEntry extracts usage entry from JSON data
func extractUsageEntry(data map[string]interface{}) (models.UsageEntry, bool) {
	var entry models.UsageEntry
	var hasUsage bool

	// Extract timestamp
	if timestampStr, ok := data["timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			entry.Timestamp = ts
		} else {
			return entry, false
		}
	} else {
		return entry, false
	}

	// Handle different message types
	typeStr, _ := data["type"].(string)

	if typeStr == "assistant" {
		// Claude Code session format
		if message, ok := data["message"].(map[string]interface{}); ok {
			// Extract model
			if model, ok := message["model"].(string); ok {
				entry.Model = model
			}

			// Extract usage
			if usage, ok := message["usage"].(map[string]interface{}); ok {
				if val, ok := usage["input_tokens"]; ok {
					entry.InputTokens = int(val.(float64))
					hasUsage = true
				}
				if val, ok := usage["output_tokens"]; ok {
					entry.OutputTokens = int(val.(float64))
					hasUsage = true
				}
				if val, ok := usage["cache_creation_input_tokens"]; ok {
					entry.CacheCreationTokens = int(val.(float64))
				}
				if val, ok := usage["cache_read_input_tokens"]; ok {
					entry.CacheReadTokens = int(val.(float64))
				}
			}
		}
	} else if typeStr == "message" {
		// Direct API format
		if model, ok := data["model"].(string); ok {
			entry.Model = model
		}

		if usage, ok := data["usage"].(map[string]interface{}); ok {
			if val, ok := usage["input_tokens"]; ok {
				entry.InputTokens = int(val.(float64))
				hasUsage = true
			}
			if val, ok := usage["output_tokens"]; ok {
				entry.OutputTokens = int(val.(float64))
				hasUsage = true
			}
			if val, ok := usage["cache_creation_tokens"]; ok {
				entry.CacheCreationTokens = int(val.(float64))
			}
			if val, ok := usage["cache_read_tokens"]; ok {
				entry.CacheReadTokens = int(val.(float64))
			}
		}
	}

	// Calculate total tokens
	entry.TotalTokens = entry.InputTokens + entry.OutputTokens + entry.CacheCreationTokens + entry.CacheReadTokens

	return entry, hasUsage
}

// LoadUsageEntriesOptions configures the usage loading behavior
type LoadUsageEntriesOptions struct {
	DataPath           string          // Path to Claude data directory
	HoursBack          *int            // Only include entries from last N hours (nil = all data)
	Mode               models.CostMode // Cost calculation mode
	IncludeRaw         bool            // Whether to return raw JSON data alongside entries
	CacheStore         CacheStore      // Optional cache store for file summaries
	EnableSummaryCache bool            // Whether to enable summary caching
	IsWatchMode        bool            // Whether loading is triggered by file watch (TUI mode)
}

// CacheStore defines the interface for file summary caching
type CacheStore interface {
	GetFileSummary(absolutePath string) (*cache.FileSummary, error)
	SetFileSummary(summary *cache.FileSummary) error
	HasFileSummary(absolutePath string) bool
	InvalidateFileSummary(absolutePath string) error
}

// LoadUsageEntriesResult contains the loaded data
type LoadUsageEntriesResult struct {
	Entries    []models.UsageEntry      // Processed usage entries
	RawEntries []map[string]interface{} // Raw JSON data (if requested)
	Metadata   LoadMetadata             // Loading metadata
}

// LoadMetadata contains information about the loading process
type LoadMetadata struct {
	FilesProcessed   int                    `json:"files_processed"`
	EntriesLoaded    int                    `json:"entries_loaded"`
	EntriesFiltered  int                    `json:"entries_filtered"`
	LoadDuration     time.Duration          `json:"load_duration"`
	ProcessingErrors []string               `json:"processing_errors,omitempty"`
	CacheMissReasons map[string]int         `json:"cache_miss_reasons,omitempty"`
	CacheStats       *CachePerformanceStats `json:"cache_stats,omitempty"`
}

// CachePerformanceStats tracks cache performance metrics
type CachePerformanceStats struct {
	Hits                int     `json:"hits"`
	Misses              int     `json:"misses"`
	HitRate             float64 `json:"hit_rate"`
	NewFiles            int     `json:"new_files"`
	ModifiedFiles       int     `json:"modified_files"`
	NoAssistantMessages int     `json:"no_assistant_messages"`
	OtherMisses         int     `json:"other_misses"`
}

// LoadUsageEntries loads and converts JSONL files to UsageEntry objects
func LoadUsageEntries(opts LoadUsageEntriesOptions) (*LoadUsageEntriesResult, error) {
	startTime := time.Now()

	// Find all JSONL files
	jsonlFiles, err := findJSONLFiles(opts.DataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find JSONL files: %w", err)
	}

	// Check if we should use concurrent loading
	useConcurrent := len(jsonlFiles) > 10 // Use concurrent loading for more than 10 files

	var allEntries []models.UsageEntry
	var allRawEntries []map[string]interface{}
	var processingErrors []string
	var cacheHits, cacheMisses int
	cacheMissReasons := map[string]int{
		"new_file":              0,
		"modified_file":         0,
		"no_assistant_messages": 0,
		"other":                 0,
	}
	var summariesToCache []*cache.FileSummary // Collect summaries for batch writing

	if useConcurrent {
		// Use concurrent loader
		loader := NewConcurrentLoader(0) // Use default worker count
		ctx := context.Background()

		// Load files concurrently with progress
		results, err := loader.LoadFilesWithProgress(ctx, jsonlFiles, opts)
		if err != nil {
			return nil, fmt.Errorf("concurrent loading failed: %w", err)
		}

		// Merge results
		var mergeErrors []error
		allEntries, allRawEntries, mergeErrors = MergeResults(results)

		// Convert errors to strings
		for _, err := range mergeErrors {
			processingErrors = append(processingErrors, err.Error())
		}

		// Calculate cache stats and collect summaries
		for _, result := range results {
			if result.Error == nil {
				if result.FromCache {
					cacheHits++
				} else {
					cacheMisses++
					if result.MissReason != "" {
						cacheMissReasons[result.MissReason]++
					}
				}
				// Collect summary for batch writing
				if result.Summary != nil {
					summariesToCache = append(summariesToCache, result.Summary)
				}
			}
		}
	} else {
		// Use sequential loading for small file counts
		// Calculate cutoff time if specified
		var cutoffTime *time.Time
		if opts.HoursBack != nil {
			cutoff := time.Now().UTC().Add(-time.Duration(*opts.HoursBack) * time.Hour)
			cutoffTime = &cutoff
		}

		for i, filePath := range jsonlFiles {
			if i < 5 || i%100 == 0 { // Log first 5 files and every 100th file
				logging.LogDebugf("Processing file %d/%d: %s", i+1, len(jsonlFiles), filepath.Base(filePath))
			}

			entries, rawEntries, fromCache, missReason, err, summary := processSingleFileWithCacheWithReason(filePath, opts, cutoffTime)
			if err != nil {
				if i < 5 { // Log errors for first 5 files
					logging.LogErrorf("Error processing file %s: %v", filepath.Base(filePath), err)
				}
				processingErrors = append(processingErrors, fmt.Sprintf("%s: %v", filePath, err))
				continue
			}

			if fromCache {
				cacheHits++
			} else {
				cacheMisses++
				if missReason != "" {
					cacheMissReasons[missReason]++
				}
			}

			if i < 5 { // Log successful processing for first 5 files
				logging.LogDebugf("File %s processed: %d entries (from cache: %v)", filepath.Base(filePath), len(entries), fromCache)
			}

			allEntries = append(allEntries, entries...)
			if opts.IncludeRaw && rawEntries != nil {
				allRawEntries = append(allRawEntries, rawEntries...)
			}

			// Collect summary for batch writing
			if summary != nil {
				summariesToCache = append(summariesToCache, summary)
			}
		}
	}

	// Sort entries by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Batch write summaries if we have any
	if len(summariesToCache) > 0 && opts.EnableSummaryCache && opts.CacheStore != nil {
		if batcher, ok := opts.CacheStore.(interface {
			BatchSet([]*cache.FileSummary) error
		}); ok {
			if err := batcher.BatchSet(summariesToCache); err != nil {
				logging.LogWarnf("Failed to batch write %d summaries: %v", len(summariesToCache), err)
			} else {
				logging.LogDebugf("Batch wrote %d summaries to cache", len(summariesToCache))
			}
		} else {
			// Fallback to individual writes if batch is not supported
			for _, summary := range summariesToCache {
				if err := opts.CacheStore.SetFileSummary(summary); err != nil {
					logging.LogWarnf("Failed to cache summary for %s: %v", filepath.Base(summary.Path), err)
				}
			}
		}
	}

	// Calculate cache hit rate
	hitRate := float64(0)
	if totalRequests := cacheHits + cacheMisses; totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests)
	}

	// Log cache performance
	if opts.EnableSummaryCache && opts.CacheStore != nil {
		logging.LogInfof("Cache performance: hits=%d, misses=%d (rate=%.1f%%)",
			cacheHits, cacheMisses, hitRate*100)
		if cacheMisses > 0 {
			logging.LogDebugf("Cache miss reasons: new=%d, modified=%d, no_assistant=%d, other=%d",
				cacheMissReasons["new_file"],
				cacheMissReasons["modified_file"],
				cacheMissReasons["no_assistant_messages"],
				cacheMissReasons["other"])
		}
	}

	result := &LoadUsageEntriesResult{
		Entries:    allEntries,
		RawEntries: allRawEntries,
		Metadata: LoadMetadata{
			FilesProcessed:   len(jsonlFiles),
			EntriesLoaded:    len(allEntries),
			LoadDuration:     time.Since(startTime),
			ProcessingErrors: processingErrors,
			CacheMissReasons: cacheMissReasons,
			CacheStats: &CachePerformanceStats{
				Hits:                cacheHits,
				Misses:              cacheMisses,
				HitRate:             hitRate,
				NewFiles:            cacheMissReasons["new_file"],
				ModifiedFiles:       cacheMissReasons["modified_file"],
				NoAssistantMessages: cacheMissReasons["no_assistant_messages"],
				OtherMisses:         cacheMissReasons["other"],
			},
		},
	}

	logging.LogInfof("Loaded %d entries from %d files in %v",
		len(allEntries), len(jsonlFiles), time.Since(startTime))

	if len(processingErrors) > 0 {
		logging.LogWarnf("Encountered %d errors during processing", len(processingErrors))
		for i, err := range processingErrors {
			if i < 5 { // Only log first 5 errors
				logging.LogDebugf("Error %d: %s", i+1, err)
			}
		}
		if len(processingErrors) > 5 {
			logging.LogDebugf("... and %d more errors", len(processingErrors)-5)
		}
	}

	return result, nil
}

// processSingleFileWithCacheWithReason processes a single JSONL file with caching support and returns cache miss reason
func processSingleFileWithCacheWithReason(filePath string, opts LoadUsageEntriesOptions, cutoffTime *time.Time) ([]models.UsageEntry, []map[string]interface{}, bool, string, error, *cache.FileSummary) {
	// Get absolute path for cache key
	absPath, absErr := filepath.Abs(filePath)
	if absErr != nil {
		absPath = filePath // fallback to relative path
	}

	var summary *cache.FileSummary // Declare at the top for return

	// Check if caching is enabled
	if opts.EnableSummaryCache && opts.CacheStore != nil {
		// Get file info first
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			// File doesn't exist, fall back to normal processing
			entries, rawEntries, err := processSingleFile(filePath, opts.Mode, cutoffTime, opts.IncludeRaw)
			return entries, rawEntries, false, "new_file", err, nil
		}

		// Check cache first before reading file contents
		if cachedSummary, err := opts.CacheStore.GetFileSummary(absPath); err == nil {
			// Check if cache is still valid based on file mtime and size
			if !cachedSummary.IsExpired(fileInfo.ModTime(), fileInfo.Size()) {
				// Cache hit - check if this is a file without assistant messages
				if cachedSummary.HasNoAssistantMessages {
					// This file has no assistant messages, return empty results
					return []models.UsageEntry{}, nil, true, "", nil, nil
				}
				// Normal cache hit with data
				entries := createEntriesFromSummary(cachedSummary, cutoffTime)
				return entries, nil, true, "", nil, nil
			} else {
				// File has been modified, invalidate cache
				logging.LogDebugf("Cache miss for %s: file modified (old mtime: %v, new mtime: %v, old size: %d, new size: %d)",
					filepath.Base(filePath), cachedSummary.ModTime, fileInfo.ModTime(), cachedSummary.FileSize, fileInfo.Size())
				opts.CacheStore.InvalidateFileSummary(absPath)
				// Continue to process the file and track as modified
			}
		} else {
			// Cache miss - file not in cache
			logging.LogDebugf("Cache miss for %s: not in cache", filepath.Base(filePath))
		}

		// Cache miss or expired - now check if file has assistant messages
		if !hasAssistantMessages(filePath) {
			// File has no assistant messages - create empty summary and cache it
			summary = createEmptySummaryForFile(absPath, filePath)
			// Return empty results
			return []models.UsageEntry{}, nil, false, "no_assistant_messages", nil, summary
		}
	}

	// Determine miss reason
	missReason := "other"
	if opts.EnableSummaryCache && opts.CacheStore != nil {
		if _, err := opts.CacheStore.GetFileSummary(absPath); err != nil {
			missReason = "new_file"
		} else {
			missReason = "modified_file"
		}
	}

	// Cache miss or caching disabled, process normally
	entries, rawEntries, err := processSingleFile(filePath, opts.Mode, cutoffTime, opts.IncludeRaw)
	if err != nil {
		return entries, rawEntries, false, missReason, err, nil
	}

	// If caching is enabled and we successfully processed the file, create and cache summary
	// Skip caching if in watch mode (TUI) to avoid frequent writes
	if opts.EnableSummaryCache && opts.CacheStore != nil && len(entries) > 0 && !opts.IsWatchMode {
		// Get file info if we don't have it yet
		if fileInfo, err := os.Stat(filePath); err == nil {
			summary = createSummaryFromEntries(absPath, filePath, entries, fileInfo)
		}
	}

	return entries, rawEntries, false, missReason, nil, summary
}

// processSingleFile processes a single JSONL file
func processSingleFile(filePath string, mode models.CostMode, cutoffTime *time.Time, includeRaw bool) ([]models.UsageEntry, []map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var entries []models.UsageEntry
	var rawEntries []map[string]interface{}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB max line size

	lineNumber := 0
	processedLines := 0
	skippedLines := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse JSON
		var data map[string]interface{}
		if err := sonic.Unmarshal([]byte(line), &data); err != nil {
			logging.LogDebugf("Skipping invalid JSON at line %d in %s: %v", lineNumber, filepath.Base(filePath), err)
			skippedLines++
			continue
		}

		// Include raw data if requested
		if includeRaw {
			rawEntries = append(rawEntries, data)
		}

		// Extract usage entry
		entry, hasUsage := extractUsageEntry(data)
		if !hasUsage {
			continue
		}

		// Apply time filter if specified
		if cutoffTime != nil && entry.Timestamp.Before(*cutoffTime) {
			continue
		}

		// Calculate cost based on mode
		pricing := models.GetPricing(entry.Model)
		entry.CostUSD = entry.CalculateCost(pricing)

		// Normalize model name
		entry.NormalizeModel()

		entries = append(entries, entry)
		processedLines++
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	if lineNumber > 0 && skippedLines > 0 {
		logging.LogDebugf("File %s: processed %d/%d lines, skipped %d invalid lines",
			filepath.Base(filePath), processedLines, lineNumber, skippedLines)
	}

	return entries, rawEntries, nil
}

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
