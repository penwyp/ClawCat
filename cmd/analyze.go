package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/ClawCat/cache"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/internal"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	analyzeOutput    string
	analyzeFrom      string
	analyzeTo        string
	analyzeByModel   bool
	analyzeByDay     bool
	analyzeByHour    bool
	analyzeFormat    string
	analyzeSortBy    string
	analyzeLimit     int
	analyzeGroupBy   string
	analyzeBreakdown bool
	analyzeReset     bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [flags] [path...]",
	Short: "Analyze usage data without TUI",
	Long: `Perform analysis on Claude usage data and output results in various formats.

This command provides non-interactive analysis capabilities for batch processing,
reporting, and integration with other tools. It can process multiple data files
and generate detailed reports about usage patterns, costs, and statistics.

Examples:
  clawcat analyze ~/claude-logs                           # Basic analysis
  clawcat analyze --output table --by-model              # Group by model
  clawcat analyze --from 2025-01-01 --to 2025-01-31     # Date range
  clawcat analyze --format json --sort-by cost --limit 10 # Top 10 by cost
  clawcat analyze --group-by hour --output csv > report.csv # Hourly CSV report`,

	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := loadConfiguration(cmd)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Apply analyze-specific overrides
		if err := applyAnalyzeFlags(cfg, args); err != nil {
			return fmt.Errorf("failed to apply command flags: %w", err)
		}

		// Apply debug flag if set from command line
		if debug {
			cfg.Debug.Enabled = true
			// Set log level to debug when debug flag is enabled
			cfg.App.LogLevel = "debug"
		}

		// Initialize global logger for usage_loader cache logging
		logging.InitLogger(cfg.App.LogLevel, cfg.App.LogFile, cfg.Debug.Enabled)

		// Reset cache if requested
		if analyzeReset {
			storeConfig := cache.StoreConfig{
				MaxFileSize: 50 * 1024 * 1024,  // 50MB
				MaxMemory:   100 * 1024 * 1024, // 100MB
			}
			cacheStore := cache.NewStore(storeConfig)
			if err := cacheStore.Clear(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}
			logging.GetLogger().Info("Cache cleared successfully")
		}

		// Create analyzer
		analyzer, err := internal.NewAnalyzer(cfg)
		if err != nil {
			return fmt.Errorf("failed to create analyzer: %w", err)
		}

		// Perform analysis
		results, err := analyzer.Analyze(cfg.Data.Paths)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		// Apply filtering and grouping
		results = applyFilters(results)
		results = applyGrouping(results)
		results = applySorting(results)
		results = applyLimit(results)

		// Output results
		return outputAnalysisResults(results)
	},
}

func init() {
	// Output format flags
	analyzeCmd.Flags().StringVarP(&analyzeOutput, "output", "o", "table", "output format (table, json, csv, summary)")
	analyzeCmd.Flags().StringVar(&analyzeFormat, "format", "", "alias for --output")

	// Date range flags
	analyzeCmd.Flags().StringVar(&analyzeFrom, "from", "", "start date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	analyzeCmd.Flags().StringVar(&analyzeTo, "to", "", "end date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")

	// Grouping flags
	analyzeCmd.Flags().BoolVar(&analyzeByModel, "by-model", false, "group results by model")
	analyzeCmd.Flags().BoolVar(&analyzeByDay, "by-day", false, "group results by day")
	analyzeCmd.Flags().BoolVar(&analyzeByHour, "by-hour", false, "group results by hour")
	analyzeCmd.Flags().StringVar(&analyzeGroupBy, "group-by", "", "group by field (model, day, hour, week, month, session)")

	// Sorting and limiting flags
	analyzeCmd.Flags().StringVar(&analyzeSortBy, "sort-by", "timestamp", "sort by field (timestamp, cost, tokens, model)")
	analyzeCmd.Flags().IntVar(&analyzeLimit, "limit", 0, "limit number of results (0 = no limit)")

	// Breakdown flag
	analyzeCmd.Flags().BoolVarP(&analyzeBreakdown, "breakdown", "b", false, "Show per-model cost breakdown")

	// Reset flag
	analyzeCmd.Flags().BoolVarP(&analyzeReset, "reset", "r", false, "Clear cache before analysis")

	// Bind to viper
	_ = viper.BindPFlag("analyze.output", analyzeCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("analyze.from", analyzeCmd.Flags().Lookup("from"))
	_ = viper.BindPFlag("analyze.to", analyzeCmd.Flags().Lookup("to"))

	rootCmd.AddCommand(analyzeCmd)
}

func applyAnalyzeFlags(cfg *config.Config, args []string) error {
	// Set data paths from arguments
	if len(args) > 0 {
		// Validate paths exist
		for _, path := range args {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", path)
			}
		}
		cfg.Data.Paths = args
	}

	// Use format as alias for output if provided
	if analyzeFormat != "" {
		analyzeOutput = analyzeFormat
	}

	// Validate output format
	validOutputs := []string{"table", "json", "csv", "summary"}
	found := false
	for _, output := range validOutputs {
		if strings.EqualFold(analyzeOutput, output) {
			analyzeOutput = strings.ToLower(analyzeOutput)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid output format: %s (valid options: %s)",
			analyzeOutput, strings.Join(validOutputs, ", "))
	}

	// Set grouping based on boolean flags
	if analyzeByModel {
		analyzeGroupBy = "model"
	} else if analyzeByDay {
		analyzeGroupBy = "day"
	} else if analyzeByHour {
		analyzeGroupBy = "hour"
	}

	// Validate sort field
	if analyzeSortBy != "" {
		validSorts := []string{"timestamp", "cost", "tokens", "model", "input_tokens", "output_tokens"}
		found := false
		for _, sort := range validSorts {
			if strings.EqualFold(analyzeSortBy, sort) {
				analyzeSortBy = strings.ToLower(analyzeSortBy)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid sort field: %s (valid options: %s)",
				analyzeSortBy, strings.Join(validSorts, ", "))
		}
	}

	return nil
}

func applyFilters(results []models.AnalysisResult) []models.AnalysisResult {
	if analyzeFrom == "" && analyzeTo == "" {
		return results
	}

	var filtered []models.AnalysisResult

	var fromTime, toTime time.Time
	var err error

	// Parse from time
	if analyzeFrom != "" {
		fromTime, err = parseTimeString(analyzeFrom)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid from date %s: %v\n", analyzeFrom, err)
			return results
		}
	}

	// Parse to time
	if analyzeTo != "" {
		toTime, err = parseTimeString(analyzeTo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: invalid to date %s: %v\n", analyzeTo, err)
			return results
		}
	}

	// Filter results
	for _, result := range results {
		if analyzeFrom != "" && result.Timestamp.Before(fromTime) {
			continue
		}
		if analyzeTo != "" && result.Timestamp.After(toTime) {
			continue
		}
		filtered = append(filtered, result)
	}

	return filtered
}

func applyGrouping(results []models.AnalysisResult) []models.AnalysisResult {
	// Default to group by day if no grouping specified
	if analyzeGroupBy == "" {
		analyzeGroupBy = "day"
	}

	// If breakdown is enabled and we're grouping by time, use special breakdown grouping
	if analyzeBreakdown && (analyzeGroupBy == "hour" || analyzeGroupBy == "day" || analyzeGroupBy == "week" || analyzeGroupBy == "month") {
		return applyBreakdownGrouping(results)
	}

	// Regular grouping logic
	groups := make(map[string][]models.AnalysisResult)

	for _, result := range results {
		var key string
		switch analyzeGroupBy {
		case "model":
			key = result.Model
		case "day":
			key = result.Timestamp.Format("2006-01-02")
		case "hour":
			key = result.Timestamp.Format("2006-01-02 15:00")
		case "week":
			year, week := result.Timestamp.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", year, week)
		case "month":
			key = result.Timestamp.Format("2006-01")
		case "session":
			key = result.SessionID
		default:
			key = "all"
		}

		groups[key] = append(groups[key], result)
	}

	// Aggregate grouped results
	var aggregated []models.AnalysisResult
	for groupKey, groupResults := range groups {
		if len(groupResults) == 0 {
			continue
		}

		// Create aggregated result
		agg := models.AnalysisResult{
			GroupKey:  groupKey,
			Model:     groupResults[0].Model,
			Timestamp: groupResults[0].Timestamp,
			SessionID: groupResults[0].SessionID,
		}

		// Aggregate values
		for _, result := range groupResults {
			agg.InputTokens += result.InputTokens
			agg.OutputTokens += result.OutputTokens
			agg.CacheCreationTokens += result.CacheCreationTokens
			agg.CacheReadTokens += result.CacheReadTokens
			agg.TotalTokens += result.TotalTokens
			agg.CostUSD += result.CostUSD
		}

		agg.Count = len(groupResults)
		aggregated = append(aggregated, agg)
	}

	return aggregated
}

func applyBreakdownGrouping(results []models.AnalysisResult) []models.AnalysisResult {
	// Group by time period, then by model
	type modelData struct {
		models map[string]*models.AnalysisResult
		total  *models.AnalysisResult
	}

	groups := make(map[string]*modelData)

	// First pass: group by time period and model
	for _, result := range results {
		var timeKey string
		switch analyzeGroupBy {
		case "hour":
			timeKey = result.Timestamp.Format("2006-01-02 15:00")
		case "day":
			timeKey = result.Timestamp.Format("2006-01-02")
		case "week":
			year, week := result.Timestamp.ISOWeek()
			timeKey = fmt.Sprintf("%d-W%02d", year, week)
		case "month":
			timeKey = result.Timestamp.Format("2006-01")
		}

		if groups[timeKey] == nil {
			groups[timeKey] = &modelData{
				models: make(map[string]*models.AnalysisResult),
				total: &models.AnalysisResult{
					GroupKey:  timeKey,
					Model:     "TOTAL",
					Timestamp: result.Timestamp,
				},
			}
		}

		// Add to model-specific data
		if groups[timeKey].models[result.Model] == nil {
			groups[timeKey].models[result.Model] = &models.AnalysisResult{
				GroupKey:  timeKey,
				Model:     result.Model,
				Timestamp: result.Timestamp,
			}
		}

		modelResult := groups[timeKey].models[result.Model]
		modelResult.InputTokens += result.InputTokens
		modelResult.OutputTokens += result.OutputTokens
		modelResult.CacheCreationTokens += result.CacheCreationTokens
		modelResult.CacheReadTokens += result.CacheReadTokens
		modelResult.TotalTokens += result.TotalTokens
		modelResult.CostUSD += result.CostUSD
		modelResult.Count++

		// Add to total
		totalResult := groups[timeKey].total
		totalResult.InputTokens += result.InputTokens
		totalResult.OutputTokens += result.OutputTokens
		totalResult.CacheCreationTokens += result.CacheCreationTokens
		totalResult.CacheReadTokens += result.CacheReadTokens
		totalResult.TotalTokens += result.TotalTokens
		totalResult.CostUSD += result.CostUSD
		totalResult.Count++
	}

	// Second pass: flatten into results array
	var aggregated []models.AnalysisResult

	// Sort time keys
	var timeKeys []string
	for key := range groups {
		timeKeys = append(timeKeys, key)
	}
	sort.Strings(timeKeys)

	for _, timeKey := range timeKeys {
		groupData := groups[timeKey]

		// Sort model names
		var modelNames []string
		for modelName := range groupData.models {
			modelNames = append(modelNames, modelName)
		}
		sort.Strings(modelNames)

		// Add model-specific results
		for _, modelName := range modelNames {
			aggregated = append(aggregated, *groupData.models[modelName])
		}

		// Add total row
		aggregated = append(aggregated, *groupData.total)
	}

	return aggregated
}

func applySorting(results []models.AnalysisResult) []models.AnalysisResult {
	if analyzeSortBy == "" {
		return results
	}

	sort.Slice(results, func(i, j int) bool {
		switch analyzeSortBy {
		case "timestamp":
			return results[i].Timestamp.Before(results[j].Timestamp)
		case "cost":
			return results[i].CostUSD > results[j].CostUSD // Descending
		case "tokens":
			return results[i].TotalTokens > results[j].TotalTokens // Descending
		case "input_tokens":
			return results[i].InputTokens > results[j].InputTokens // Descending
		case "output_tokens":
			return results[i].OutputTokens > results[j].OutputTokens // Descending
		case "model":
			return results[i].Model < results[j].Model
		default:
			return false
		}
	})

	return results
}

func applyLimit(results []models.AnalysisResult) []models.AnalysisResult {
	if analyzeLimit <= 0 || analyzeLimit >= len(results) {
		return results
	}
	return results[:analyzeLimit]
}

func outputAnalysisResults(results []models.AnalysisResult) error {
	switch analyzeOutput {
	case "table":
		return outputTable(results)
	case "json":
		return outputJSON(results)
	case "csv":
		return outputCSV(results)
	case "summary":
		return outputSummary(results)
	default:
		return fmt.Errorf("unsupported output format: %s", analyzeOutput)
	}
}

func outputTable(results []models.AnalysisResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if analyzeGroupBy != "" {
		fmt.Fprintln(w, "Group\tModel\tEntries\tInput Tokens\tOutput Tokens\tCache Creation\tCache Read\tTotal Tokens\tCost (USD)")
	} else {
		fmt.Fprintln(w, "Timestamp\tModel\tSession\tInput Tokens\tOutput Tokens\tCache Creation\tCache Read\tTotal Tokens\tCost (USD)")
	}

	// Data rows
	for i, result := range results {
		if analyzeGroupBy != "" {
			// Add blank line after TOTAL row when using breakdown (except for the last row)
			if analyzeBreakdown && i > 0 && results[i-1].Model == "TOTAL" && result.Model != "TOTAL" && i < len(results) {
				fmt.Fprintln(w, "")
			}

			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t$%.4f\n",
				result.GroupKey,
				result.Model,
				result.Count,
				result.InputTokens,
				result.OutputTokens,
				result.CacheCreationTokens,
				result.CacheReadTokens,
				result.TotalTokens,
				result.CostUSD)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t$%.4f\n",
				result.Timestamp.Format("2006-01-02 15:04:05"),
				result.Model,
				result.SessionID,
				result.InputTokens,
				result.OutputTokens,
				result.CacheCreationTokens,
				result.CacheReadTokens,
				result.TotalTokens,
				result.CostUSD)
		}
	}

	return w.Flush()
}

func outputJSON(results []models.AnalysisResult) error {
	data, err := sonic.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte("\n"))
	return err
}

func outputCSV(results []models.AnalysisResult) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Header
	if analyzeGroupBy != "" {
		_ = writer.Write([]string{"Group", "Model", "Entries", "Input Tokens", "Output Tokens",
			"Cache Creation", "Cache Read", "Total Tokens", "Cost USD"})
	} else {
		_ = writer.Write([]string{"Timestamp", "Model", "Session", "Input Tokens", "Output Tokens",
			"Cache Creation", "Cache Read", "Total Tokens", "Cost USD"})
	}

	// Data rows
	for _, result := range results {
		if analyzeGroupBy != "" {
			_ = writer.Write([]string{
				result.GroupKey,
				result.Model,
				strconv.Itoa(result.Count),
				strconv.Itoa(result.InputTokens),
				strconv.Itoa(result.OutputTokens),
				strconv.Itoa(result.CacheCreationTokens),
				strconv.Itoa(result.CacheReadTokens),
				strconv.Itoa(result.TotalTokens),
				fmt.Sprintf("%.4f", result.CostUSD),
			})
		} else {
			_ = writer.Write([]string{
				result.Timestamp.Format("2006-01-02 15:04:05"),
				result.Model,
				result.SessionID,
				strconv.Itoa(result.InputTokens),
				strconv.Itoa(result.OutputTokens),
				strconv.Itoa(result.CacheCreationTokens),
				strconv.Itoa(result.CacheReadTokens),
				strconv.Itoa(result.TotalTokens),
				fmt.Sprintf("%.4f", result.CostUSD),
			})
		}
	}

	return nil
}

func outputSummary(results []models.AnalysisResult) error {
	if len(results) == 0 {
		fmt.Println("No data found.")
		return nil
	}

	// Calculate totals
	var totalEntries int
	var totalInputTokens, totalOutputTokens, totalCacheCreation, totalCacheRead, totalTokens int
	var totalCost float64
	modelCounts := make(map[string]int)
	modelStats := make(map[string]struct {
		InputTokens         int
		OutputTokens        int
		CacheCreationTokens int
		CacheReadTokens     int
		TotalTokens         int
		Cost                float64
	})

	for _, result := range results {
		if analyzeGroupBy != "" {
			totalEntries += result.Count
		} else {
			totalEntries++
		}
		totalInputTokens += result.InputTokens
		totalOutputTokens += result.OutputTokens
		totalCacheCreation += result.CacheCreationTokens
		totalCacheRead += result.CacheReadTokens
		totalTokens += result.TotalTokens
		totalCost += result.CostUSD
		modelCounts[result.Model]++

		// Aggregate model stats for breakdown
		stat := modelStats[result.Model]
		stat.InputTokens += result.InputTokens
		stat.OutputTokens += result.OutputTokens
		stat.CacheCreationTokens += result.CacheCreationTokens
		stat.CacheReadTokens += result.CacheReadTokens
		stat.TotalTokens += result.TotalTokens
		stat.Cost += result.CostUSD
		modelStats[result.Model] = stat
	}

	// Output summary
	fmt.Printf("Analysis Summary\n")
	fmt.Printf("================\n\n")
	fmt.Printf("Total Entries: %d\n", totalEntries)
	fmt.Printf("Date Range: %s to %s\n",
		results[0].Timestamp.Format("2006-01-02 15:04:05"),
		results[len(results)-1].Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("\nToken Usage:\n")
	fmt.Printf("  Input Tokens: %d\n", totalInputTokens)
	fmt.Printf("  Output Tokens: %d\n", totalOutputTokens)
	fmt.Printf("  Cache Creation: %d\n", totalCacheCreation)
	fmt.Printf("  Cache Read: %d\n", totalCacheRead)
	fmt.Printf("  Total Tokens: %d\n", totalTokens)
	fmt.Printf("\nCost: $%.4f\n\n", totalCost)

	fmt.Printf("Models Used:\n")
	for model, count := range modelCounts {
		fmt.Printf("  %s: %d entries\n", model, count)
	}

	// Show per-model breakdown if requested
	if analyzeBreakdown {
		fmt.Printf("\nPer-Model Cost Breakdown:\n")
		fmt.Printf("========================\n")

		// Sort models by cost (descending)
		type modelBreakdown struct {
			name  string
			stats struct {
				InputTokens         int
				OutputTokens        int
				CacheCreationTokens int
				CacheReadTokens     int
				TotalTokens         int
				Cost                float64
			}
		}

		var breakdowns []modelBreakdown
		for model, stats := range modelStats {
			breakdowns = append(breakdowns, modelBreakdown{name: model, stats: stats})
		}

		sort.Slice(breakdowns, func(i, j int) bool {
			return breakdowns[i].stats.Cost > breakdowns[j].stats.Cost
		})

		for _, b := range breakdowns {
			fmt.Printf("\n%s:\n", b.name)
			fmt.Printf("  Input Tokens: %d\n", b.stats.InputTokens)
			fmt.Printf("  Output Tokens: %d\n", b.stats.OutputTokens)
			fmt.Printf("  Cache Creation: %d\n", b.stats.CacheCreationTokens)
			fmt.Printf("  Cache Read: %d\n", b.stats.CacheReadTokens)
			fmt.Printf("  Total Tokens: %d\n", b.stats.TotalTokens)
			fmt.Printf("  Cost: $%.4f (%.1f%%)\n", b.stats.Cost, (b.stats.Cost/totalCost)*100)
		}
	}

	return nil
}

func parseTimeString(timeStr string) (time.Time, error) {
	// Try different time formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}
