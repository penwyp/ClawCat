package internal

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
)

// Analyzer provides data analysis functionality
type Analyzer struct {
	config *config.Config
	logger logging.LoggerInterface
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(cfg *config.Config) (*Analyzer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	// Use debug console logging if debug mode is enabled
	debugToConsole := cfg.Debug.Enabled
	
	return &Analyzer{
		config: cfg,
		logger: logging.NewLoggerWithDebug(cfg.App.LogLevel, cfg.App.LogFile, debugToConsole),
	}, nil
}

// Analyze performs analysis on the specified data paths
func (a *Analyzer) Analyze(paths []string) ([]models.AnalysisResult, error) {
	if len(paths) == 0 {
		paths = a.config.Data.Paths
	}

	// If still no paths, use default path
	if len(paths) == 0 {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultPath := fmt.Sprintf("%s/.claude/projects", homeDir)
			if _, err := os.Stat(defaultPath); err == nil {
				paths = []string{defaultPath}
				a.logger.Infof("Using default data path: %s", defaultPath)
			}
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no data paths found - please specify paths as arguments (e.g., clawcat analyze ~/claude-logs) or ensure ~/.claude/projects exists")
	}

	a.logger.Infof("Starting analysis of %d paths: %v", len(paths), paths)

	var allResults []models.AnalysisResult
	var analyzedPaths []string
	var errorPaths []string

	for _, path := range paths {
		a.logger.Infof("Analyzing path: %s", path)
		results, err := a.analyzePath(path)
		if err != nil {
			a.logger.Errorf("Failed to analyze path %s: %v", path, err)
			errorPaths = append(errorPaths, path)
			continue
		}

		if len(results) > 0 {
			analyzedPaths = append(analyzedPaths, path)
			a.logger.Infof("Found %d usage entries in path: %s", len(results), path)
		} else {
			a.logger.Warnf("No usage data found in path: %s", path)
		}

		allResults = append(allResults, results...)
	}

	// Sort results by timestamp
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	if len(allResults) == 0 {
		if len(errorPaths) > 0 {
			return nil, fmt.Errorf("no usage data found - %d paths had errors: %v", len(errorPaths), errorPaths)
		}
		return nil, fmt.Errorf("no usage data found in any of the specified paths: %v\n\nExpected data format:\n- JSONL files with usage data\n- Files should contain either 'type: message' with usage field, or 'type: assistant' with message.usage field\n- Check that the paths contain Claude conversation or API usage logs", paths)
	}

	a.logger.Infof("Analysis completed: %d results from %d paths", len(allResults), len(analyzedPaths))
	return allResults, nil
}

// analyzePath analyzes a single path (file or directory)
func (a *Analyzer) analyzePath(path string) ([]models.AnalysisResult, error) {
	// Discover files in the path
	files, err := fileio.DiscoverFiles(path)
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %w", err)
	}

	var allResults []models.AnalysisResult

	for _, file := range files {
		results, err := a.analyzeFile(file)
		if err != nil {
			a.logger.Errorf("Failed to analyze file %s: %v", file, err)
			continue
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// analyzeFile analyzes a single file
func (a *Analyzer) analyzeFile(filePath string) ([]models.AnalysisResult, error) {
	reader, err := fileio.NewReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	var results []models.AnalysisResult
	entryCh, errCh := reader.ReadEntries()

	for {
		select {
		case entry, ok := <-entryCh:
			if !ok {
				// Channel closed, we're done
				return results, nil
			}

			// Calculate cost and total tokens if not already calculated
			if entry.CostUSD == 0 {
				pricing := models.GetPricing(entry.Model)
				entry.CostUSD = entry.CalculateCost(pricing)
			}
			if entry.TotalTokens == 0 {
				entry.TotalTokens = entry.CalculateTotalTokens()
			}

			// Convert to analysis result
			result := models.AnalysisResult{
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
			}

			results = append(results, result)

		case err := <-errCh:
			if err != nil {
				return results, fmt.Errorf("error reading entries: %w", err)
			}
		}
	}
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
