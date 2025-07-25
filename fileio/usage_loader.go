package fileio

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/penwyp/ClawCat/models"
)

// LoadUsageEntriesOptions configures the usage loading behavior
type LoadUsageEntriesOptions struct {
	DataPath    string            // Path to Claude data directory
	HoursBack   *int              // Only include entries from last N hours (nil = all data)
	Mode        models.CostMode   // Cost calculation mode
	IncludeRaw  bool              // Whether to return raw JSON data alongside entries
}

// LoadUsageEntriesResult contains the loaded data
type LoadUsageEntriesResult struct {
	Entries    []models.UsageEntry     // Processed usage entries
	RawEntries []map[string]interface{} // Raw JSON data (if requested)
	Metadata   LoadMetadata             // Loading metadata
}

// LoadMetadata contains information about the loading process
type LoadMetadata struct {
	FilesProcessed    int           `json:"files_processed"`
	EntriesLoaded     int           `json:"entries_loaded"`
	EntriesFiltered   int           `json:"entries_filtered"`
	LoadDuration      time.Duration `json:"load_duration"`
	ProcessingErrors  []string      `json:"processing_errors,omitempty"`
}

// LoadUsageEntries loads and converts JSONL files to UsageEntry objects
// This is equivalent to Claude Monitor's load_usage_entries() function
func LoadUsageEntries(opts LoadUsageEntriesOptions) (*LoadUsageEntriesResult, error) {
	startTime := time.Now()
	
	// Default data path if not provided
	if opts.DataPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		opts.DataPath = filepath.Join(homeDir, ".claude", "projects")
	}

	// Calculate cutoff time if specified
	var cutoffTime *time.Time
	if opts.HoursBack != nil {
		cutoff := time.Now().UTC().Add(-time.Duration(*opts.HoursBack) * time.Hour)
		cutoffTime = &cutoff
	}

	// Find all JSONL files
	jsonlFiles, err := findJSONLFiles(opts.DataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find JSONL files: %w", err)
	}

	// Process all files
	var allEntries []models.UsageEntry
	var allRawEntries []map[string]interface{}
	var processedHashes = make(map[string]bool) // For deduplication
	var processingErrors []string

	for _, filePath := range jsonlFiles {
		entries, rawEntries, err := processSingleFile(filePath, opts.Mode, cutoffTime, processedHashes, opts.IncludeRaw)
		if err != nil {
			processingErrors = append(processingErrors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}
		
		allEntries = append(allEntries, entries...)
		if opts.IncludeRaw && rawEntries != nil {
			allRawEntries = append(allRawEntries, rawEntries...)
		}
	}

	// Sort entries by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	loadDuration := time.Since(startTime)

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
	
	err := filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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
	
	return jsonlFiles, nil
}

// processSingleFile processes a single JSONL file
func processSingleFile(filePath string, mode models.CostMode, cutoffTime *time.Time, processedHashes map[string]bool, includeRaw bool) ([]models.UsageEntry, []map[string]interface{}, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var entries []models.UsageEntry
	var rawEntries []map[string]interface{}
	
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 64KB initial, 1MB max
	
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		
		if len(line) == 0 {
			continue
		}
		
		var rawData map[string]interface{}
		if err := json.Unmarshal(line, &rawData); err != nil {
			continue // Skip invalid JSON lines
		}
		
		// Filter by timestamp if cutoff is specified
		if cutoffTime != nil {
			if timestampStr, ok := rawData["timestamp"].(string); ok {
				if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					if timestamp.Before(*cutoffTime) {
						continue
					}
				}
			}
		}
		
		// Convert to UsageEntry
		entry, err := convertRawToUsageEntry(rawData, mode)
		if err != nil {
			continue // Skip entries that can't be converted
		}
		
		// Deduplicate based on content hash
		entryHash := generateEntryHash(entry)
		if processedHashes[entryHash] {
			continue
		}
		processedHashes[entryHash] = true
		
		// Normalize model name
		entry.NormalizeModel()
		
		entries = append(entries, entry)
		
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
		if err := json.Unmarshal(line, &rawData); err != nil {
			continue
		}
		
		rawEntries = append(rawEntries, rawData)
	}
	
	return rawEntries, scanner.Err()
}

// convertRawToUsageEntry converts raw JSON data to a UsageEntry
func convertRawToUsageEntry(rawData map[string]interface{}, mode models.CostMode) (models.UsageEntry, error) {
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
	hashData := fmt.Sprintf("%s-%s-%d-%d-%d-%d-%f",
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