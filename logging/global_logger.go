package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelPanic
)

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// LogFormat represents the output format
type LogFormat string

const (
	FormatText LogFormat = "text"
	FormatJSON LogFormat = "json"
)

// Output represents a log output destination
type Output interface {
	Write(entry LogEntry) error
	Close() error
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	level   LogLevel
	outputs []Output
	fields  map[string]interface{}
	format  LogFormat
	mu      sync.RWMutex
}

// LoggerInterface defines the public interface for logging
type LoggerInterface interface {
	Debug(msg string, fields ...Field)
	Debugf(format string, args ...interface{})
	Info(msg string, fields ...Field)
	Infof(format string, args ...interface{})
	Warn(msg string, fields ...Field)
	Warnf(format string, args ...interface{})
	Error(msg string, fields ...Field)
	Errorf(format string, args ...interface{})
	Fatal(msg string, fields ...Field)
	Fatalf(format string, args ...interface{})
	With(fields ...Field) LoggerInterface
	WithContext(ctx context.Context) LoggerInterface
	SetLevel(level LogLevel)
	AddOutput(output Output)
}

var (
	globalLogger LoggerInterface
	loggerOnce   sync.Once
)

// NewLogger creates a new logger with the specified level and log file
func NewLogger(levelStr string, logFile string) *Logger {
	return NewLoggerWithDebug(levelStr, logFile, false)
}

// NewLoggerWithDebug creates a new logger with optional console output for debug mode
func NewLoggerWithDebug(levelStr string, logFile string, debugToConsole bool) *Logger {
	level := parseLogLevel(levelStr)

	logger := &Logger{
		level:   level,
		outputs: make([]Output, 0),
		fields:  make(map[string]interface{}),
		format:  FormatText,
	}

	// Add appropriate output based on debug mode
	if debugToConsole {
		logger.AddOutput(NewConsoleOutput(os.Stderr, FormatText))
	}

	if logFile != "" {
		fileOutput, err := NewFileOutput(logFile, FormatText)
		if err != nil {
			panic(fmt.Sprintf("Failed to create file output for %s: %v", logFile, err))
		}
		logger.AddOutput(fileOutput)
	} else if !debugToConsole {
		panic("Log file must be specified when not in debug mode")
	}

	return logger
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
	case "fatal":
		return LevelFatal
	case "panic":
		return LevelPanic
	default:
		return LevelInfo
	}
}

// levelToString converts LogLevel to string
func levelToString(level LogLevel) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	case LevelPanic:
		return "PANIC"
	default:
		return "UNKNOWN"
	}
}

// log writes a log entry to all outputs
func (l *Logger) log(level LogLevel, msg string, fields ...Field) {
	if l.level > level {
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     levelToString(level),
		Message:   msg,
		Fields:    make(map[string]interface{}),
	}

	// Copy logger fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Add provided fields
	for _, field := range fields {
		entry.Fields[field.Key] = field.Value
	}

	// Write to all outputs
	for _, output := range l.outputs {
		if err := output.Write(entry); err != nil {
			// Log to stderr if we can't write to an output
			log.Printf("Failed to write log entry: %v", err)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(LevelDebug, fmt.Sprintf(format, args...))
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(LevelInfo, fmt.Sprintf(format, args...))
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(LevelWarn, fmt.Sprintf(format, args...))
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, args...))
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(msg string, fields ...Field) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

// Fatalf logs a formatted fatal error and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// With returns a new logger with additional fields
func (l *Logger) With(fields ...Field) LoggerInterface {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(map[string]interface{})
	// Copy existing fields
	for k, v := range l.fields {
		newFields[k] = v
	}
	// Add new fields
	for _, field := range fields {
		newFields[field.Key] = field.Value
	}

	return &Logger{
		level:   l.level,
		outputs: l.outputs,
		fields:  newFields,
		format:  l.format,
	}
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) LoggerInterface {
	// Extract common context values
	fields := []Field{}

	// Add trace ID if present
	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields = append(fields, Field{Key: "trace_id", Value: traceID})
	}

	// Add user ID if present
	if userID := ctx.Value("user_id"); userID != nil {
		fields = append(fields, Field{Key: "user_id", Value: userID})
	}

	return l.With(fields...)
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// AddOutput adds a new output destination
func (l *Logger) AddOutput(output Output) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, output)
}

// InitGlobalLogger initializes the global logger instance
func InitGlobalLogger(logLevel, logFile string) {
	InitGlobalLoggerWithDebug(logLevel, logFile, false)
}

// InitGlobalLoggerWithDebug initializes the global logger instance with debug mode support
func InitGlobalLoggerWithDebug(logLevel, logFile string, debugToConsole bool) {
	loggerOnce.Do(func() {
		globalLogger = NewLoggerWithDebug(logLevel, logFile, debugToConsole)
	})
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() LoggerInterface {
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

// ConsoleOutput writes logs to console
type ConsoleOutput struct {
	writer io.Writer
	format LogFormat
	mu     sync.Mutex
}

// NewConsoleOutput creates a new console output
func NewConsoleOutput(writer io.Writer, format LogFormat) Output {
	return &ConsoleOutput{
		writer: writer,
		format: format,
	}
}

// Write writes a log entry to console
func (c *ConsoleOutput) Write(entry LogEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var output string
	if c.format == FormatJSON {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		output = string(data)
	} else {
		// Text format
		timestamp := entry.Timestamp.Format("2006/01/02 15:04:05")
		output = fmt.Sprintf("%s [%s] %s", timestamp, entry.Level, entry.Message)

		// Add fields if any
		if len(entry.Fields) > 0 {
			fieldStrs := make([]string, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
			}
			output += " " + strings.Join(fieldStrs, " ")
		}
	}

	_, err := fmt.Fprintln(c.writer, output)
	return err
}

// Close closes the console output
func (c *ConsoleOutput) Close() error {
	return nil
}

// FileOutput writes logs to a file
type FileOutput struct {
	file   *os.File
	format LogFormat
	mu     sync.Mutex
}

// NewFileOutput creates a new file output
func NewFileOutput(path string, format LogFormat) (Output, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileOutput{
		file:   file,
		format: format,
	}, nil
}

// Write writes a log entry to file
func (f *FileOutput) Write(entry LogEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var output string
	if f.format == FormatJSON {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		output = string(data)
	} else {
		// Text format with caller info
		timestamp := entry.Timestamp.Format("2006/01/02 15:04:05")
		output = fmt.Sprintf("%s [%s] %s", timestamp, entry.Level, entry.Message)

		// Add fields if any
		if len(entry.Fields) > 0 {
			fieldStrs := make([]string, 0, len(entry.Fields))
			for k, v := range entry.Fields {
				fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", k, v))
			}
			output += " " + strings.Join(fieldStrs, " ")
		}
	}

	_, err := fmt.Fprintln(f.file, output)
	return err
}

// Close closes the file
func (f *FileOutput) Close() error {
	return f.file.Close()
}
