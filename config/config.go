package config

import (
	"runtime"
	"time"
)

// Config represents the complete application configuration
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

	// Limits
	Limits LimitsConfig `yaml:"limits" json:"limits"`

	// Debug
	Debug DebugConfig `yaml:"debug" json:"debug"`
}

// AppConfig contains general application settings
type AppConfig struct {
	Name     string `yaml:"name" json:"name"`
	Version  string `yaml:"version" json:"version"`
	LogLevel string `yaml:"log_level" json:"log_level"`
	LogFile  string `yaml:"log_file" json:"log_file"`
	Timezone string `yaml:"timezone" json:"timezone"`
}

// DataConfig contains data source and processing settings
type DataConfig struct {
	Paths         []string      `yaml:"paths" json:"paths"`
	AutoDiscover  bool          `yaml:"auto_discover" json:"auto_discover"`
	WatchInterval time.Duration `yaml:"watch_interval" json:"watch_interval"`
	MaxFileSize   int64         `yaml:"max_file_size" json:"max_file_size"`
	CacheEnabled  bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheSize     int           `yaml:"cache_size" json:"cache_size"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	Theme         string        `yaml:"theme" json:"theme"`
	RefreshRate   time.Duration `yaml:"refresh_rate" json:"refresh_rate"`
	CompactMode   bool          `yaml:"compact_mode" json:"compact_mode"`
	ShowSpinner   bool          `yaml:"show_spinner" json:"show_spinner"`
	ChartHeight   int           `yaml:"chart_height" json:"chart_height"`
	TablePageSize int           `yaml:"table_page_size" json:"table_page_size"`
	DateFormat    string        `yaml:"date_format" json:"date_format"`
	TimeFormat    string        `yaml:"time_format" json:"time_format"`
}

// PerformanceConfig contains performance tuning settings
type PerformanceConfig struct {
	WorkerCount int           `yaml:"worker_count" json:"worker_count"`
	BufferSize  int           `yaml:"buffer_size" json:"buffer_size"`
	BatchSize   int           `yaml:"batch_size" json:"batch_size"`
	MaxMemory   int64         `yaml:"max_memory" json:"max_memory"`
	GCInterval  time.Duration `yaml:"gc_interval" json:"gc_interval"`
}

// SubscriptionConfig contains subscription and limit settings
type SubscriptionConfig struct {
	Plan             string  `yaml:"plan" json:"plan"`
	CustomTokenLimit int     `yaml:"custom_token_limit" json:"custom_token_limit"`
	CustomCostLimit  float64 `yaml:"custom_cost_limit" json:"custom_cost_limit"`
	WarnThreshold    float64 `yaml:"warn_threshold" json:"warn_threshold"`
	AlertThreshold   float64 `yaml:"alert_threshold" json:"alert_threshold"`
}

// DebugConfig contains debugging and profiling settings
type DebugConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	ProfileCPU    bool   `yaml:"profile_cpu" json:"profile_cpu"`
	ProfileMemory bool   `yaml:"profile_memory" json:"profile_memory"`
	TraceFile     string `yaml:"trace_file" json:"trace_file"`
	MetricsPort   int    `yaml:"metrics_port" json:"metrics_port"`
}

// LimitsConfig contains subscription limit settings
type LimitsConfig struct {
	Enabled       bool               `yaml:"enabled" json:"enabled"`
	Notifications []NotificationType `yaml:"notifications" json:"notifications"`
	WebhookURL    string             `yaml:"webhook_url" json:"webhook_url"`
	EmailEnabled  bool               `yaml:"email_enabled" json:"email_enabled"`
	EmailSMTP     SMTPConfig         `yaml:"email_smtp" json:"email_smtp"`
}

// NotificationType represents the type of notification
type NotificationType string

const (
	NotifyDesktop NotificationType = "desktop"
	NotifySound   NotificationType = "sound"
	NotifyWebhook NotificationType = "webhook"
	NotifyEmail   NotificationType = "email"
)

// SMTPConfig contains SMTP settings for email notifications
type SMTPConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	From     string `yaml:"from" json:"from"`
	To       string `yaml:"to" json:"to"`
}

// Format represents configuration file format
type Format int

const (
	FormatYAML Format = iota
	FormatJSON
	FormatTOML
)

// ConfigPaths returns the default configuration file paths in order of precedence
func ConfigPaths() []string {
	return []string{
		"./clawcat.yaml",
		"$HOME/.config/clawcat/config.yaml",
		"$HOME/.clawcat/config.yaml",
		"/etc/clawcat/config.yaml",
	}
}

// Version will be set at build time
var Version = "dev"

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:     "ClawCat",
			Version:  Version,
			LogLevel: "info",
			LogFile:  "clawcat.log",
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
			Plan:           "pro",
			WarnThreshold:  0.80, // 80%
			AlertThreshold: 0.95, // 95%
		},
		Limits: LimitsConfig{
			Enabled:       true,
			Notifications: []NotificationType{NotifyDesktop},
		},
		Debug: DebugConfig{
			Enabled: false,
		},
	}
}

// MinimalConfig returns a minimal configuration for basic operation
func MinimalConfig() *Config {
	cfg := DefaultConfig()
	cfg.Data.CacheEnabled = false
	cfg.Performance.WorkerCount = 1
	cfg.Performance.BufferSize = 1024
	cfg.Performance.BatchSize = 10
	cfg.UI.CompactMode = true
	cfg.UI.ShowSpinner = false
	return cfg
}

// DevelopmentConfig returns a configuration optimized for development
func DevelopmentConfig() *Config {
	cfg := DefaultConfig()
	cfg.App.LogLevel = "debug"
	cfg.Debug.Enabled = true
	cfg.UI.RefreshRate = 500 * time.Millisecond
	cfg.Performance.GCInterval = time.Minute
	return cfg
}

// ProductionConfig returns a configuration optimized for production
func ProductionConfig() *Config {
	cfg := DefaultConfig()
	cfg.App.LogLevel = "warn"
	cfg.Debug.Enabled = false
	cfg.Performance.WorkerCount = runtime.NumCPU() * 2
	cfg.Performance.MaxMemory = 1024 * 1024 * 1024 // 1GB
	cfg.Performance.GCInterval = 10 * time.Minute
	return cfg
}