package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ValidationRule represents a single validation rule
type ValidationRule struct {
	Field   string
	Check   func(value interface{}) error
	Message string
}

// StandardValidator provides standard configuration validation
type StandardValidator struct {
	rules []ValidationRule
}

// NewStandardValidator creates a new standard validator with built-in rules
func NewStandardValidator() *StandardValidator {
	v := &StandardValidator{
		rules: make([]ValidationRule, 0),
	}

	// Add standard validation rules
	v.addStandardRules()
	return v
}

// AddRule adds a custom validation rule
func (v *StandardValidator) AddRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// Validate validates the entire configuration
func (v *StandardValidator) Validate(cfg *Config) error {
	var errors []string

	// Validate App config
	if err := v.validateApp(&cfg.App); err != nil {
		errors = append(errors, fmt.Sprintf("app: %v", err))
	}

	// Validate Data config
	if err := v.validateData(&cfg.Data); err != nil {
		errors = append(errors, fmt.Sprintf("data: %v", err))
	}

	// Validate UI config
	if err := v.validateUI(&cfg.UI); err != nil {
		errors = append(errors, fmt.Sprintf("ui: %v", err))
	}

	// Validate Performance config
	if err := v.validatePerformance(&cfg.Performance); err != nil {
		errors = append(errors, fmt.Sprintf("performance: %v", err))
	}

	// Validate Subscription config
	if err := v.validateSubscription(&cfg.Subscription); err != nil {
		errors = append(errors, fmt.Sprintf("subscription: %v", err))
	}

	// Validate Debug config
	if err := v.validateDebug(&cfg.Debug); err != nil {
		errors = append(errors, fmt.Sprintf("debug: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// addStandardRules adds the standard validation rules
func (v *StandardValidator) addStandardRules() {
	// These rules are applied during the specific validation functions
	// This method is kept for extensibility
}

// validateApp validates application configuration
func (v *StandardValidator) validateApp(app *AppConfig) error {
	var errors []string

	// Validate log level
	if err := ValidateLogLevel(app.LogLevel); err != nil {
		errors = append(errors, fmt.Sprintf("log_level: %v", err))
	}

	// Validate log file path if specified
	if app.LogFile != "" {
		dir := filepath.Dir(app.LogFile)
		if dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("log_file: directory does not exist: %s", dir))
			}
		}
	}

	// Validate timezone
	if app.Timezone != "" && app.Timezone != "Local" {
		if _, err := time.LoadLocation(app.Timezone); err != nil {
			errors = append(errors, fmt.Sprintf("timezone: invalid timezone: %s", app.Timezone))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// validateData validates data configuration
func (v *StandardValidator) validateData(data *DataConfig) error {
	var errors []string

	// Validate paths
	if err := ValidatePaths(data.Paths); err != nil {
		errors = append(errors, fmt.Sprintf("paths: %v", err))
	}

	// Validate watch interval
	if data.WatchInterval < 10*time.Millisecond {
		errors = append(errors, "watch_interval: must be at least 10ms")
	}
	if data.WatchInterval > 10*time.Second {
		errors = append(errors, "watch_interval: must not exceed 10s")
	}

	// Validate max file size
	if data.MaxFileSize < 1024 {
		errors = append(errors, "max_file_size: must be at least 1KB")
	}
	if data.MaxFileSize > 10*1024*1024*1024 {
		errors = append(errors, "max_file_size: must not exceed 10GB")
	}

	// Validate cache size
	if data.CacheSize < 0 {
		errors = append(errors, "cache_size: must be non-negative")
	}
	if data.CacheSize > 10*1024 {
		errors = append(errors, "cache_size: must not exceed 10GB")
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// validateUI validates UI configuration
func (v *StandardValidator) validateUI(ui *UIConfig) error {
	var errors []string

	// Validate theme
	if err := ValidateTheme(ui.Theme); err != nil {
		errors = append(errors, fmt.Sprintf("theme: %v", err))
	}

	// Validate refresh rate
	if ui.RefreshRate < 100*time.Millisecond {
		errors = append(errors, "refresh_rate: must be at least 100ms")
	}
	if ui.RefreshRate > time.Minute {
		errors = append(errors, "refresh_rate: must not exceed 1 minute")
	}

	// Validate chart height
	if ui.ChartHeight < 5 {
		errors = append(errors, "chart_height: must be at least 5")
	}
	if ui.ChartHeight > 50 {
		errors = append(errors, "chart_height: must not exceed 50")
	}

	// Validate table page size
	if ui.TablePageSize < 1 {
		errors = append(errors, "table_page_size: must be at least 1")
	}
	if ui.TablePageSize > 1000 {
		errors = append(errors, "table_page_size: must not exceed 1000")
	}

	// Validate date format
	if ui.DateFormat != "" {
		if _, err := time.Parse(ui.DateFormat, "2006-01-02"); err != nil {
			errors = append(errors, fmt.Sprintf("date_format: invalid format: %v", err))
		}
	}

	// Validate time format
	if ui.TimeFormat != "" {
		if _, err := time.Parse(ui.TimeFormat, "15:04:05"); err != nil {
			errors = append(errors, fmt.Sprintf("time_format: invalid format: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// validatePerformance validates performance configuration
func (v *StandardValidator) validatePerformance(perf *PerformanceConfig) error {
	var errors []string

	// Validate worker count
	if perf.WorkerCount < 1 {
		errors = append(errors, "worker_count: must be at least 1")
	}
	if perf.WorkerCount > 1000 {
		errors = append(errors, "worker_count: must not exceed 1000")
	}

	// Validate buffer size
	if perf.BufferSize < 1024 {
		errors = append(errors, "buffer_size: must be at least 1KB")
	}
	if perf.BufferSize > 100*1024*1024 {
		errors = append(errors, "buffer_size: must not exceed 100MB")
	}

	// Validate batch size
	if perf.BatchSize < 1 {
		errors = append(errors, "batch_size: must be at least 1")
	}
	if perf.BatchSize > 10000 {
		errors = append(errors, "batch_size: must not exceed 10000")
	}

	// Validate max memory
	if perf.MaxMemory < 10*1024*1024 {
		errors = append(errors, "max_memory: must be at least 10MB")
	}
	if perf.MaxMemory > 100*1024*1024*1024 {
		errors = append(errors, "max_memory: must not exceed 100GB")
	}

	// Validate GC interval
	if perf.GCInterval < 10*time.Second {
		errors = append(errors, "gc_interval: must be at least 10 seconds")
	}
	if perf.GCInterval > time.Hour {
		errors = append(errors, "gc_interval: must not exceed 1 hour")
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// validateSubscription validates subscription configuration
func (v *StandardValidator) validateSubscription(sub *SubscriptionConfig) error {
	var errors []string

	// Validate plan
	if err := ValidatePlan(sub.Plan); err != nil {
		errors = append(errors, fmt.Sprintf("plan: %v", err))
	}

	// Validate custom limits
	if sub.CustomTokenLimit < 0 {
		errors = append(errors, "custom_token_limit: must be non-negative")
	}
	if sub.CustomCostLimit < 0 {
		errors = append(errors, "custom_cost_limit: must be non-negative")
	}

	// Validate thresholds
	if sub.WarnThreshold < 0 || sub.WarnThreshold > 1 {
		errors = append(errors, "warn_threshold: must be between 0 and 1")
	}
	if sub.AlertThreshold < 0 || sub.AlertThreshold > 1 {
		errors = append(errors, "alert_threshold: must be between 0 and 1")
	}
	if sub.WarnThreshold >= sub.AlertThreshold {
		errors = append(errors, "warn_threshold: must be less than alert_threshold")
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// validateDebug validates debug configuration
func (v *StandardValidator) validateDebug(debug *DebugConfig) error {
	var errors []string

	// Validate trace file path if specified
	if debug.TraceFile != "" {
		dir := filepath.Dir(debug.TraceFile)
		if dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("trace_file: directory does not exist: %s", dir))
			}
		}
	}

	// Validate metrics port
	if debug.MetricsPort != 0 {
		if debug.MetricsPort < 1024 || debug.MetricsPort > 65535 {
			errors = append(errors, "metrics_port: must be between 1024 and 65535")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

// Built-in validation functions

// ValidatePlan validates subscription plan
func ValidatePlan(plan string) error {
	validPlans := map[string]bool{
		"free": true,
		"pro":  true,
		"team": true,
	}

	if !validPlans[plan] {
		return fmt.Errorf("invalid plan: %s (valid: free, pro, team)", plan)
	}
	return nil
}

// ValidateTheme validates UI theme
func ValidateTheme(theme string) error {
	validThemes := map[string]bool{
		"dark":          true,
		"light":         true,
		"high-contrast": true,
		"auto":          true,
	}

	if !validThemes[theme] {
		return fmt.Errorf("invalid theme: %s (valid: dark, light, high-contrast, auto)", theme)
	}
	return nil
}

// ValidateLogLevel validates log level
func ValidateLogLevel(level string) error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}

	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error, fatal)", level)
	}
	return nil
}

// ValidatePaths validates data paths
func ValidatePaths(paths []string) error {
	if len(paths) == 0 {
		return nil // Empty paths are allowed, auto-discovery will be used
	}

	for i, path := range paths {
		if path == "" {
			return fmt.Errorf("path %d: empty path not allowed", i)
		}

		// Expand environment variables
		expandedPath := os.ExpandEnv(path)

		// Check if path exists (allow both files and directories)
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			return fmt.Errorf("path %d: does not exist: %s", i, expandedPath)
		}
	}

	return nil
}