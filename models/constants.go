package models

import "time"

// Session and gap duration constants
const (
	SessionDuration = 5 * time.Hour
	MaxGapDuration  = 5 * time.Hour
)

// Model identifiers
const (
	ModelOpus   = "claude-3-opus-20240229"
	ModelSonnet = "claude-3-5-sonnet-20241022"
	ModelHaiku  = "claude-3-5-haiku-20241022"
)

// Plan identifiers
const (
	PlanPro   = "pro"
	PlanMax5  = "max5"
	PlanMax20 = "max20"
)

// View refresh rates
const (
	DefaultRefreshInterval = 1 * time.Second
	MinRefreshInterval     = 100 * time.Millisecond
	MaxRefreshInterval     = 10 * time.Second
)

// Performance limits
const (
	MaxConcurrentSessions = 100
	MaxEntriesPerSession  = 10000
	DefaultCacheSize      = 1000
)

// File paths and patterns
const (
	DefaultLogFileName    = "claude_code_usage.jsonl"
	LogFileSearchPattern  = "*claude_code_usage*.jsonl"
	ConfigFileName        = ".clawcat.yml"
)

// Time formats
const (
	DisplayTimeFormat = "2006-01-02 15:04:05"
	FileTimeFormat    = "20060102-150405"
)

// UI constants
const (
	DefaultTerminalWidth  = 80
	DefaultTerminalHeight = 24
	MinTerminalWidth      = 60
	MinTerminalHeight     = 10
)