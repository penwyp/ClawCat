package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Source represents a configuration source
type Source interface {
	Name() string
	Load() (*Config, error)
	Priority() int
}

// Validator validates configuration
type Validator interface {
	Validate(cfg *Config) error
}

// Merger merges configurations from multiple sources
type Merger interface {
	Merge(base, override *Config) *Config
}

// Loader loads configuration from multiple sources
type Loader struct {
	sources    []Source
	validators []Validator
	merger     Merger
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		sources:    make([]Source, 0),
		validators: make([]Validator, 0),
		merger:     &DefaultMerger{},
	}
}

// AddSource adds a configuration source
func (l *Loader) AddSource(source Source) {
	l.sources = append(l.sources, source)
}

// AddValidator adds a configuration validator
func (l *Loader) AddValidator(validator Validator) {
	l.validators = append(l.validators, validator)
}

// SetMerger sets the configuration merger
func (l *Loader) SetMerger(merger Merger) {
	l.merger = merger
}

// Load loads configuration from all sources
func (l *Loader) Load() (*Config, error) {
	// Sort sources by priority
	sort.Slice(l.sources, func(i, j int) bool {
		return l.sources[i].Priority() < l.sources[j].Priority()
	})

	var config *Config
	for _, source := range l.sources {
		cfg, err := source.Load()
		if err != nil {
			// Log error but continue with other sources
			continue
		}

		if config == nil {
			config = cfg
		} else {
			config = l.merger.Merge(config, cfg)
		}
	}

	if config == nil {
		return nil, fmt.Errorf("no valid configuration sources found")
	}

	// Validate final configuration
	for _, validator := range l.validators {
		if err := validator.Validate(config); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	return config, nil
}

// LoadWithDefaults loads configuration with defaults as base
func (l *Loader) LoadWithDefaults() (*Config, error) {
	defaultConfig := DefaultConfig()

	// Sort sources by priority
	sort.Slice(l.sources, func(i, j int) bool {
		return l.sources[i].Priority() < l.sources[j].Priority()
	})

	config := defaultConfig
	for _, source := range l.sources {
		cfg, err := source.Load()
		if err != nil {
			// Log error but continue with other sources
			continue
		}

		config = l.merger.Merge(config, cfg)
	}

	// Validate final configuration
	for _, validator := range l.validators {
		if err := validator.Validate(config); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	return config, nil
}

// FileSource loads configuration from a file
type FileSource struct {
	path   string
	format Format
}

// NewFileSource creates a new file configuration source
func NewFileSource(path string) *FileSource {
	format := FormatYAML
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		format = FormatJSON
	case ".toml":
		format = FormatTOML
	case ".yaml", ".yml":
		format = FormatYAML
	}

	return &FileSource{
		path:   path,
		format: format,
	}
}

// Name returns the source name
func (f *FileSource) Name() string {
	return fmt.Sprintf("file:%s", f.path)
}

// Priority returns the source priority (lower = higher priority)
func (f *FileSource) Priority() int {
	return 100
}

// Load loads configuration from the file
func (f *FileSource) Load() (*Config, error) {
	// Expand environment variables in path
	expandedPath := os.ExpandEnv(f.path)

	// Check if file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", expandedPath)
	}

	v := viper.New()
	v.SetConfigFile(expandedPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", expandedPath, err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from %s: %w", expandedPath, err)
	}

	return &config, nil
}

// EnvSource loads configuration from environment variables
type EnvSource struct {
	prefix string
}

// NewEnvSource creates a new environment variable configuration source
func NewEnvSource(prefix string) *EnvSource {
	return &EnvSource{
		prefix: prefix,
	}
}

// Name returns the source name
func (e *EnvSource) Name() string {
	return fmt.Sprintf("env:%s", e.prefix)
}

// Priority returns the source priority (lower = higher priority)
func (e *EnvSource) Priority() int {
	return 200
}

// Load loads configuration from environment variables
func (e *EnvSource) Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(e.prefix)
	v.AutomaticEnv()

	// Replace dots and dashes with underscores for env vars
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Set all possible config keys to enable env var reading
	e.setAllKeys(v)

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from environment: %w", err)
	}

	return &config, nil
}

// setAllKeys sets all possible configuration keys for environment variable reading
func (e *EnvSource) setAllKeys(v *viper.Viper) {
	// App config
	v.SetDefault("app.name", "")
	v.SetDefault("app.version", "")
	v.SetDefault("app.log_level", "")
	v.SetDefault("app.log_file", "")
	v.SetDefault("app.timezone", "")

	// Data config
	v.SetDefault("data.paths", []string{})
	v.SetDefault("data.auto_discover", false)
	v.SetDefault("data.watch_interval", "")
	v.SetDefault("data.max_file_size", 0)
	v.SetDefault("data.cache_enabled", false)
	v.SetDefault("data.cache_size", 0)

	// UI config
	v.SetDefault("ui.theme", "")
	v.SetDefault("ui.refresh_rate", "")
	v.SetDefault("ui.compact_mode", false)
	v.SetDefault("ui.show_spinner", false)
	v.SetDefault("ui.chart_height", 0)
	v.SetDefault("ui.table_page_size", 0)
	v.SetDefault("ui.date_format", "")
	v.SetDefault("ui.time_format", "")

	// Performance config
	v.SetDefault("performance.worker_count", 0)
	v.SetDefault("performance.buffer_size", 0)
	v.SetDefault("performance.batch_size", 0)
	v.SetDefault("performance.max_memory", 0)
	v.SetDefault("performance.gc_interval", "")

	// Subscription config
	v.SetDefault("subscription.plan", "")
	v.SetDefault("subscription.custom_token_limit", 0)
	v.SetDefault("subscription.custom_cost_limit", 0.0)
	v.SetDefault("subscription.warn_threshold", 0.0)
	v.SetDefault("subscription.alert_threshold", 0.0)

	// Debug config
	v.SetDefault("debug.enabled", false)
	v.SetDefault("debug.profile_cpu", false)
	v.SetDefault("debug.profile_memory", false)
	v.SetDefault("debug.trace_file", "")
	v.SetDefault("debug.metrics_port", 0)
}

// FlagSource loads configuration from command-line flags
type FlagSource struct {
	flags *pflag.FlagSet
}

// NewFlagSource creates a new flag configuration source
func NewFlagSource(flags *pflag.FlagSet) *FlagSource {
	return &FlagSource{
		flags: flags,
	}
}

// Name returns the source name
func (f *FlagSource) Name() string {
	return "flags"
}

// Priority returns the source priority (lower = higher priority)
func (f *FlagSource) Priority() int {
	return 300
}

// Load loads configuration from command-line flags
func (f *FlagSource) Load() (*Config, error) {
	config := &Config{}
	
	// Handle flags that are bound to nested config fields
	f.flags.VisitAll(func(flag *pflag.Flag) {
		if !flag.Changed {
			return
		}
		
		switch flag.Name {
		case "debug":
			if val, err := f.flags.GetBool("debug"); err == nil {
				config.Debug.Enabled = val
			}
		case "log-level":
			if val, err := f.flags.GetString("log-level"); err == nil {
				config.App.LogLevel = val
			}
		case "no-color":
			if val, err := f.flags.GetBool("no-color"); err == nil {
				config.UI.NoColor = val
			}
		case "verbose":
			if val, err := f.flags.GetBool("verbose"); err == nil {
				config.App.Verbose = val
			}
		}
	})

	return config, nil
}

// DefaultMerger is the default configuration merger
type DefaultMerger struct{}

// Merge merges two configurations, with override taking precedence
func (m *DefaultMerger) Merge(base, override *Config) *Config {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	result := *base

	// Merge App config
	if override.App.Name != "" {
		result.App.Name = override.App.Name
	}
	if override.App.Version != "" {
		result.App.Version = override.App.Version
	}
	if override.App.LogLevel != "" {
		result.App.LogLevel = override.App.LogLevel
	}
	if override.App.LogFile != "" {
		result.App.LogFile = override.App.LogFile
	}
	if override.App.Timezone != "" {
		result.App.Timezone = override.App.Timezone
	}

	// Merge Data config
	if len(override.Data.Paths) > 0 {
		result.Data.Paths = override.Data.Paths
	}
	if override.Data.WatchInterval > 0 {
		result.Data.WatchInterval = override.Data.WatchInterval
	}
	if override.Data.MaxFileSize > 0 {
		result.Data.MaxFileSize = override.Data.MaxFileSize
	}
	if override.Data.CacheSize > 0 {
		result.Data.CacheSize = override.Data.CacheSize
	}

	// Merge UI config
	if override.UI.Theme != "" {
		result.UI.Theme = override.UI.Theme
	}
	if override.UI.RefreshRate > 0 {
		result.UI.RefreshRate = override.UI.RefreshRate
	}
	if override.UI.ChartHeight > 0 {
		result.UI.ChartHeight = override.UI.ChartHeight
	}
	if override.UI.TablePageSize > 0 {
		result.UI.TablePageSize = override.UI.TablePageSize
	}
	if override.UI.DateFormat != "" {
		result.UI.DateFormat = override.UI.DateFormat
	}
	if override.UI.TimeFormat != "" {
		result.UI.TimeFormat = override.UI.TimeFormat
	}

	// Merge Performance config
	if override.Performance.WorkerCount > 0 {
		result.Performance.WorkerCount = override.Performance.WorkerCount
	}
	if override.Performance.BufferSize > 0 {
		result.Performance.BufferSize = override.Performance.BufferSize
	}
	if override.Performance.BatchSize > 0 {
		result.Performance.BatchSize = override.Performance.BatchSize
	}
	if override.Performance.MaxMemory > 0 {
		result.Performance.MaxMemory = override.Performance.MaxMemory
	}
	if override.Performance.GCInterval > 0 {
		result.Performance.GCInterval = override.Performance.GCInterval
	}

	// Merge Subscription config
	if override.Subscription.Plan != "" {
		result.Subscription.Plan = override.Subscription.Plan
	}
	if override.Subscription.CustomTokenLimit > 0 {
		result.Subscription.CustomTokenLimit = override.Subscription.CustomTokenLimit
	}
	if override.Subscription.CustomCostLimit > 0 {
		result.Subscription.CustomCostLimit = override.Subscription.CustomCostLimit
	}
	if override.Subscription.WarnThreshold > 0 {
		result.Subscription.WarnThreshold = override.Subscription.WarnThreshold
	}
	if override.Subscription.AlertThreshold > 0 {
		result.Subscription.AlertThreshold = override.Subscription.AlertThreshold
	}

	// Merge Debug config (boolean fields always override)
	result.Debug = override.Debug

	return &result
}
