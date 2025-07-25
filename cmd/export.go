package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/penwyp/ClawCat/internal"
)

var (
	exportFormat    string
	exportRange     string
	exportFrom      string
	exportTo        string
	exportAggregate bool
	exportCompress  bool
	exportOverwrite bool
	exportTemplate  string
)


var exportCmd = &cobra.Command{
	Use:   "export [flags] <output-file>",
	Short: "Export usage data to various formats",
	Long: `Export Claude usage data to various formats for analysis, reporting, or backup.

Supported export formats:
  csv     - Comma-separated values (default)
  json    - JavaScript Object Notation
  xlsx    - Microsoft Excel spreadsheet
  parquet - Apache Parquet columnar format

Time ranges:
  today   - Today's data only
  week    - Last 7 days
  month   - Last 30 days  
  year    - Last 365 days
  all     - All available data (default)

Examples:
  clawcat export data.csv                              # Export all data to CSV
  clawcat export --format json data.json              # Export to JSON
  clawcat export --range week --compress weekly.csv   # Last week, compressed
  clawcat export --from 2025-01-01 --to 2025-01-31 jan.xlsx  # Date range to Excel
  clawcat export --aggregate --format json summary.json      # Aggregated data`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile := args[0]

		// Validate output file
		if err := validateOutputFile(outputFile); err != nil {
			return err
		}

		// Load configuration
		cfg, err := loadConfiguration()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Create export options
		options := internal.ExportOptions{
			Format:     exportFormat,
			TimeRange:  exportRange,
			FromTime:   exportFrom,
			ToTime:     exportTo,
			Aggregate:  exportAggregate,
			Compress:   exportCompress,
			Overwrite:  exportOverwrite,
			Template:   exportTemplate,
			OutputFile: outputFile,
		}

		// Validate and normalize options
		if err := validateExportOptions(&options); err != nil {
			return fmt.Errorf("invalid export options: %w", err)
		}

		// Create exporter
		exporter, err := internal.NewExporter(cfg)
		if err != nil {
			return fmt.Errorf("failed to create exporter: %w", err)
		}

		// Perform export
		if verbose {
			fmt.Fprintf(os.Stderr, "Exporting data to %s (format: %s, range: %s)...\n", 
				outputFile, options.Format, options.TimeRange)
		}

		result, err := exporter.Export(options)
		if err != nil {
			return fmt.Errorf("export failed: %w", err)
		}

		// Print export summary
		fmt.Printf("Export completed successfully:\n")
		fmt.Printf("  Output file: %s\n", result.OutputFile)
		fmt.Printf("  Format: %s\n", result.Format)
		fmt.Printf("  Records exported: %d\n", result.RecordCount)
		fmt.Printf("  File size: %d bytes\n", result.FileSize)
		if result.Compressed {
			fmt.Printf("  Compression: enabled\n")
		}
		fmt.Printf("  Export duration: %v\n", result.Duration)

		return nil
	},
}

func init() {
	// Format flags
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "csv", "export format (csv, json, xlsx, parquet)")

	// Time range flags
	exportCmd.Flags().StringVarP(&exportRange, "range", "r", "all", "time range (today, week, month, year, all)")
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "start date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")
	exportCmd.Flags().StringVar(&exportTo, "to", "", "end date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)")

	// Processing flags
	exportCmd.Flags().BoolVar(&exportAggregate, "aggregate", false, "aggregate data by session")
	exportCmd.Flags().BoolVar(&exportCompress, "compress", false, "compress output file")
	exportCmd.Flags().BoolVar(&exportOverwrite, "overwrite", false, "overwrite existing files")

	// Template flags
	exportCmd.Flags().StringVar(&exportTemplate, "template", "", "custom export template file")

	// Bind to viper
	if err := viper.BindPFlag("export.default_format", exportCmd.Flags().Lookup("format")); err != nil {
		log.Printf("Failed to bind format flag: %v", err)
	}
	if err := viper.BindPFlag("export.compress", exportCmd.Flags().Lookup("compress")); err != nil {
		log.Printf("Failed to bind compress flag: %v", err)
	}

	rootCmd.AddCommand(exportCmd)
}

func validateOutputFile(outputFile string) error {
	// Check if file already exists
	if _, err := os.Stat(outputFile); err == nil && !exportOverwrite {
		return fmt.Errorf("file already exists: %s (use --overwrite to replace)", outputFile)
	}

	// Check if directory exists
	dir := filepath.Dir(outputFile)
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
	}

	// Check if we can write to the directory
	tempFile := filepath.Join(dir, ".clawcat_write_test")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %s", dir)
	}
	file.Close()
	os.Remove(tempFile)

	return nil
}

func validateExportOptions(options *internal.ExportOptions) error {
	// Validate format
	validFormats := []string{"csv", "json", "xlsx", "parquet"}
	found := false
	for _, format := range validFormats {
		if strings.EqualFold(options.Format, format) {
			options.Format = strings.ToLower(options.Format)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid format: %s (valid options: %s)", 
			options.Format, strings.Join(validFormats, ", "))
	}

	// Validate time range
	validRanges := []string{"today", "week", "month", "year", "all"}
	found = false
	for _, timeRange := range validRanges {
		if strings.EqualFold(options.TimeRange, timeRange) {
			options.TimeRange = strings.ToLower(options.TimeRange)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid time range: %s (valid options: %s)", 
			options.TimeRange, strings.Join(validRanges, ", "))
	}

	// Validate custom date range
	if options.FromTime != "" || options.ToTime != "" {
		if options.FromTime != "" {
			if _, err := parseTimeString(options.FromTime); err != nil {
				return fmt.Errorf("invalid from time: %s", options.FromTime)
			}
		}
		if options.ToTime != "" {
			if _, err := parseTimeString(options.ToTime); err != nil {
				return fmt.Errorf("invalid to time: %s", options.ToTime)
			}
		}
		// Override time range if custom dates provided
		options.TimeRange = "custom"
	}

	// Validate template file if provided
	if options.Template != "" {
		if _, err := os.Stat(options.Template); os.IsNotExist(err) {
			return fmt.Errorf("template file does not exist: %s", options.Template)
		}
	}

	// Auto-detect format from file extension if not explicitly set
	if exportFormat == "csv" { // Default value, might be auto-detected
		ext := strings.ToLower(filepath.Ext(options.OutputFile))
		switch ext {
		case ".json":
			options.Format = "json"
		case ".xlsx":
			options.Format = "xlsx"  
		case ".parquet":
			options.Format = "parquet"
		}
	}

	// Validate compression support
	if options.Compress {
		switch options.Format {
		case "json", "csv":
			// These formats support compression
		case "xlsx", "parquet":
			// These formats have built-in compression
			options.Compress = false
		}
	}

	return nil
}

// ExportResult contains the results of an export operation
type ExportResult struct {
	OutputFile   string
	Format       string
	RecordCount  int
	FileSize     int64
	Compressed   bool
	Duration     time.Duration
	Error        error
}