package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/fileio"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
	"github.com/penwyp/claudecat/models/pricing"
)

// Analyzer provides data analysis functionality
type Analyzer struct {
	config *config.Config
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(cfg *config.Config) (*Analyzer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	return &Analyzer{
		config: cfg,
	}, nil
}

// Analyze performs analysis on the specified data paths
func (a *Analyzer) Analyze(paths []string) ([]models.AnalysisResult, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no data paths found - please specify paths as arguments (e.g., claudecat analyze ~/claude-logs) or ensure ~/.claude/projects exists")
	}

	logging.LogInfof("Starting analysis of %d paths: %v", len(paths), paths)

	// Expand cache directory path for use in both cache and pricing
	cacheDir := a.config.Cache.Dir
	if cacheDir != "" && cacheDir[:2] == "~/" {
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, cacheDir[2:])
	}

	// Create BadgerDB cache store if caching is enabled
	var cacheStore fileio.CacheStore
	if a.config.Cache.Enabled && a.config.Data.SummaryCache.Enabled {
		// Use file-based cache with memory preloading
		fileCache, err := cache.NewFileBasedSummaryCache(cacheDir)
		if err != nil {
			logging.LogErrorf("Failed to create file-based cache: %v", err)
			// Cache is disabled on error
		} else {
			cacheStore = fileCache
		}
	}

	// Create pricing provider
	pricingProvider, err := pricing.CreatePricingProvider(&a.config.Data, cacheDir)
	if err != nil {
		logging.LogErrorf("Failed to create pricing provider: %v", err)
		// Fall back to default provider
		pricingProvider = pricing.NewDefaultProvider()
	}

	var allResults []models.AnalysisResult
	for _, path := range paths {
		// Use LoadUsageEntries with caching support
		opts := fileio.LoadUsageEntriesOptions{
			DataPath:            path,
			Mode:                models.CostModeCalculated,
			CacheStore:          cacheStore,
			EnableSummaryCache:  a.config.Data.SummaryCache.Enabled,
			EnableDeduplication: a.config.Data.Deduplication,
			PricingProvider:     pricingProvider,
		}

		result, err := fileio.LoadUsageEntries(opts)
		if err != nil {
			logging.LogErrorf("Failed to load usage entries from %s: %v", path, err)
			continue
		}

		// Convert usage entries to analysis results
		for _, entry := range result.Entries {
			analysisResult := models.AnalysisResult{
				Timestamp:           entry.Timestamp,
				Model:               entry.Model,
				SessionID:           a.generateSessionID(entry.Timestamp),
				InputTokens:         entry.InputTokens,
				OutputTokens:        entry.OutputTokens,
				CacheCreationTokens: entry.CacheCreationTokens,
				CacheReadTokens:     entry.CacheReadTokens,
				TotalTokens:         entry.TotalTokens,
				CostUSD:             entry.CostUSD,
				Count:               1,
				Project:             entry.Project,
			}
			allResults = append(allResults, analysisResult)
		}

		logging.LogInfof("Processed %d entries from %s (files: %d, errors: %d)",
			result.Metadata.EntriesLoaded, path,
			result.Metadata.FilesProcessed,
			len(result.Metadata.ProcessingErrors))
	}

	// Sort results by timestamp
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	if len(allResults) == 0 {
		return nil, fmt.Errorf("no usage data found in any of the specified paths: %v\n\nExpected data format:\n- JSONL files with usage data\n- Files should contain either 'type: message' with usage field, or 'type: assistant' with message.usage field\n- Check that the paths contain Claude conversation or API usage logs", paths)
	}

	logging.LogInfof("Analysis completed: %d results from %d paths", len(allResults), len(paths))
	return allResults, nil
}

// generateSessionID generates a session ID based on timestamp
func (a *Analyzer) generateSessionID(timestamp time.Time) string {
	// Simple session ID generation - group by 5-hour blocks
	sessionStart := timestamp.Truncate(5 * time.Hour)
	return fmt.Sprintf("session_%s", sessionStart.Format("2006-01-02_15"))
}

// GetSummaryStats returns summary statistics for the results
func (a *Analyzer) GetSummaryStats(results []models.AnalysisResult) models.SummaryStats {
	if len(results) == 0 {
		return models.SummaryStats{}
	}

	stats := models.SummaryStats{
		StartTime:   results[0].Timestamp,
		EndTime:     results[len(results)-1].Timestamp,
		ModelCounts: make(map[string]int),
	}

	for _, result := range results {
		stats.TotalEntries++
		stats.TotalTokens += result.TotalTokens
		stats.TotalCost += result.CostUSD
		stats.InputTokens += result.InputTokens
		stats.OutputTokens += result.OutputTokens
		stats.CacheCreationTokens += result.CacheCreationTokens
		stats.CacheReadTokens += result.CacheReadTokens

		stats.ModelCounts[result.Model]++

		if result.CostUSD > stats.MaxCost {
			stats.MaxCost = result.CostUSD
		}
		if result.TotalTokens > stats.MaxTokens {
			stats.MaxTokens = result.TotalTokens
		}
	}

	// Calculate averages
	if stats.TotalEntries > 0 {
		stats.AvgCost = stats.TotalCost / float64(stats.TotalEntries)
		stats.AvgTokens = float64(stats.TotalTokens) / float64(stats.TotalEntries)
	}

	return stats
}
