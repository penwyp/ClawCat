package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/penwyp/claudecat/cache"
	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/internal"
	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	analyzeOutput              string
	analyzeFrom                string
	analyzeTo                  string
	analyzeFormat              string
	analyzeSortBy              string
	analyzeLimit               int
	analyzeGroupBy             string
	analyzeBreakdown           bool
	analyzeReset               bool
	analyzePricingSource       string
	analyzePricingOffline      bool
	analyzeEnableDeduplication bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [flags] [path...]",
	Short: "Analyze usage data without TUI",
	Long: `Perform analysis on Claude usage data and output results in various formats.

This command provides non-interactive analysis capabilities for batch processing,
reporting, and integration with other tools. It can process multiple data files
and generate detailed reports about usage patterns, costs, and statistics.

Examples:
  claudecat analyze ~/claude-logs                           # Basic analysis
  claudecat analyze --output table --by-model              # Group by model
  claudecat analyze --from 2025-01-01 --to 2025-01-31     # Date range
  claudecat analyze --format json --sort-by cost --limit 10 # Top 10 by cost
  claudecat analyze --group-by hour --output csv > report.csv # Hourly CSV report`,

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
			// Use file-based cache for clearing
			cacheDir := cfg.Cache.Dir
			if cacheDir[:2] == "~/" {
				homeDir, _ := os.UserHomeDir()
				cacheDir = filepath.Join(homeDir, cacheDir[2:])
			}
			fileCache, err := cache.NewFileBasedSummaryCache(cacheDir)
			if err != nil {
				return fmt.Errorf("failed to open cache: %w", err)
			}
			if err := fileCache.Clear(); err != nil {
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
	analyzeCmd.Flags().StringVar(&analyzeGroupBy, "group-by", "", "group by field (model, project, day, week, month)")

	// Sorting and limiting flags
	analyzeCmd.Flags().StringVar(&analyzeSortBy, "sort-by", "timestamp", "sort by field (timestamp, cost, tokens, model)")
	analyzeCmd.Flags().IntVar(&analyzeLimit, "limit", 0, "limit number of results (0 = no limit)")

	// Breakdown flag
	analyzeCmd.Flags().BoolVarP(&analyzeBreakdown, "breakdown", "b", false, "Show per-model cost breakdown")

	// Reset flag
	analyzeCmd.Flags().BoolVarP(&analyzeReset, "reset", "r", false, "Clear cache before analysis")

	// Pricing and deduplication flags
	analyzeCmd.Flags().StringVar(&analyzePricingSource, "pricing-source", "", "pricing source (default, litellm)")
	analyzeCmd.Flags().BoolVar(&analyzePricingOffline, "pricing-offline", false, "use cached pricing data for offline mode")
	analyzeCmd.Flags().BoolVar(&analyzeEnableDeduplication, "deduplication", false, "enable deduplication of entries across all files")

	// Bind to viper
	_ = viper.BindPFlag("analyze.output", analyzeCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("analyze.from", analyzeCmd.Flags().Lookup("from"))
	_ = viper.BindPFlag("analyze.to", analyzeCmd.Flags().Lookup("to"))
	_ = viper.BindPFlag("data.pricing_source", analyzeCmd.Flags().Lookup("pricing-source"))
	_ = viper.BindPFlag("data.pricing_offline_mode", analyzeCmd.Flags().Lookup("pricing-offline"))
	_ = viper.BindPFlag("data.deduplication", analyzeCmd.Flags().Lookup("deduplication"))

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

	homeDir, _ := os.UserHomeDir()
	if len(cfg.Data.Paths) == 0 {
		p := path.Join(homeDir, ".claude", "projects")
		cfg.Data.Paths = []string{p}
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

	// Apply pricing source if provided
	if analyzePricingSource != "" {
		validSources := []string{"default", "litellm"}
		found := false
		for _, source := range validSources {
			if strings.EqualFold(analyzePricingSource, source) {
				cfg.Data.PricingSource = strings.ToLower(analyzePricingSource)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid pricing source: %s (valid options: %s)",
				analyzePricingSource, strings.Join(validSources, ", "))
		}
	}

	// Apply pricing offline mode if set
	if analyzePricingOffline {
		cfg.Data.PricingOfflineMode = true
	}

	// Apply deduplication if set
	if analyzeEnableDeduplication {
		cfg.Data.Deduplication = true
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
		case "project":
			key = result.Project
			if key == "" {
				key = "unknown"
			}
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
			Model:     "", // Clear model since we're aggregating across models
			Timestamp: groupResults[0].Timestamp,
			SessionID: groupResults[0].SessionID,
			Project:   groupResults[0].Project,
		}

		// Aggregate values and collect unique models
		modelSet := make(map[string]bool)
		for _, result := range groupResults {
			agg.InputTokens += result.InputTokens
			agg.OutputTokens += result.OutputTokens
			agg.CacheCreationTokens += result.CacheCreationTokens
			agg.CacheReadTokens += result.CacheReadTokens
			agg.TotalTokens += result.TotalTokens
			agg.CostUSD += result.CostUSD
			if result.Model != "" {
				modelSet[result.Model] = true
			}
		}
		
		// For time-based groupings, set the model to a comma-separated list
		if analyzeGroupBy == "hour" || analyzeGroupBy == "day" || analyzeGroupBy == "week" || analyzeGroupBy == "month" {
			var models []string
			for model := range modelSet {
				models = append(models, model)
			}
			sortModelsByPreference(models)
			agg.Model = strings.Join(models, ", ")
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
		sortModelsByPreference(modelNames)

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
	if len(results) == 0 {
		fmt.Println("No data to display.")
		return nil
	}

	if analyzeBreakdown {
		return outputTableWithBreakdown(results)
	}
	return outputTableWithoutBreakdown(results)
}

func outputTableWithoutBreakdown(results []models.AnalysisResult) error {
	// Determine the primary grouping column header
	var groupColumnHeader string
	switch analyzeGroupBy {
	case "project":
		groupColumnHeader = "Project"
	case "model":
		groupColumnHeader = "Model"
	case "session":
		groupColumnHeader = "Session"
	case "hour", "day", "week", "month":
		groupColumnHeader = "Date"
	default:
		groupColumnHeader = "Group"
	}

	// Create table headers
	headers := []string{groupColumnHeader, "Input", "Output", "Cache Create", "Cache Read", "Total Tokens", "Cost (USD)"}
	if analyzeGroupBy != "model" && analyzeGroupBy != "project" {
		// Add Models column for time-based groupings
		headers = []string{groupColumnHeader, "Models", "Input", "Output", "Cache Create", "Cache Read", "Total Tokens", "Cost (USD)"}
	}
	table := newTableFormatter(headers)

	// For all groupings, we can use the aggregated results directly
	if analyzeGroupBy != "model" && analyzeGroupBy != "project" && analyzeGroupBy != "session" {
		// Time-based groupings - add Models column
		// Sort results by group key
		sort.Slice(results, func(i, j int) bool {
			return results[i].GroupKey < results[j].GroupKey
		})

		// Add rows directly from results
		for _, result := range results {
			row := []string{
				result.GroupKey,
				result.Model, // This contains the comma-separated list of models
				formatWithCommas(result.InputTokens),
				formatWithCommas(result.OutputTokens),
				formatWithCommas(result.CacheCreationTokens),
				formatWithCommas(result.CacheReadTokens),
				formatWithCommas(result.TotalTokens),
				formatCost(result.CostUSD),
			}
			table.addRow(row)
		}

		// Add summary row
		addSummaryRowWithModels(table, results)
	} else {
		// For non-time-based groupings (model, project, session)
		// Sort results by group key
		sort.Slice(results, func(i, j int) bool {
			return results[i].GroupKey < results[j].GroupKey
		})

		// Add rows directly from results
		for _, result := range results {
			row := []string{
				result.GroupKey,
				formatWithCommas(result.InputTokens),
				formatWithCommas(result.OutputTokens),
				formatWithCommas(result.CacheCreationTokens),
				formatWithCommas(result.CacheReadTokens),
				formatWithCommas(result.TotalTokens),
				formatCost(result.CostUSD),
			}
			table.addRow(row)
		}

		// Add summary row for non-time-based groupings
		addSummaryRowSimple(table, results)
	}

	fmt.Print(table.render())
	return nil
}

func outputTableWithBreakdown(results []models.AnalysisResult) error {
	// Group results by date, then by model
	dateGroups := make(map[string]*dateGroupWithModels)

	for _, result := range results {
		var dateKey string
		if analyzeGroupBy == "day" {
			dateKey = result.GroupKey
		} else {
			dateKey = result.Timestamp.Format("2006-01-02")
		}

		if dateGroups[dateKey] == nil {
			dateGroups[dateKey] = &dateGroupWithModels{
				date:       dateKey,
				modelStats: make(map[string]*modelStat),
			}
		}

		group := dateGroups[dateKey]

		// Skip TOTAL rows from breakdown grouping as we'll calculate our own totals
		if result.Model == "TOTAL" {
			continue
		}

		if group.modelStats[result.Model] == nil {
			group.modelStats[result.Model] = &modelStat{}
		}

		stat := group.modelStats[result.Model]
		stat.inputTokens += result.InputTokens
		stat.outputTokens += result.OutputTokens
		stat.cacheCreationTokens += result.CacheCreationTokens
		stat.cacheReadTokens += result.CacheReadTokens
		stat.totalTokens += result.TotalTokens
		stat.costUSD += result.CostUSD

		// Update group totals
		group.totalInputTokens += result.InputTokens
		group.totalOutputTokens += result.OutputTokens
		group.totalCacheCreationTokens += result.CacheCreationTokens
		group.totalCacheReadTokens += result.CacheReadTokens
		group.totalTotalTokens += result.TotalTokens
		group.totalCostUSD += result.CostUSD
	}

	// Create table
	headers := []string{"Date", "Models", "Input", "Output", "Cache Create", "Cache Read", "Total Tokens", "Cost (USD)"}
	table := newTableFormatter(headers)

	// Sort dates
	var dates []string
	for date := range dateGroups {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	// Add rows with breakdown
	for i, date := range dates {
		group := dateGroups[date]

		// Sort models
		var modelNames []string
		for model := range group.modelStats {
			modelNames = append(modelNames, model)
		}
		sortModelsByPreference(modelNames)

		// Convert models to string list
		var modelList []string
		for _, model := range modelNames {
			modelList = append(modelList, model)
		}

		// Add main date row with aggregated data (leave models column empty in breakdown mode)
		row := []string{
			date,
			"", // Empty models column in breakdown mode
			formatWithCommas(group.totalInputTokens),
			formatWithCommas(group.totalOutputTokens),
			formatWithCommas(group.totalCacheCreationTokens),
			formatWithCommas(group.totalCacheReadTokens),
			formatWithCommas(group.totalTotalTokens),
			formatCost(group.totalCostUSD),
		}
		table.addRow(row)

		// Add model breakdown rows
		for _, model := range modelNames {
			stat := group.modelStats[model]
			breakdownRow := []string{
				"",
				"└─ " + model,
				formatWithCommas(stat.inputTokens),
				formatWithCommas(stat.outputTokens),
				formatWithCommas(stat.cacheCreationTokens),
				formatWithCommas(stat.cacheReadTokens),
				formatWithCommas(stat.totalTokens),
				formatCost(stat.costUSD),
			}
			table.addRow(breakdownRow)
		}

		// Add separator row between dates (except for the last one)
		if i < len(dates)-1 {
			separatorRow := make([]string, len(headers))
			table.addRow(separatorRow)
		}
	}

	// Add summary row for breakdown mode
	addSummaryRowBreakdown(table, dateGroups)

	fmt.Print(table.render())
	return nil
}

// Helper types for grouping data
type dateGroup struct {
	date                string
	models              map[string]bool
	inputTokens         int
	outputTokens        int
	cacheCreationTokens int
	cacheReadTokens     int
	totalTokens         int
	costUSD             float64
}

type modelStat struct {
	inputTokens         int
	outputTokens        int
	cacheCreationTokens int
	cacheReadTokens     int
	totalTokens         int
	costUSD             float64
}

type dateGroupWithModels struct {
	date                     string
	modelStats               map[string]*modelStat
	totalInputTokens         int
	totalOutputTokens        int
	totalCacheCreationTokens int
	totalCacheReadTokens     int
	totalTotalTokens         int
	totalCostUSD             float64
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

// Table formatting utilities for bordered tables

type tableFormatter struct {
	headers []string
	rows    [][]string
	widths  []int
}

func newTableFormatter(headers []string) *tableFormatter {
	return &tableFormatter{
		headers: headers,
		rows:    make([][]string, 0),
		widths:  make([]int, len(headers)),
	}
}

func (tf *tableFormatter) addRow(row []string) {
	// Ensure row has same length as headers
	if len(row) < len(tf.headers) {
		extendedRow := make([]string, len(tf.headers))
		copy(extendedRow, row)
		row = extendedRow
	} else if len(row) > len(tf.headers) {
		row = row[:len(tf.headers)]
	}

	tf.rows = append(tf.rows, row)
}

func (tf *tableFormatter) addSeparatorLine() {
	separatorRow := make([]string, len(tf.headers))
	for i := range separatorRow {
		separatorRow[i] = "SEPARATOR"
	}
	tf.rows = append(tf.rows, separatorRow)
}

func (tf *tableFormatter) calculateWidths() {
	// Initialize with header widths
	for i, header := range tf.headers {
		tf.widths[i] = runeWidth(header)
	}

	// Check row data widths
	for _, row := range tf.rows {
		for i, cell := range row {
			if i < len(tf.widths) {
				cellWidth := runeWidth(cell)
				if cellWidth > tf.widths[i] {
					tf.widths[i] = cellWidth
				}
			}
		}
	}
}

func (tf *tableFormatter) render() string {
	if len(tf.headers) == 0 {
		return ""
	}

	tf.calculateWidths()
	var lines []string

	// Top border
	lines = append(lines, tf.renderTopBorder())

	// Headers
	lines = append(lines, tf.renderRow(tf.headers))

	// Header separator
	lines = append(lines, tf.renderSeparator())

	// Data rows
	for _, row := range tf.rows {
		if len(row) > 0 && row[0] == "SEPARATOR" {
			lines = append(lines, tf.renderSeparator())
		} else {
			lines = append(lines, tf.renderRow(row))
		}
	}

	// Bottom border
	lines = append(lines, tf.renderBottomBorder())

	return strings.Join(lines, "\n")
}

func (tf *tableFormatter) renderTopBorder() string {
	var parts []string
	parts = append(parts, "┌")

	for i, width := range tf.widths {
		parts = append(parts, strings.Repeat("─", width+2)) // +2 for padding
		if i < len(tf.widths)-1 {
			parts = append(parts, "┬")
		}
	}

	parts = append(parts, "┐")
	return strings.Join(parts, "")
}

func (tf *tableFormatter) renderBottomBorder() string {
	var parts []string
	parts = append(parts, "└")

	for i, width := range tf.widths {
		parts = append(parts, strings.Repeat("─", width+2)) // +2 for padding
		if i < len(tf.widths)-1 {
			parts = append(parts, "┴")
		}
	}

	parts = append(parts, "┘")
	return strings.Join(parts, "")
}

func (tf *tableFormatter) renderSeparator() string {
	var parts []string
	parts = append(parts, "├")

	for i, width := range tf.widths {
		parts = append(parts, strings.Repeat("─", width+2)) // +2 for padding
		if i < len(tf.widths)-1 {
			parts = append(parts, "┼")
		}
	}

	parts = append(parts, "┤")
	return strings.Join(parts, "")
}

func (tf *tableFormatter) renderRow(row []string) string {
	var parts []string
	parts = append(parts, "│")

	for i, cell := range row {
		if i < len(tf.widths) {
			// Right-align numeric columns (tokens and cost), left-align others
			padded := tf.padCell(cell, tf.widths[i], tf.isNumericColumn(i))
			parts = append(parts, " "+padded+" ")
			parts = append(parts, "│")
		}
	}

	return strings.Join(parts, "")
}

func (tf *tableFormatter) isNumericColumn(colIndex int) bool {
	if colIndex >= len(tf.headers) {
		return false
	}

	header := strings.ToLower(tf.headers[colIndex])
	return strings.Contains(header, "input") ||
		strings.Contains(header, "output") ||
		strings.Contains(header, "cache") ||
		strings.Contains(header, "tokens") ||
		strings.Contains(header, "cost")
}

// runeWidth calculates the display width of a string, accounting for Unicode characters
func runeWidth(s string) int {
	width := 0
	for _, r := range s {
		// Most printable ASCII characters have width 1
		if r >= 32 && r <= 126 {
			width++
		} else if r == '\t' {
			width += 8 // Assume tab width of 8
		} else {
			// For Unicode characters, assume width 1 for now
			// This is a simplification - some characters may be wider
			width++
		}
	}
	return width
}

func (tf *tableFormatter) padCell(text string, width int, rightAlign bool) string {
	textWidth := runeWidth(text)
	if textWidth >= width {
		return text
	}

	padding := width - textWidth
	if rightAlign {
		return strings.Repeat(" ", padding) + text
	}
	return text + strings.Repeat(" ", padding)
}

// Number formatting functions

func formatWithCommas(n int) string {
	str := strconv.Itoa(n)
	if len(str) <= 3 {
		return str
	}

	var result []byte
	for i, digit := range []byte(str) {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, digit)
	}

	return string(result)
}

func formatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

func formatModels(models []string) string {
	if len(models) == 0 {
		return ""
	}
	if len(models) == 1 {
		return models[0]
	}

	return strings.Join(models, ", ")
}

// sortModelsByPreference sorts models by preference: ops first, then sonnet, then others
func sortModelsByPreference(models []string) {
	sort.Slice(models, func(i, j int) bool {
		return getModelPriority(models[i]) < getModelPriority(models[j])
	})
}

// getModelPriority returns priority order for model sorting
// Lower numbers have higher priority
func getModelPriority(model string) int {
	modelLower := strings.ToLower(model)

	// Opus models (highest priority)
	if strings.Contains(modelLower, "opus") {
		return 1
	}

	// Sonnet models (medium priority)
	if strings.Contains(modelLower, "sonnet") {
		return 2
	}

	// Haiku models
	if strings.Contains(modelLower, "haiku") {
		return 3
	}

	// Other models (lowest priority)
	return 4
}

// addSummaryRow adds a summary row to the table for non-breakdown mode
func addSummaryRow(table *tableFormatter, dateGroups map[string]*dateGroup) {
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead, totalTokens int
	var totalCost float64
	var allModels = make(map[string]bool)

	for _, group := range dateGroups {
		totalInput += group.inputTokens
		totalOutput += group.outputTokens
		totalCacheCreation += group.cacheCreationTokens
		totalCacheRead += group.cacheReadTokens
		totalTokens += group.totalTokens
		totalCost += group.costUSD

		for model := range group.models {
			allModels[model] = true
		}
	}

	// Convert models map to sorted slice
	var modelList []string
	for model := range allModels {
		modelList = append(modelList, model)
	}
	sortModelsByPreference(modelList)

	// Add separator line before TOTAL
	table.addSeparatorLine()

	// Add summary row
	summaryRow := []string{
		"TOTAL",
		formatModels(modelList),
		formatWithCommas(totalInput),
		formatWithCommas(totalOutput),
		formatWithCommas(totalCacheCreation),
		formatWithCommas(totalCacheRead),
		formatWithCommas(totalTokens),
		formatCost(totalCost),
	}
	table.addRow(summaryRow)
}

// addSummaryRowSimple adds a summary row for non-time-based groupings
func addSummaryRowSimple(table *tableFormatter, results []models.AnalysisResult) {
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead, totalTokens int
	var totalCost float64

	for _, result := range results {
		totalInput += result.InputTokens
		totalOutput += result.OutputTokens
		totalCacheCreation += result.CacheCreationTokens
		totalCacheRead += result.CacheReadTokens
		totalTokens += result.TotalTokens
		totalCost += result.CostUSD
	}

	// Add separator line before TOTAL
	table.addSeparatorLine()

	// Add summary row
	summaryRow := []string{
		"TOTAL",
		formatWithCommas(totalInput),
		formatWithCommas(totalOutput),
		formatWithCommas(totalCacheCreation),
		formatWithCommas(totalCacheRead),
		formatWithCommas(totalTokens),
		formatCost(totalCost),
	}
	table.addRow(summaryRow)
}

// addSummaryRowWithModels adds a summary row for time-based groupings with models column
func addSummaryRowWithModels(table *tableFormatter, results []models.AnalysisResult) {
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead, totalTokens int
	var totalCost float64
	allModels := make(map[string]bool)

	for _, result := range results {
		totalInput += result.InputTokens
		totalOutput += result.OutputTokens
		totalCacheCreation += result.CacheCreationTokens
		totalCacheRead += result.CacheReadTokens
		totalTokens += result.TotalTokens
		totalCost += result.CostUSD
		
		// Extract models from the comma-separated list
		if result.Model != "" {
			models := strings.Split(result.Model, ", ")
			for _, model := range models {
				allModels[strings.TrimSpace(model)] = true
			}
		}
	}

	// Convert models map to sorted slice
	var modelList []string
	for model := range allModels {
		modelList = append(modelList, model)
	}
	sortModelsByPreference(modelList)

	// Add separator line before TOTAL
	table.addSeparatorLine()

	// Add summary row
	summaryRow := []string{
		"TOTAL",
		formatModels(modelList),
		formatWithCommas(totalInput),
		formatWithCommas(totalOutput),
		formatWithCommas(totalCacheCreation),
		formatWithCommas(totalCacheRead),
		formatWithCommas(totalTokens),
		formatCost(totalCost),
	}
	table.addRow(summaryRow)
}

// addSummaryRowBreakdown adds a summary row to the table for breakdown mode
func addSummaryRowBreakdown(table *tableFormatter, dateGroups map[string]*dateGroupWithModels) {
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead, totalTokens int
	var totalCost float64

	for _, group := range dateGroups {
		totalInput += group.totalInputTokens
		totalOutput += group.totalOutputTokens
		totalCacheCreation += group.totalCacheCreationTokens
		totalCacheRead += group.totalCacheReadTokens
		totalTokens += group.totalTotalTokens
		totalCost += group.totalCostUSD
	}

	// Add separator line before TOTAL
	table.addSeparatorLine()

	// Add summary row (empty models column in breakdown mode)
	summaryRow := []string{
		"TOTAL",
		"", // Empty models column in breakdown mode
		formatWithCommas(totalInput),
		formatWithCommas(totalOutput),
		formatWithCommas(totalCacheCreation),
		formatWithCommas(totalCacheRead),
		formatWithCommas(totalTokens),
		formatCost(totalCost),
	}
	table.addRow(summaryRow)
}
