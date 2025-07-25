package internal

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

// Logger provides structured logging functionality
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

// NewLogger creates a new logger with the specified level and log file
func NewLogger(levelStr string, logFile string) *Logger {
	level := parseLogLevel(levelStr)
	
	var file *os.File
	var err error
	
	if logFile != "" {
		file, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			panic(fmt.Sprintf("Failed to open log file %s: %v", logFile, err))
		}
	} else {
		panic("Log file must be specified")
	}
	
	logger := log.New(file, "", log.LstdFlags|log.Lshortfile)
	
	return &Logger{
		level:  level,
		logger: logger,
	}
}

// parseLogLevel parses a log level string
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LogDebug
	case "info":
		return LogInfo
	case "warn", "warning":
		return LogWarn
	case "error":
		return LogError
	default:
		return LogInfo
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	if l.level <= LogDebug {
		l.logger.Printf("[DEBUG] %s", msg)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= LogDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	if l.level <= LogInfo {
		l.logger.Printf("[INFO] %s", msg)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= LogInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	if l.level <= LogWarn {
		l.logger.Printf("[WARN] %s", msg)
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= LogWarn {
		l.logger.Printf("[WARN] "+format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	if l.level <= LogError {
		l.logger.Printf("[ERROR] %s", msg)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= LogError {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(msg string) {
	l.logger.Printf("[FATAL] %s", msg)
	os.Exit(1)
}

// Fatalf logs a formatted fatal error and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logger.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}