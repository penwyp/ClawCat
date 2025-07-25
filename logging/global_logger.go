package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging functionality
type Logger struct {
	level  LogLevel
	logger *log.Logger
}

var (
	globalLogger *Logger
	loggerOnce   sync.Once
)

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
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] %s", msg)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] %s", msg)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] %s", msg)
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] "+format, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	if l.level <= LevelError {
		l.logger.Printf("[ERROR] %s", msg)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= LevelError {
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

// InitGlobalLogger initializes the global logger instance
func InitGlobalLogger(logLevel, logFile string) {
	loggerOnce.Do(func() {
		globalLogger = NewLogger(logLevel, logFile)
	})
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		panic("Global logger not initialized. Call InitGlobalLogger first.")
	}
	return globalLogger
}

// Global convenience functions for logging
func LogInfo(msg string) {
	if globalLogger != nil {
		globalLogger.Info(msg)
	}
}

func LogInfof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

func LogDebug(msg string) {
	if globalLogger != nil {
		globalLogger.Debug(msg)
	}
}

func LogDebugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

func LogWarn(msg string) {
	if globalLogger != nil {
		globalLogger.Warn(msg)
	}
}

func LogWarnf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

func LogError(msg string) {
	if globalLogger != nil {
		globalLogger.Error(msg)
	}
}

func LogErrorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}