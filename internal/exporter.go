package internal

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
)

// Exporter provides data export functionality
type Exporter struct {
	config *config.Config
	logger logging.LoggerInterface
}

// ExportOptions contains export configuration
type ExportOptions struct {
	Format     string
	TimeRange  string
	FromTime   string
	ToTime     string
	Aggregate  bool
	Compress   bool
	Overwrite  bool
	Template   string
	OutputFile string
}

// ExportResult contains the results of an export operation
type ExportResult struct {
	OutputFile  string
	Format      string
	RecordCount int
	FileSize    int64
	Compressed  bool
	Duration    time.Duration
	Error       error
}

// NewExporter creates a new exporter instance
func NewExporter(cfg *config.Config) (*Exporter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	return &Exporter{
		config: cfg,
		logger: logging.NewLogger(cfg.App.LogLevel, cfg.App.LogFile),
	}, nil
}

// Export performs data export based on the provided options
func (e *Exporter) Export(options ExportOptions) (*ExportResult, error) {
	startTime := time.Now()

	result := &ExportResult{
		OutputFile: options.OutputFile,
		Format:     options.Format,
		Compressed: options.Compress,
	}

	// Create analyzer to get the data
	analyzer, err := NewAnalyzer(e.config)
	if err != nil {
		result.Error = fmt.Errorf("failed to create analyzer: %w", err)
		return result, result.Error
	}

	// Get data to export
	data, err := analyzer.Analyze(e.config.Data.Paths)
	if err != nil {
		result.Error = fmt.Errorf("failed to analyze data: %w", err)
		return result, result.Error
	}

	// Apply time range filtering
	data = e.filterByTimeRange(data, options)

	// Apply aggregation if requested
	if options.Aggregate {
		data = e.aggregateData(data)
	}

	// Export based on format
	switch options.Format {
	case "csv":
		err = e.exportCSV(data, options)
	case "json":
		err = e.exportJSON(data, options)
	case "xlsx":
		err = e.exportXLSX(data, options)
	case "parquet":
		err = e.exportParquet(data, options)
	default:
		err = fmt.Errorf("unsupported export format: %s", options.Format)
	}

	if err != nil {
		result.Error = err
		return result, err
	}

	// Get file info
	fileInfo, err := os.Stat(options.OutputFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to get file info: %w", err)
		return result, result.Error
	}

	result.RecordCount = len(data)
	result.FileSize = fileInfo.Size()
	result.Duration = time.Since(startTime)

	e.logger.Infof("Export completed: %d records, %d bytes, %v",
		result.RecordCount, result.FileSize, result.Duration)

	return result, nil
}

// filterByTimeRange filters data based on time range options
func (e *Exporter) filterByTimeRange(data []models.AnalysisResult, options ExportOptions) []models.AnalysisResult {
	if options.TimeRange == "all" && options.FromTime == "" && options.ToTime == "" {
		return data
	}

	var fromTime, toTime time.Time
	var err error

	// Handle predefined ranges
	now := time.Now()
	switch options.TimeRange {
	case "today":
		fromTime = now.Truncate(24 * time.Hour)
		toTime = now
	case "week":
		fromTime = now.AddDate(0, 0, -7)
		toTime = now
	case "month":
		fromTime = now.AddDate(0, -1, 0)
		toTime = now
	case "year":
		fromTime = now.AddDate(-1, 0, 0)
		toTime = now
	case "custom":
		// Use FromTime and ToTime from options
		if options.FromTime != "" {
			fromTime, err = parseTimeString(options.FromTime)
			if err != nil {
				e.logger.Warnf("Invalid from time: %v", err)
				return data
			}
		}
		if options.ToTime != "" {
			toTime, err = parseTimeString(options.ToTime)
			if err != nil {
				e.logger.Warnf("Invalid to time: %v", err)
				return data
			}
		}
	}

	// Filter data
	var filtered []models.AnalysisResult
	for _, item := range data {
		if !fromTime.IsZero() && item.Timestamp.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && item.Timestamp.After(toTime) {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered
}

// aggregateData aggregates data by session
func (e *Exporter) aggregateData(data []models.AnalysisResult) []models.AnalysisResult {
	sessions := make(map[string]models.AnalysisResult)

	for _, item := range data {
		if existing, exists := sessions[item.SessionID]; exists {
			// Aggregate with existing
			existing.InputTokens += item.InputTokens
			existing.OutputTokens += item.OutputTokens
			existing.CacheCreationTokens += item.CacheCreationTokens
			existing.CacheReadTokens += item.CacheReadTokens
			existing.TotalTokens += item.TotalTokens
			existing.CostUSD += item.CostUSD
			existing.Count += item.Count
			sessions[item.SessionID] = existing
		} else {
			// First entry for this session
			sessions[item.SessionID] = item
		}
	}

	// Convert back to slice
	var aggregated []models.AnalysisResult
	for _, item := range sessions {
		aggregated = append(aggregated, item)
	}

	return aggregated
}

// exportCSV exports data to CSV format
func (e *Exporter) exportCSV(data []models.AnalysisResult, options ExportOptions) error {
	file, err := e.createOutputFile(options.OutputFile, options.Compress)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp", "Model", "Session ID", "Input Tokens", "Output Tokens",
		"Cache Creation", "Cache Read", "Total Tokens", "Cost USD",
	}
	if options.Aggregate {
		header = append(header, "Count")
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, item := range data {
		record := []string{
			item.Timestamp.Format("2006-01-02 15:04:05"),
			item.Model,
			item.SessionID,
			strconv.Itoa(item.InputTokens),
			strconv.Itoa(item.OutputTokens),
			strconv.Itoa(item.CacheCreationTokens),
			strconv.Itoa(item.CacheReadTokens),
			strconv.Itoa(item.TotalTokens),
			fmt.Sprintf("%.4f", item.CostUSD),
		}

		if options.Aggregate {
			record = append(record, strconv.Itoa(item.Count))
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// exportJSON exports data to JSON format
func (e *Exporter) exportJSON(data []models.AnalysisResult, options ExportOptions) error {
	file, err := e.createOutputFile(options.OutputFile, options.Compress)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// exportXLSX exports data to Excel format (placeholder)
func (e *Exporter) exportXLSX(data []models.AnalysisResult, options ExportOptions) error {
	// This would require a library like excelize
	// For now, fall back to CSV
	e.logger.Warn("XLSX export not implemented, falling back to CSV")
	return e.exportCSV(data, options)
}

// exportParquet exports data to Parquet format (placeholder)
func (e *Exporter) exportParquet(data []models.AnalysisResult, options ExportOptions) error {
	// This would require a parquet library
	// For now, fall back to JSON
	e.logger.Warn("Parquet export not implemented, falling back to JSON")
	return e.exportJSON(data, options)
}

// createOutputFile creates the output file with optional compression
func (e *Exporter) createOutputFile(filename string, compress bool) (*os.File, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// If compression is enabled, wrap with gzip
	if compress && (filepath.Ext(filename) == ".csv" || filepath.Ext(filename) == ".json") {
		// For CSV and JSON files, we'd need to wrap the writer
		// This is a simplified implementation
		return file, nil
	}

	return file, nil
}

// parseTimeString parses various time string formats
func parseTimeString(timeStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}
