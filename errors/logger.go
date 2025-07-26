package errors

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/logging"
)

// ErrorLogger 错误日志记录器
type ErrorLogger struct {
	config  LogConfig
	writers []LogWriter
	buffer  *LogBuffer
	encoder LogEncoder
	logger  logging.LoggerInterface
	mu      sync.Mutex
}

// LogConfig 日志配置
type LogConfig struct {
	Level           LogLevel
	OutputPath      string
	MaxSize         int64
	MaxAge          time.Duration
	EnableRotation  bool
	Format          LogFormat
	IncludeStack    bool
	SensitiveFields []string
}

// LogLevel 日志级别
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// LogFormat 日志格式
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// ErrorLogEntry 错误日志条目
type ErrorLogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Type      ErrorType              `json:"type"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Operation string                 `json:"operation"`
	TraceID   string                 `json:"trace_id"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Error     string                 `json:"error"`
	Stack     string                 `json:"stack,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Recovery  *RecoveryInfo          `json:"recovery,omitempty"`
}

// RecoveryInfo 恢复信息
type RecoveryInfo struct {
	Attempted  bool          `json:"attempted"`
	Successful bool          `json:"successful"`
	Strategy   string        `json:"strategy"`
	Duration   time.Duration `json:"duration"`
	RetryCount int           `json:"retry_count"`
}

// LogWriter 日志写入器接口
type LogWriter interface {
	Write(entry *ErrorLogEntry) error
	WriteBatch(entries []*ErrorLogEntry) error
}

// LogEncoder 日志编码器接口
type LogEncoder interface {
	Encode(entry *ErrorLogEntry) ([]byte, error)
}

// LogBuffer 日志缓冲区
type LogBuffer struct {
	entries []*ErrorLogEntry
	mu      sync.Mutex
	maxSize int
}

// NewErrorLogger 创建错误日志记录器
func NewErrorLogger(config LogConfig) *ErrorLogger {
	if config.OutputPath == "" {
		config.OutputPath = "clawcat-errors.log"
	}
	if config.Format == "" {
		config.Format = LogFormatJSON
	}

	// Get the global logger or create a new one
	var baseLogger logging.LoggerInterface
	if logging.GetLogger() != nil {
		baseLogger = logging.GetLogger()
	} else {
		// Create a default logger if global logger not initialized
		baseLogger = logging.NewLogger("error", config.OutputPath)
	}

	logger := &ErrorLogger{
		config:  config,
		buffer:  NewLogBuffer(1000),
		encoder: NewJSONEncoder(),
		logger:  baseLogger.With(logging.Field{Key: "component", Value: "error_handler"}),
	}

	// 初始化写入器
	logger.initWriters()

	// 启动日志处理
	go logger.processLogs()

	return logger
}

// LogError 记录错误
func (el *ErrorLogger) LogError(err *RecoverableError, context *ErrorContext) {
	entry := &ErrorLogEntry{
		Timestamp: time.Now(),
		Level:     el.severityToLevel(err.Severity),
		Type:      err.Type,
		Message:   err.Message,
		Component: context.Component,
		Operation: context.ContextName,
		TraceID:   "", // TraceID not available in new ErrorContext
		UserID:    "", // User not available in new ErrorContext
		SessionID: "", // SessionID not available in new ErrorContext
		Error:     err.Cause.Error(),
		Context:   el.sanitizeContext(context.ContextData),
	}

	// 添加堆栈信息
	if el.config.IncludeStack && err.Severity >= SeverityHigh {
		entry.Stack = context.StackTrace
	}

	// 添加恢复信息
	if recovery := context.ContextData["recovery"]; recovery != nil {
		if info, ok := recovery.(*RecoveryInfo); ok {
			entry.Recovery = info
		}
	}

	// 写入缓冲区
	el.buffer.Write(entry)
}

// LogFatal 记录致命错误
func (el *ErrorLogger) LogFatal(panicErr *PanicError) {
	entry := &ErrorLogEntry{
		Timestamp: panicErr.Timestamp,
		Level:     "fatal",
		Type:      ErrorTypeSystem,
		Message:   "application panic",
		Error:     fmt.Sprintf("panic: %v", panicErr.Value),
		Stack:     panicErr.Stack,
	}

	if panicErr.Context != nil {
		entry.Component = panicErr.Context.Component
		entry.Operation = panicErr.Context.ContextName
		entry.TraceID = "" // TraceID not available in new ErrorContext
	}

	// 立即写入（不经过缓冲区）
	for _, writer := range el.writers {
		if err := writer.Write(entry); err != nil {
			// 写入失败 - 无法记录到文件，静默失败以避免控制台输出
			// Last resort error reporting is disabled to prevent console output
		}
	}
}

// processLogs 处理日志
func (el *ErrorLogger) processLogs() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		entries := el.buffer.Flush()
		if len(entries) == 0 {
			continue
		}

		// 批量写入
		el.mu.Lock()
		for _, writer := range el.writers {
			if err := writer.WriteBatch(entries); err != nil {
				// 写入失败，尝试备用方案
				el.handleWriteError(err, entries)
			}
		}
		el.mu.Unlock()
	}
}

// initWriters 初始化写入器
func (el *ErrorLogger) initWriters() {
	// 文件写入器
	if el.config.OutputPath != "" {
		fileWriter := NewFileLogWriter(el.config.OutputPath)
		el.writers = append(el.writers, fileWriter)
	}
}

// handleWriteError 处理写入错误
func (el *ErrorLogger) handleWriteError(err error, entries []*ErrorLogEntry) {
	// 最后的备用方案：使用统一logger记录
	for _, entry := range entries {
		el.logger.Errorf("ERROR LOG WRITE FAILED: %v, entry: %+v", err, entry)
	}
}

// severityToLevel 将严重程度转换为日志级别
func (el *ErrorLogger) severityToLevel(severity ErrorSeverity) string {
	switch severity {
	case SeverityLow:
		return "debug"
	case SeverityMedium:
		return "info"
	case SeverityHigh:
		return "warn"
	case SeverityCritical:
		return "error"
	default:
		return "info"
	}
}

// sanitizeContext 清理敏感信息
func (el *ErrorLogger) sanitizeContext(context map[string]interface{}) map[string]interface{} {
	if context == nil {
		return nil
	}

	sanitized := make(map[string]interface{})
	for k, v := range context {
		// 检查是否为敏感字段
		if el.isSensitiveField(k) {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

// isSensitiveField 检查是否为敏感字段
func (el *ErrorLogger) isSensitiveField(field string) bool {
	for _, sensitive := range el.config.SensitiveFields {
		if field == sensitive {
			return true
		}
	}
	return false
}

// NewLogBuffer 创建日志缓冲区
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]*ErrorLogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Write 写入日志条目
func (lb *LogBuffer) Write(entry *ErrorLogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries = append(lb.entries, entry)

	// 如果超过最大大小，移除最旧的条目
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[1:]
	}
}

// Flush 刷新缓冲区
func (lb *LogBuffer) Flush() []*ErrorLogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.entries) == 0 {
		return nil
	}

	entries := make([]*ErrorLogEntry, len(lb.entries))
	copy(entries, lb.entries)
	lb.entries = lb.entries[:0]

	return entries
}

// JSONEncoder JSON 编码器
type JSONEncoder struct{}

// NewJSONEncoder 创建 JSON 编码器
func NewJSONEncoder() *JSONEncoder {
	return &JSONEncoder{}
}

// Encode 编码日志条目
func (je *JSONEncoder) Encode(entry *ErrorLogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// FileLogWriter 文件日志写入器
type FileLogWriter struct {
	file    *os.File
	path    string
	encoder LogEncoder
	mu      sync.Mutex
}

// NewFileLogWriter 创建文件日志写入器
func NewFileLogWriter(path string) *FileLogWriter {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Use global logger if available, otherwise silently fail
		if logging.GetLogger() != nil {
			logging.GetLogger().Errorf("Failed to open log file %s: %v", path, err)
		}
		return nil
	}

	return &FileLogWriter{
		file:    file,
		path:    path,
		encoder: NewJSONEncoder(),
	}
}

// Write 写入单个日志条目
func (flw *FileLogWriter) Write(entry *ErrorLogEntry) error {
	if flw == nil || flw.file == nil {
		return fmt.Errorf("file writer not initialized")
	}

	flw.mu.Lock()
	defer flw.mu.Unlock()

	data, err := flw.encoder.Encode(entry)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = flw.file.Write(data)
	return err
}

// WriteBatch 批量写入日志条目
func (flw *FileLogWriter) WriteBatch(entries []*ErrorLogEntry) error {
	if flw == nil || flw.file == nil {
		return fmt.Errorf("file writer not initialized")
	}

	flw.mu.Lock()
	defer flw.mu.Unlock()

	for _, entry := range entries {
		data, err := flw.encoder.Encode(entry)
		if err != nil {
			continue
		}

		data = append(data, '\n')
		if _, err := flw.file.Write(data); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return flw.file.Sync()
}

// ConsoleLogWriter 控制台日志写入器
type ConsoleLogWriter struct {
	encoder LogEncoder
}

// NewConsoleLogWriter 创建控制台日志写入器
func NewConsoleLogWriter() *ConsoleLogWriter {
	return &ConsoleLogWriter{
		encoder: NewJSONEncoder(),
	}
}

// Write 写入单个日志条目
func (clw *ConsoleLogWriter) Write(entry *ErrorLogEntry) error {
	_, err := clw.encoder.Encode(entry)
	if err != nil {
		return err
	}

	// Console output disabled to prevent logs to stderr
	// data would be written to os.Stderr but is suppressed
	return nil
}

// WriteBatch 批量写入日志条目
func (clw *ConsoleLogWriter) WriteBatch(entries []*ErrorLogEntry) error {
	for _, entry := range entries {
		if err := clw.Write(entry); err != nil {
			return fmt.Errorf("failed to write console log entry: %w", err)
		}
	}
	return nil
}
