# Module: config

## Overview
The config package manages application configuration through multiple sources including files, environment variables, and command-line flags. It supports hot-reloading, validation, and type-safe access to configuration values.

## Package Structure
```
config/
├── config.go       # Main configuration structure
├── loader.go       # Configuration loading logic
├── validator.go    # Configuration validation
├── env.go          # Environment variable handling
├── defaults.go     # Default configurations
├── watcher.go      # Configuration hot-reload
└── *_test.go       # Configuration tests
```

## Core Components

### Configuration Structure
Comprehensive application configuration.

```go
type Config struct {
    // Application
    App AppConfig `yaml:"app" json:"app"`
    
    // Data Sources
    Data DataConfig `yaml:"data" json:"data"`
    
    // User Interface
    UI UIConfig `yaml:"ui" json:"ui"`
    
    // Performance
    Performance PerformanceConfig `yaml:"performance" json:"performance"`
    
    // Subscription
    Subscription SubscriptionConfig `yaml:"subscription" json:"subscription"`
    
    // Debug
    Debug DebugConfig `yaml:"debug" json:"debug"`
}

type AppConfig struct {
    Name        string        `yaml:"name" json:"name"`
    Version     string        `yaml:"version" json:"version"`
    LogLevel    string        `yaml:"log_level" json:"log_level"`
    LogFile     string        `yaml:"log_file" json:"log_file"`
    Timezone    string        `yaml:"timezone" json:"timezone"`
}

type DataConfig struct {
    Paths           []string      `yaml:"paths" json:"paths"`
    AutoDiscover    bool          `yaml:"auto_discover" json:"auto_discover"`
    WatchInterval   time.Duration `yaml:"watch_interval" json:"watch_interval"`
    MaxFileSize     int64         `yaml:"max_file_size" json:"max_file_size"`
    CacheEnabled    bool          `yaml:"cache_enabled" json:"cache_enabled"`
    CacheSize       int           `yaml:"cache_size" json:"cache_size"`
}

type UIConfig struct {
    Theme           string        `yaml:"theme" json:"theme"`
    RefreshRate     time.Duration `yaml:"refresh_rate" json:"refresh_rate"`
    CompactMode     bool          `yaml:"compact_mode" json:"compact_mode"`
    ShowSpinner     bool          `yaml:"show_spinner" json:"show_spinner"`
    ChartHeight     int           `yaml:"chart_height" json:"chart_height"`
    TablePageSize   int           `yaml:"table_page_size" json:"table_page_size"`
    DateFormat      string        `yaml:"date_format" json:"date_format"`
    TimeFormat      string        `yaml:"time_format" json:"time_format"`
}

type PerformanceConfig struct {
    WorkerCount     int           `yaml:"worker_count" json:"worker_count"`
    BufferSize      int           `yaml:"buffer_size" json:"buffer_size"`
    BatchSize       int           `yaml:"batch_size" json:"batch_size"`
    MaxMemory       int64         `yaml:"max_memory" json:"max_memory"`
    GCInterval      time.Duration `yaml:"gc_interval" json:"gc_interval"`
}

type SubscriptionConfig struct {
    Plan            string        `yaml:"plan" json:"plan"`
    CustomTokenLimit int          `yaml:"custom_token_limit" json:"custom_token_limit"`
    CustomCostLimit  float64      `yaml:"custom_cost_limit" json:"custom_cost_limit"`
    WarnThreshold    float64      `yaml:"warn_threshold" json:"warn_threshold"`
    AlertThreshold   float64      `yaml:"alert_threshold" json:"alert_threshold"`
}

type DebugConfig struct {
    Enabled         bool          `yaml:"enabled" json:"enabled"`
    ProfileCPU      bool          `yaml:"profile_cpu" json:"profile_cpu"`
    ProfileMemory   bool          `yaml:"profile_memory" json:"profile_memory"`
    TraceFile       string        `yaml:"trace_file" json:"trace_file"`
    MetricsPort     int           `yaml:"metrics_port" json:"metrics_port"`
}
```

### Configuration Loader
Multi-source configuration loading with precedence.

```go
type Loader struct {
    sources     []Source
    validators  []Validator
    merger      Merger
}

type Source interface {
    Name() string
    Load() (*Config, error)
    Priority() int
}

type FileSource struct {
    path   string
    format Format
}

type EnvSource struct {
    prefix string
}

type FlagSource struct {
    flags *pflag.FlagSet
}

type Format int
const (
    FormatYAML Format = iota
    FormatJSON
    FormatTOML
)

func NewLoader() *Loader
func (l *Loader) AddSource(source Source)
func (l *Loader) Load() (*Config, error)
func (l *Loader) LoadWithDefaults() (*Config, error)
```

### Configuration Validator
Comprehensive validation rules.

```go
type Validator interface {
    Validate(cfg *Config) error
}

type ValidationRule struct {
    Field    string
    Check    func(value interface{}) error
    Message  string
}

type StandardValidator struct {
    rules []ValidationRule
}

func NewStandardValidator() *StandardValidator
func (v *StandardValidator) AddRule(rule ValidationRule)
func (v *StandardValidator) Validate(cfg *Config) error

// Built-in validators
func ValidatePlan(plan string) error
func ValidateTheme(theme string) error
func ValidateLogLevel(level string) error
func ValidatePaths(paths []string) error
```

### Environment Variable Handling
Structured environment variable support.

```go
type EnvMapper struct {
    prefix   string
    mappings map[string]string
}

func NewEnvMapper(prefix string) *EnvMapper
func (e *EnvMapper) Map(envKey, configPath string)
func (e *EnvMapper) Apply(cfg *Config) error

// Standard mappings
var StandardEnvMappings = map[string]string{
    "CLAWCAT_PLAN":         "subscription.plan",
    "CLAWCAT_THEME":        "ui.theme",
    "CLAWCAT_LOG_LEVEL":    "app.log_level",
    "CLAWCAT_DATA_PATHS":   "data.paths",
    "CLAWCAT_CACHE_SIZE":   "data.cache_size",
}
```

### Configuration Watcher
Hot-reload configuration changes.

```go
type Watcher struct {
    path      string
    config    *Config
    onChange  func(*Config)
    watcher   *fsnotify.Watcher
    stopCh    chan struct{}
}

func NewWatcher(path string, onChange func(*Config)) (*Watcher, error)
func (w *Watcher) Start() error
func (w *Watcher) Stop() error
func (w *Watcher) Current() *Config
```

## Default Configurations

```go
func DefaultConfig() *Config {
    return &Config{
        App: AppConfig{
            Name:     "claudecat",
            Version:  Version,
            LogLevel: "info",
            Timezone: "Local",
        },
        Data: DataConfig{
            AutoDiscover:  true,
            WatchInterval: 100 * time.Millisecond,
            MaxFileSize:   100 * 1024 * 1024, // 100MB
            CacheEnabled:  true,
            CacheSize:     50, // 50MB
        },
        UI: UIConfig{
            Theme:         "dark",
            RefreshRate:   time.Second,
            CompactMode:   false,
            ShowSpinner:   true,
            ChartHeight:   10,
            TablePageSize: 20,
            DateFormat:    "2006-01-02",
            TimeFormat:    "15:04:05",
        },
        Performance: PerformanceConfig{
            WorkerCount: runtime.NumCPU(),
            BufferSize:  64 * 1024, // 64KB
            BatchSize:   100,
            MaxMemory:   500 * 1024 * 1024, // 500MB
            GCInterval:  5 * time.Minute,
        },
        Subscription: SubscriptionConfig{
            Plan:          "pro",
            WarnThreshold: 0.80, // 80%
            AlertThreshold: 0.95, // 95%
        },
        Debug: DebugConfig{
            Enabled: false,
        },
    }
}

func MinimalConfig() *Config
func DevelopmentConfig() *Config
func ProductionConfig() *Config
```

## Configuration Files

### File Locations
```go
func ConfigPaths() []string {
    return []string{
        "./claudecat.yaml",
        "$HOME/.config/claudecat/config.yaml",
        "$HOME/.claudecat/config.yaml",
        "/etc/claudecat/config.yaml",
    }
}
```

### Example Configuration
```yaml
app:
  log_level: info
  timezone: America/New_York

data:
  paths:
    - ~/.claude/projects
    - ~/.config/claude/projects
  auto_discover: true
  cache_enabled: true
  cache_size: 100

ui:
  theme: dark
  refresh_rate: 1s
  compact_mode: false
  chart_height: 12

subscription:
  plan: pro
  warn_threshold: 0.85
  alert_threshold: 0.95

performance:
  worker_count: 8
  buffer_size: 65536
  batch_size: 200
```

## Usage Example

```go
package main

import (
    "github.com/penwyp/claudecat/config"
    "log"
)

func main() {
    // Load configuration
    loader := config.NewLoader()
    loader.AddSource(config.NewFileSource("./claudecat.yaml"))
    loader.AddSource(config.NewEnvSource("CLAWCAT"))
    loader.AddSource(config.NewFlagSource())
    
    cfg, err := loader.LoadWithDefaults()
    if err != nil {
        log.Fatal(err)
    }
    
    // Watch for changes
    watcher, err := config.NewWatcher("./claudecat.yaml", func(newCfg *config.Config) {
        log.Println("Configuration reloaded")
    })
    if err == nil {
        watcher.Start()
        defer watcher.Stop()
    }
    
    // Use configuration
    log.Printf("Using theme: %s", cfg.UI.Theme)
    log.Printf("Token limit: %d", cfg.Subscription.CustomTokenLimit)
}
```

## Testing Strategy

1. **Unit Tests**:
   - Configuration loading from all sources
   - Validation rules
   - Environment variable mapping
   - Default values

2. **Integration Tests**:
   - Multi-source merging
   - Hot-reload functionality
   - File format parsing
   - Invalid configuration handling

3. **Scenarios**:
   - Missing configuration files
   - Partial configurations
   - Invalid values
   - Permission errors