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
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
)

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
	GetFileSummary(absolutePath string) (*FileSummary, error)
	SetFileSummary(summary *FileSummary) error
	HasFileSummary(absolutePath string) bool
	InvalidateFileSummary(absolutePath string) error
}

// FileSummary represents a cached summary of a parsed usage file
// This is imported from cache package but defined here to avoid circular import
type FileSummary struct {
	Path            string
	AbsolutePath    string
	ModTime         time.Time
	FileSize        int64
	EntryCount      int
	TotalCost       float64
	TotalTokens     int
	ModelStats      map[string]FileSummaryModelStat
	HourlyBuckets   map[string]*FileSummaryTemporalBucket  // Hour-level aggregations (key: "2006-01-02 15")
	DailyBuckets    map[string]*FileSummaryTemporalBucket  // Day-level aggregations (key: "2006-01-02")
	ProcessedAt     time.Time
	Checksum        string
	ProcessedHashes map[string]bool
}

// FileSummaryTemporalBucket represents aggregated usage data for a specific time period
type FileSummaryTemporalBucket struct {
	Period      string                                  // The time period (e.g., "2006-01-02 15" for hour, "2006-01-02" for day)
	EntryCount  int
	TotalCost   float64
	TotalTokens int
	ModelStats  map[string]*FileSummaryModelStat  // Per-model statistics within this time bucket
}

// FileSummaryModelStat is used for file summary caching with additional fields
type FileSummaryModelStat struct {
	Model               string
	EntryCount          int
	TotalCost           float64
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}


// LoadUsageEntriesResult contains the loaded data
type LoadUsageEntriesResult struct {
	Entries    []models.UsageEntry      // Processed usage entries
	RawEntries []map[string]interface{} // Raw JSON data (if requested)
	Metadata   LoadMetadata             // Loading metadata
}

// LoadMetadata contains information about the loading process
type LoadMetadata struct {
	FilesProcessed   int           `json:"files_processed"`
	EntriesLoaded    int           `json:"entries_loaded"`
	EntriesFiltered  int           `json:"entries_filtered"`
	LoadDuration     time.Duration `json:"load_duration"`
	ProcessingErrors []string      `json:"processing_errors,omitempty"`
}

// LoadUsageEntries loads and converts JSONL files to UsageEntry objects
// This is equivalent to Claude Monitor's load_usage_entries() function
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

		// Calculate cache stats
		for _, result := range results {
			if result.Error == nil {
				if result.FromCache {
					cacheHits++
				} else {
					cacheMisses++
				}
			}
		}
	} else {
		// Use sequential loading for small file counts
		var processedHashes = make(map[string]bool) // For deduplication

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

			entries, rawEntries, fromCache, err := processSingleFileWithCache(filePath, opts, cutoffTime, processedHashes)
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
			}

			if i < 5 { // Log successful processing for first 5 files
				logging.LogDebugf("File %s processed: %d entries (from cache: %v)", filepath.Base(filePath), len(entries), fromCache)
			}

			allEntries = append(allEntries, entries...)
			if opts.IncludeRaw && rawEntries != nil {
				allRawEntries = append(allRawEntries, rawEntries...)
			}
		}
	}

	// Sort entries by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	loadDuration := time.Since(startTime)

	if opts.EnableSummaryCache && opts.CacheStore != nil {
		logging.LogInfof("Cache performance: %d hits, %d misses (%.1f%% hit rate)",
			cacheHits, cacheMisses, float64(cacheHits)/float64(cacheHits+cacheMisses)*100)
	}

	result := &LoadUsageEntriesResult{
		Entries:    allEntries,
		RawEntries: allRawEntries,
		Metadata: LoadMetadata{
			FilesProcessed:   len(jsonlFiles),
			EntriesLoaded:    len(allEntries),
			LoadDuration:     loadDuration,
			ProcessingErrors: processingErrors,
		},
	}

	return result, nil
}

// LoadAllRawEntries loads all raw JSONL entries without processing
func LoadAllRawEntries(dataPath string) ([]map[string]interface{}, error) {
	if dataPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dataPath = filepath.Join(homeDir, ".claude", "projects")
	}

	jsonlFiles, err := findJSONLFiles(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find JSONL files: %w", err)
	}

	var allRawEntries []map[string]interface{}

	for _, filePath := range jsonlFiles {
		rawEntries, err := loadRawEntriesFromFile(filePath)
		if err != nil {
			continue // Skip files with errors
		}
		allRawEntries = append(allRawEntries, rawEntries...)
	}

	return allRawEntries, nil
}

// findJSONLFiles finds all .jsonl files in the data directory
func findJSONLFiles(dataPath string) ([]string, error) {
	var jsonlFiles []string

	logging.LogDebugf("Searching for JSONL files in: %s", dataPath)

	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logging.LogWarnf("Error accessing path %s: %v", path, err)
			return nil // Continue walking even if we can't access some files
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".jsonl") {
			jsonlFiles = append(jsonlFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	logging.LogInfof("Found %d JSONL files total", len(jsonlFiles))
	return jsonlFiles, nil
}

// hasAssistantMessages quickly checks if a file contains assistant messages with usage data
// Uses simple string matching to avoid JSON parsing overhead
func hasAssistantMessages(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // Same buffer size as main processing

	// Check only the first few lines for efficiency
	for i := 0; i < 5 && scanner.Scan(); i++ {
		line := scanner.Text()
		// Quick string matching - look for assistant messages with usage data
		if strings.Contains(line, `"type":"assistant"`) &&
			(strings.Contains(line, `"usage"`) || strings.Contains(line, `"input_tokens"`)) {
			return true
		}
		// Also check for legacy format (no type field but has usage)
		if !strings.Contains(line, `"type":`) &&
			strings.Contains(line, `"usage"`) &&
			strings.Contains(line, `"input_tokens"`) {
			return true
		}
	}

	return false
}

// processSingleFileWithCache processes a single JSONL file with caching support
func processSingleFileWithCache(filePath string, opts LoadUsageEntriesOptions, cutoffTime *time.Time, processedHashes map[string]bool) ([]models.UsageEntry, []map[string]interface{}, bool, error) {
	// This is a wrapper for compatibility - it uses a regular map
	return processSingleFileWithCacheInternal(filePath, opts, cutoffTime, processedHashes, nil)
}

// processSingleFileWithCacheConcurrent processes a single JSONL file with caching support using sync.Map
func processSingleFileWithCacheConcurrent(filePath string, opts LoadUsageEntriesOptions, cutoffTime *time.Time, processedHashes *sync.Map) ([]models.UsageEntry, []map[string]interface{}, bool, error) {
	// This version uses sync.Map for concurrent access
	return processSingleFileWithCacheInternal(filePath, opts, cutoffTime, nil, processedHashes)
}

// processSingleFileWithCacheInternal is the internal implementation that supports both map types
func processSingleFileWithCacheInternal(filePath string, opts LoadUsageEntriesOptions, cutoffTime *time.Time, regularMap map[string]bool, syncMap *sync.Map) ([]models.UsageEntry, []map[string]interface{}, bool, error) {
	// Get absolute path for cache key
	absPath, absErr := filepath.Abs(filePath)
	if absErr != nil {
		absPath = filePath // fallback to relative path
	}

	// Check if caching is enabled
	if opts.EnableSummaryCache && opts.CacheStore != nil {
		// Quick pre-check: if file doesn't contain assistant messages, skip caching entirely
		if !hasAssistantMessages(filePath) {
			// Process directly without cache operations, avoiding unnecessary debug logs
			if regularMap != nil {
				entries, rawEntries, err := processSingleFile(filePath, opts.Mode, cutoffTime, regularMap, opts.IncludeRaw)
				return entries, rawEntries, false, err
			} else {
				entries, rawEntries, err := processSingleFileConcurrent(filePath, opts.Mode, cutoffTime, syncMap, opts.IncludeRaw)
				return entries, rawEntries, false, err
			}
		}

		// Get file info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			// File doesn't exist, fall back to normal processing
			if regularMap != nil {
				entries, rawEntries, err := processSingleFile(filePath, opts.Mode, cutoffTime, regularMap, opts.IncludeRaw)
				return entries, rawEntries, false, err
			} else {
				entries, rawEntries, err := processSingleFileConcurrent(filePath, opts.Mode, cutoffTime, syncMap, opts.IncludeRaw)
				return entries, rawEntries, false, err
			}
		}

		// Check if we have a cached summary
		if summary, err := opts.CacheStore.GetFileSummary(absPath); err == nil {
			// Check if cache is still valid based on file mtime and size
			if !summary.IsExpired(fileInfo.ModTime(), fileInfo.Size()) {
				// Use cached summary
				logging.LogDebugf("Cache hit: %s (mtime: %v, size: %d)", filepath.Base(filePath), fileInfo.ModTime(), fileInfo.Size())

				// Merge processed hashes to avoid duplicates
				if regularMap != nil {
					summary.MergeHashes(regularMap)
				} else {
					// For sync.Map, we need to merge differently
					for hash := range summary.ProcessedHashes {
						syncMap.Store(hash, true)
					}
				}

				// Create entries from summary, but filter out duplicates
				entries := createEntriesFromSummaryWithDedup(summary, cutoffTime, regularMap, syncMap)

				return entries, nil, true, nil
			} else {
				// File has been modified, invalidate cache
				logging.LogDebugf("Cache invalidated for %s: file modified (old mtime: %v, new mtime: %v, old size: %d, new size: %d)",
					filepath.Base(filePath), summary.ModTime, fileInfo.ModTime(), summary.FileSize, fileInfo.Size())
				opts.CacheStore.InvalidateFileSummary(absPath)
			}
		}
	}

	// Cache miss or caching disabled, process normally
	var entries []models.UsageEntry
	var rawEntries []map[string]interface{}
	var err error

	if regularMap != nil {
		entries, rawEntries, err = processSingleFile(filePath, opts.Mode, cutoffTime, regularMap, opts.IncludeRaw)
	} else {
		entries, rawEntries, err = processSingleFileConcurrent(filePath, opts.Mode, cutoffTime, syncMap, opts.IncludeRaw)
	}

	if err != nil {
		return entries, rawEntries, false, err
	}

	// If caching is enabled and we successfully processed the file, create and cache summary
	// Skip caching if in watch mode (TUI) to avoid frequent writes
	if opts.EnableSummaryCache && opts.CacheStore != nil && len(entries) > 0 && !opts.IsWatchMode {
		// Create summary with the appropriate hash map
		var summary *FileSummary
		if regularMap != nil {
			summary = createSummaryFromEntries(absPath, filePath, entries, regularMap)
		} else {
			// For sync.Map, we need to convert to regular map for storage
			tempMap := make(map[string]bool)
			syncMap.Range(func(key, value interface{}) bool {
				if hash, ok := key.(string); ok {
					tempMap[hash] = true
				}
				return true
			})
			summary = createSummaryFromEntries(absPath, filePath, entries, tempMap)
		}
		if summary != nil {
			if err := opts.CacheStore.SetFileSummary(summary); err != nil {
				logging.LogWarnf("Failed to cache summary for %s: %v", filepath.Base(filePath), err)
			} else {
				logging.LogDebugf("Cached summary for %s", filepath.Base(filePath))
			}
		}
	} else if opts.IsWatchMode && opts.EnableSummaryCache {
		logging.LogDebugf("Skipping cache write for %s (watch mode)", filepath.Base(filePath))
	}

	return entries, rawEntries, false, nil
}

// processSingleFile processes a single JSONL file
func processSingleFile(filePath string, mode models.CostMode, cutoffTime *time.Time, processedHashes map[string]bool, includeRaw bool) ([]models.UsageEntry, []map[string]interface{}, error) {
	// This is a wrapper for compatibility
	return processSingleFileInternal(filePath, mode, cutoffTime, processedHashes, nil, includeRaw)
}

// processSingleFileConcurrent processes a single JSONL file with sync.Map for concurrent access
func processSingleFileConcurrent(filePath string, mode models.CostMode, cutoffTime *time.Time, processedHashes *sync.Map, includeRaw bool) ([]models.UsageEntry, []map[string]interface{}, error) {
	// This version uses sync.Map
	return processSingleFileInternal(filePath, mode, cutoffTime, nil, processedHashes, includeRaw)
}

// processSingleFileInternal is the actual implementation supporting both map types
func processSingleFileInternal(filePath string, mode models.CostMode, cutoffTime *time.Time, regularMap map[string]bool, syncMap *sync.Map, includeRaw bool) ([]models.UsageEntry, []map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var entries []models.UsageEntry
	var rawEntries []map[string]interface{}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 200*1024*1024) // 64KB initial, 1MB max

	lineNum := 0
	processedLines := 0
	skippedByTime := 0
	conversionErrors := 0
	duplicates := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		var rawData map[string]interface{}
		if err := sonic.Unmarshal(line, &rawData); err != nil {
			continue // Skip invalid JSON lines
		}

		// Filter by timestamp if cutoff is specified
		if cutoffTime != nil {
			if timestampStr, ok := rawData["timestamp"].(string); ok {
				if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					if timestamp.Before(*cutoffTime) {
						skippedByTime++
						continue
					}
				}
			}
		}

		// Convert to UsageEntry
		entry, err := convertRawToUsageEntry(rawData, mode)
		if err != nil {
			conversionErrors++
			continue // Skip entries that can't be converted
		}

		// Deduplicate based on content hash
		entryHash := generateEntryHash(entry)

		// Check for duplicate using the appropriate map type
		isDuplicate := false
		if regularMap != nil {
			if regularMap[entryHash] {
				isDuplicate = true
			} else {
				regularMap[entryHash] = true
			}
		} else {
			// For sync.Map, use LoadOrStore for atomic check-and-set
			_, loaded := syncMap.LoadOrStore(entryHash, true)
			isDuplicate = loaded
		}

		if isDuplicate {
			duplicates++
			continue
		}

		// Normalize model name
		entry.NormalizeModel()

		entries = append(entries, entry)
		processedLines++

		if includeRaw {
			rawEntries = append(rawEntries, rawData)
		}
	}

	if err := scanner.Err(); err != nil {
		return entries, rawEntries, fmt.Errorf("scanner error: %w", err)
	}

	return entries, rawEntries, nil
}

// loadRawEntriesFromFile loads raw entries from a single file
func loadRawEntriesFromFile(filePath string) ([]map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var rawEntries []map[string]interface{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rawData map[string]interface{}
		if err := sonic.Unmarshal(line, &rawData); err != nil {
			continue
		}

		rawEntries = append(rawEntries, rawData)
	}

	return rawEntries, scanner.Err()
}

// convertRawToUsageEntry converts raw JSON data to a UsageEntry
func convertRawToUsageEntry(rawData map[string]interface{}, mode models.CostMode) (models.UsageEntry, error) {
	var entry models.UsageEntry

	// Try to unmarshal into ConversationLog struct first
	jsonBytes, err := sonic.Marshal(rawData)
	if err != nil {
		return entry, fmt.Errorf("failed to re-marshal raw data: %w", err)
	}

	var convLog models.ConversationLog
	if err := sonic.Unmarshal(jsonBytes, &convLog); err != nil {
		// If it fails, fall back to legacy format parsing
		return convertLegacyFormat(rawData, mode)
	}

	// Check if this is actually a ConversationLog format (has type field)
	if convLog.Type == "" {
		// No type field, treat as legacy format
		return convertLegacyFormat(rawData, mode)
	}

	// Only process assistant messages for usage data
	if convLog.Type != "assistant" {
		return entry, fmt.Errorf("not an assistant message")
	}

	// Extract timestamp
	if convLog.Timestamp != "" {
		timestamp, err := time.Parse(time.RFC3339, convLog.Timestamp)
		if err != nil {
			return entry, fmt.Errorf("invalid timestamp format: %w", err)
		}
		entry.Timestamp = timestamp
	} else {
		return entry, fmt.Errorf("missing timestamp")
	}

	// Extract model and usage data from message
	if convLog.Message.Model == "" {
		return entry, fmt.Errorf("missing model in message")
	}
	entry.Model = convLog.Message.Model

	// Extract usage data
	usage := convLog.Message.Usage
	entry.InputTokens = usage.InputTokens
	entry.OutputTokens = usage.OutputTokens
	entry.CacheCreationTokens = usage.CacheCreationInputTokens
	entry.CacheReadTokens = usage.CacheReadInputTokens

	// Extract IDs
	entry.MessageID = convLog.Message.Id
	entry.RequestID = convLog.RequestId
	entry.SessionID = convLog.SessionId

	// Calculate total tokens
	entry.TotalTokens = entry.CalculateTotalTokens()

	// Calculate cost based on mode
	switch mode {
	case models.CostModeCalculated, models.CostModeAuto:
		pricing := models.GetPricing(entry.Model)
		entry.CostUSD = entry.CalculateCost(pricing)
	case models.CostModeCached:
		// For cached mode, still calculate (no cached cost in ConversationLog)
		pricing := models.GetPricing(entry.Model)
		entry.CostUSD = entry.CalculateCost(pricing)
	}

	// Validate the entry
	if err := entry.Validate(); err != nil {
		return entry, fmt.Errorf("entry validation failed: %w", err)
	}

	return entry, nil
}

// convertLegacyFormat handles the legacy format parsing
func convertLegacyFormat(rawData map[string]interface{}, mode models.CostMode) (models.UsageEntry, error) {
	var entry models.UsageEntry

	// Extract timestamp
	if timestampStr, ok := rawData["timestamp"].(string); ok {
		if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			entry.Timestamp = timestamp
		} else {
			return entry, fmt.Errorf("invalid timestamp format")
		}
	} else {
		return entry, fmt.Errorf("missing timestamp")
	}

	// Extract model
	if model, ok := rawData["model"].(string); ok {
		entry.Model = model
	} else {
		return entry, fmt.Errorf("missing model")
	}

	// Extract usage data
	if usageData, ok := rawData["usage"].(map[string]interface{}); ok {
		if inputTokens, ok := usageData["input_tokens"].(float64); ok {
			entry.InputTokens = int(inputTokens)
		}
		if outputTokens, ok := usageData["output_tokens"].(float64); ok {
			entry.OutputTokens = int(outputTokens)
		}
		if cacheCreationTokens, ok := usageData["cache_creation_tokens"].(float64); ok {
			entry.CacheCreationTokens = int(cacheCreationTokens)
		}
		if cacheReadTokens, ok := usageData["cache_read_tokens"].(float64); ok {
			entry.CacheReadTokens = int(cacheReadTokens)
		}
	}

	// Extract IDs if available
	if messageID, ok := rawData["message_id"].(string); ok {
		entry.MessageID = messageID
	}
	if requestID, ok := rawData["request_id"].(string); ok {
		entry.RequestID = requestID
	}

	// Calculate total tokens
	entry.TotalTokens = entry.CalculateTotalTokens()

	// Calculate cost based on mode
	switch mode {
	case models.CostModeCalculated, models.CostModeAuto:
		pricing := models.GetPricing(entry.Model)
		entry.CostUSD = entry.CalculateCost(pricing)
	case models.CostModeCached:
		if costUSD, ok := rawData["cost_usd"].(float64); ok {
			entry.CostUSD = costUSD
		} else {
			// Fallback to calculation if cached cost not available
			pricing := models.GetPricing(entry.Model)
			entry.CostUSD = entry.CalculateCost(pricing)
		}
	}

	// Validate the entry
	if err := entry.Validate(); err != nil {
		return entry, fmt.Errorf("entry validation failed: %w", err)
	}

	return entry, nil
}

// generateEntryHash generates a hash for deduplication
func generateEntryHash(entry models.UsageEntry) string {
	hashData := fmt.Sprintf("%s-%s-%d-%d-%d-%d",
		entry.Timestamp.Format(time.RFC3339),
		entry.Model,
		entry.InputTokens,
		entry.OutputTokens,
		entry.CacheCreationTokens,
		entry.CacheReadTokens,
	)

	hash := md5.Sum([]byte(hashData))
	return fmt.Sprintf("%x", hash)
}

// createEntriesFromSummary creates placeholder entries from a cached summary
// Note: This is a simplified approach that creates aggregate entries for cache hits
// createEntriesFromSummaryWithDedup creates entries from summary with deduplication
func createEntriesFromSummaryWithDedup(summary *FileSummary, cutoffTime *time.Time, regularMap map[string]bool, syncMap *sync.Map) []models.UsageEntry {
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
					// FIXED: Create individual synthetic entries to preserve granularity
					// Instead of 1 aggregated entry, create EntryCount individual entries
					// This preserves the expected entry count for analyze command
					
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
						
						// Create individual synthetic entry
						entry := models.UsageEntry{
							Timestamp:           hourTime.Add(time.Duration(i) * time.Minute), // Spread across hour
							Model:               modelStat.Model,
							InputTokens:         inputTokens,
							OutputTokens:        outputTokens,
							CacheCreationTokens: cacheCreationTokens,
							CacheReadTokens:     cacheReadTokens,
							TotalTokens:         inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
							CostUSD:             avgCostUSD,
							MessageID:           fmt.Sprintf("cached_%s_%s_%s_%d", modelStat.Model, hourKey, summary.Checksum[:8], i),
						}

						entry.NormalizeModel()

						// Check if this entry would be a duplicate
						entryHash := generateEntryHash(entry)
						isDuplicate := false

						if regularMap != nil {
							if regularMap[entryHash] {
								isDuplicate = true
							}
						} else if syncMap != nil {
							_, loaded := syncMap.Load(entryHash)
							isDuplicate = loaded
						}

						if !isDuplicate {
							entries = append(entries, entry)
						}
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
					// FIXED: Create individual synthetic entries to preserve granularity
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
						
						// Create individual synthetic entry
						entry := models.UsageEntry{
							Timestamp:           dayTime.Add(time.Duration(i) * time.Hour), // Spread across day
							Model:               modelStat.Model,
							InputTokens:         inputTokens,
							OutputTokens:        outputTokens,
							CacheCreationTokens: cacheCreationTokens,
							CacheReadTokens:     cacheReadTokens,
							TotalTokens:         inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
							CostUSD:             avgCostUSD,
							MessageID:           fmt.Sprintf("cached_%s_%s_%s_%d", modelStat.Model, dayKey, summary.Checksum[:8], i),
						}

						entry.NormalizeModel()

						// Check if this entry would be a duplicate
						entryHash := generateEntryHash(entry)
						isDuplicate := false

						if regularMap != nil {
							if regularMap[entryHash] {
								isDuplicate = true
							}
						} else if syncMap != nil {
							_, loaded := syncMap.Load(entryHash)
							isDuplicate = loaded
						}

						if !isDuplicate {
							entries = append(entries, entry)
						}
					}
				}
			}
		}
	} else {
		// Fallback to old behavior if no temporal buckets (for backward compatibility)
		for modelName, modelStat := range summary.ModelStats {
			if modelStat.EntryCount > 0 {
				// Create a representative entry for this model
				entry := models.UsageEntry{
					Timestamp:           summary.ProcessedAt, // Use processed time as timestamp
					Model:               modelStat.Model,
					InputTokens:         modelStat.InputTokens,
					OutputTokens:        modelStat.OutputTokens,
					CacheCreationTokens: modelStat.CacheCreationTokens,
					CacheReadTokens:     modelStat.CacheReadTokens,
					TotalTokens:         modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
					CostUSD:             modelStat.TotalCost,
					MessageID:           fmt.Sprintf("cached_%s_%d", modelName, summary.ProcessedAt.Unix()),
				}

				entry.NormalizeModel()

				// Check if this entry would be a duplicate
				entryHash := generateEntryHash(entry)
				isDuplicate := false

				if regularMap != nil {
					if regularMap[entryHash] {
						isDuplicate = true
					}
				} else if syncMap != nil {
					_, loaded := syncMap.Load(entryHash)
					isDuplicate = loaded
				}

				if !isDuplicate {
					entries = append(entries, entry)
				}
			}
		}
	}

	return entries
}

func createEntriesFromSummary(summary *FileSummary, cutoffTime *time.Time) []models.UsageEntry {
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
					// Create a representative entry for this model in this hour
					entry := models.UsageEntry{
						Timestamp:           hourTime, // Use the hour timestamp
						Model:               modelStat.Model,
						InputTokens:         modelStat.InputTokens,
						OutputTokens:        modelStat.OutputTokens,
						CacheCreationTokens: modelStat.CacheCreationTokens,
						CacheReadTokens:     modelStat.CacheReadTokens,
						TotalTokens:         modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
						CostUSD:             modelStat.TotalCost,
						MessageID:           fmt.Sprintf("cached_%s_%s_%s", modelStat.Model, hourKey, summary.Checksum[:8]),
					}

					entry.NormalizeModel()
					entries = append(entries, entry)
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
					// Create a representative entry for this model in this day
					entry := models.UsageEntry{
						Timestamp:           dayTime, // Use the day timestamp
						Model:               modelStat.Model,
						InputTokens:         modelStat.InputTokens,
						OutputTokens:        modelStat.OutputTokens,
						CacheCreationTokens: modelStat.CacheCreationTokens,
						CacheReadTokens:     modelStat.CacheReadTokens,
						TotalTokens:         modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
						CostUSD:             modelStat.TotalCost,
						MessageID:           fmt.Sprintf("cached_%s_%s_%s", modelStat.Model, dayKey, summary.Checksum[:8]),
					}

					entry.NormalizeModel()
					entries = append(entries, entry)
				}
			}
		}
	} else {
		// Fallback to old behavior if no temporal buckets (for backward compatibility)
		for modelName, modelStat := range summary.ModelStats {
			if modelStat.EntryCount > 0 {
				// Create a representative entry for this model
				entry := models.UsageEntry{
					Timestamp:           summary.ProcessedAt, // Use processed time as timestamp
					Model:               modelStat.Model,
					InputTokens:         modelStat.InputTokens,
					OutputTokens:        modelStat.OutputTokens,
					CacheCreationTokens: modelStat.CacheCreationTokens,
					CacheReadTokens:     modelStat.CacheReadTokens,
					TotalTokens:         modelStat.InputTokens + modelStat.OutputTokens + modelStat.CacheCreationTokens + modelStat.CacheReadTokens,
					CostUSD:             modelStat.TotalCost,
					MessageID:           fmt.Sprintf("cached_%s_%d", modelName, summary.ProcessedAt.Unix()),
				}

				entry.NormalizeModel()
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

// createSummaryFromEntries creates a FileSummary from processed entries
func createSummaryFromEntries(absPath, relPath string, entries []models.UsageEntry, processedHashes map[string]bool) *FileSummary {
	if len(entries) == 0 {
		return nil
	}

	// Get file info
	fileInfo, err := os.Stat(relPath)
	if err != nil {
		return nil
	}

	// Initialize summary
	summary := &FileSummary{
		Path:            relPath,
		AbsolutePath:    absPath,
		ModTime:         fileInfo.ModTime(),
		FileSize:        fileInfo.Size(),
		EntryCount:      len(entries),
		ProcessedAt:     time.Now(),
		ModelStats:      make(map[string]FileSummaryModelStat),
		HourlyBuckets:   make(map[string]*FileSummaryTemporalBucket),
		DailyBuckets:    make(map[string]*FileSummaryTemporalBucket),
		ProcessedHashes: make(map[string]bool),
	}

	// Copy processed hashes
	for hash := range processedHashes {
		summary.ProcessedHashes[hash] = true
	}

	// Calculate checksum (simple approach based on file mod time and size)
	summary.Checksum = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s_%d_%d",
		absPath, fileInfo.ModTime().Unix(), fileInfo.Size()))))

	// Process entries to create statistics
	var totalCost float64
	var totalTokens int
	var startTime, endTime time.Time

	for i, entry := range entries {
		totalCost += entry.CostUSD
		totalTokens += entry.TotalTokens

		// Track time range
		if i == 0 {
			startTime = entry.Timestamp
			endTime = entry.Timestamp
		} else {
			if entry.Timestamp.Before(startTime) {
				startTime = entry.Timestamp
			}
			if entry.Timestamp.After(endTime) {
				endTime = entry.Timestamp
			}
		}

		// Update model statistics
		modelStat, exists := summary.ModelStats[entry.Model]
		if !exists {
			modelStat = FileSummaryModelStat{
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
			hourBucket = &FileSummaryTemporalBucket{
				Period:     hourKey,
				ModelStats: make(map[string]*FileSummaryModelStat),
			}
			summary.HourlyBuckets[hourKey] = hourBucket
		}

		hourBucket.EntryCount++
		hourBucket.TotalCost += entry.CostUSD
		hourBucket.TotalTokens += entry.TotalTokens

		// Update model stats within hourly bucket
		hourModelStat, exists := hourBucket.ModelStats[entry.Model]
		if !exists {
			hourModelStat = &FileSummaryModelStat{
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
			dayBucket = &FileSummaryTemporalBucket{
				Period:     dayKey,
				ModelStats: make(map[string]*FileSummaryModelStat),
			}
			summary.DailyBuckets[dayKey] = dayBucket
		}

		dayBucket.EntryCount++
		dayBucket.TotalCost += entry.CostUSD
		dayBucket.TotalTokens += entry.TotalTokens

		// Update model stats within daily bucket
		dayModelStat, exists := dayBucket.ModelStats[entry.Model]
		if !exists {
			dayModelStat = &FileSummaryModelStat{
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

// IsExpired checks if the summary is expired based on file modification time or size
func (fs *FileSummary) IsExpired(currentModTime time.Time, currentSize int64) bool {
	return !fs.ModTime.Equal(currentModTime) || fs.FileSize != currentSize
}

// MergeHashes merges processed hashes from summary into the target map
func (fs *FileSummary) MergeHashes(target map[string]bool) {
	for hash := range fs.ProcessedHashes {
		target[hash] = true
	}
}
