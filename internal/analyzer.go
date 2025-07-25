package internal

import (
	"fmt"
	"sort"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/models"
)

// Analyzer provides data analysis functionality
type Analyzer struct {
	config *config.Config
	logger *Logger
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(cfg *config.Config) (*Analyzer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	
	return &Analyzer{
		config: cfg,
		logger: NewLogger(cfg.App.LogLevel, cfg.App.LogFile),
	}, nil
}

// Analyze performs analysis on the specified data paths
func (a *Analyzer) Analyze(paths []string) ([]models.AnalysisResult, error) {
	if len(paths) == 0 {
		paths = a.config.Data.Paths
	}
	
	a.logger.Infof("Starting analysis of %d paths", len(paths))
	
	var allResults []models.AnalysisResult
	
	for _, path := range paths {
		results, err := a.analyzePath(path)
		if err != nil {
			a.logger.Errorf("Failed to analyze path %s: %v", path, err)
			continue
		}
		
		allResults = append(allResults, results...)
	}
	
	// Sort results by timestamp
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})
	
	a.logger.Infof("Analysis completed: %d results", len(allResults))
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
		StartTime: results[0].Timestamp,
		EndTime:   results[len(results)-1].Timestamp,
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