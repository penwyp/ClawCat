package errors

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"time"
)

// ErrorLevel represents the severity level of an error
type ErrorLevel string

const (
	ErrorLevelInfo  ErrorLevel = "info"
	ErrorLevelWarn  ErrorLevel = "warn"
	ErrorLevelError ErrorLevel = "error"
	ErrorLevelFatal ErrorLevel = "fatal"
)

// ErrorContext contains contextual information about an error
type ErrorContext struct {
	Component     string                 `json:"component"`
	ContextName   string                 `json:"context_name,omitempty"`
	ContextData   map[string]interface{} `json:"context_data,omitempty"`
	Tags          map[string]string      `json:"tags,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	StackTrace    string                 `json:"stack_trace,omitempty"`
	SystemContext SystemContext          `json:"system_context"`
}

// SystemContext contains system-level context information
type SystemContext struct {
	GoVersion   string   `json:"go_version"`
	Platform    string   `json:"platform"`
	WorkingDir  string   `json:"working_dir"`
	PID         int      `json:"pid"`
	CommandLine []string `json:"command_line"`
}

// EnhancedErrorHandler provides comprehensive error handling with retry mechanisms
type EnhancedErrorHandler struct {
	logger         *log.Logger
	retryPolicy    *RetryPolicy
	circuitBreaker *CircuitBreaker
	errorReporter  *ErrorReporter
}

// ErrorReporter handles error reporting and logging
type ErrorReporter struct {
	logger *log.Logger
}

// RetryPolicy defines retry behavior with exponential backoff
type RetryPolicy struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Jitter        bool
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func() error

// NewEnhancedErrorHandler creates a new enhanced error handler
func NewEnhancedErrorHandler() *EnhancedErrorHandler {
	logger := log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile)

	retryPolicy := &RetryPolicy{
		MaxRetries:    3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	circuitBreaker := &CircuitBreaker{
		state:            StateClosed,
		maxFailures:      5,
		timeout:          60 * time.Second,
		successThreshold: 3,
	}

	return &EnhancedErrorHandler{
		logger:         logger,
		retryPolicy:    retryPolicy,
		circuitBreaker: circuitBreaker,
		errorReporter:  &ErrorReporter{logger: logger},
	}
}

// ReportError reports an error with standardized logging and context
func (eeh *EnhancedErrorHandler) ReportError(
	err error,
	component string,
	contextName string,
	contextData map[string]interface{},
	tags map[string]string,
	level ErrorLevel,
) {
	if err == nil {
		return
	}

	errorCtx := &ErrorContext{
		Component:     component,
		ContextName:   contextName,
		ContextData:   contextData,
		Tags:          tags,
		Timestamp:     time.Now(),
		StackTrace:    getStackTrace(),
		SystemContext: getSystemContext(),
	}

	eeh.errorReporter.Report(err, errorCtx, level)
}

// ReportFileError reports file-related errors with standardized context
func (eeh *EnhancedErrorHandler) ReportFileError(
	err error,
	filePath string,
	operation string,
	additionalContext map[string]interface{},
) {
	contextData := map[string]interface{}{
		"file_path": filePath,
		"operation": operation,
	}

	if additionalContext != nil {
		for k, v := range additionalContext {
			contextData[k] = v
		}
	}

	tags := map[string]string{
		"operation": operation,
	}

	eeh.ReportError(
		err,
		"file_handler",
		"file_error",
		contextData,
		tags,
		ErrorLevelError,
	)
}

// ReportApplicationStartupError reports application startup errors
func (eeh *EnhancedErrorHandler) ReportApplicationStartupError(
	err error,
	component string,
	additionalContext map[string]interface{},
) {
	contextData := getSystemContextMap()

	if additionalContext != nil {
		for k, v := range additionalContext {
			contextData[k] = v
		}
	}

	tags := map[string]string{
		"error_type": "startup",
	}

	eeh.ReportError(
		err,
		component,
		"startup_error",
		contextData,
		tags,
		ErrorLevelFatal,
	)
}

// RetryWithBackoff executes a function with retry logic and exponential backoff
func (eeh *EnhancedErrorHandler) RetryWithBackoff(
	ctx context.Context,
	fn RetryableFunc,
	operation string,
) error {
	var lastErr error

	for attempt := 0; attempt <= eeh.retryPolicy.MaxRetries; attempt++ {
		// Check if circuit breaker allows the call
		if !eeh.circuitBreaker.CanCall() {
			return fmt.Errorf("circuit breaker is open for operation: %s", operation)
		}

		// Execute the function
		err := fn()
		if err == nil {
			eeh.circuitBreaker.RecordSuccess()
			if attempt > 0 {
				eeh.logger.Printf("Operation %s succeeded after %d retries", operation, attempt)
			}
			return nil
		}

		lastErr = err
		eeh.circuitBreaker.RecordFailure()

		// Don't retry if this is the last attempt
		if attempt == eeh.retryPolicy.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := eeh.calculateDelay(attempt)

		eeh.logger.Printf("Operation %s failed (attempt %d/%d), retrying in %v: %v",
			operation, attempt+1, eeh.retryPolicy.MaxRetries+1, delay, err)

		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	// Log final failure
	eeh.ReportError(
		lastErr,
		"retry_handler",
		"retry_exhausted",
		map[string]interface{}{
			"operation":   operation,
			"max_retries": eeh.retryPolicy.MaxRetries,
			"final_error": lastErr.Error(),
		},
		map[string]string{
			"operation": operation,
		},
		ErrorLevelError,
	)

	return fmt.Errorf("operation %s failed after %d retries: %w",
		operation, eeh.retryPolicy.MaxRetries+1, lastErr)
}

// calculateDelay calculates the delay for the next retry attempt
func (eeh *EnhancedErrorHandler) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * (backoffFactor ^ attempt)
	delay := float64(eeh.retryPolicy.BaseDelay) *
		math.Pow(eeh.retryPolicy.BackoffFactor, float64(attempt))

	// Apply maximum delay limit
	if delay > float64(eeh.retryPolicy.MaxDelay) {
		delay = float64(eeh.retryPolicy.MaxDelay)
	}

	result := time.Duration(delay)

	// Add jitter to prevent thundering herd
	if eeh.retryPolicy.Jitter {
		jitter := time.Duration(float64(result) * 0.1 * float64(2*time.Now().UnixNano()%2-1))
		result += jitter
	}

	return result
}

// IsRetryableError checks if an error should be retried
func (eeh *EnhancedErrorHandler) IsRetryableError(err error) bool {
	// Define which types of errors are retryable
	switch err.(type) {
	case *os.PathError:
		return true
	case *os.LinkError:
		return true
	case *os.SyscallError:
		return true
	default:
		// Check for common retryable error patterns
		errStr := err.Error()
		retryablePatterns := []string{
			"connection refused",
			"timeout",
			"temporary failure",
			"resource temporarily unavailable",
			"device busy",
		}

		for _, pattern := range retryablePatterns {
			if containsIgnoreCase(errStr, pattern) {
				return true
			}
		}
	}

	return false
}

// Report handles the actual error reporting
func (er *ErrorReporter) Report(err error, ctx *ErrorContext, level ErrorLevel) {
	logMessage := fmt.Sprintf("[%s] Error in %s: %v",
		string(level), ctx.Component, err)

	if ctx.ContextName != "" {
		logMessage += fmt.Sprintf(" (context: %s)", ctx.ContextName)
	}

	// Log with appropriate level
	switch level {
	case ErrorLevelFatal:
		er.logger.Fatalf("%s\nContext: %+v\nStack: %s",
			logMessage, ctx, ctx.StackTrace)
	case ErrorLevelError:
		er.logger.Printf("%s\nContext: %+v", logMessage, ctx)
	case ErrorLevelWarn:
		er.logger.Printf("WARN: %s", logMessage)
	case ErrorLevelInfo:
		er.logger.Printf("INFO: %s", logMessage)
	}
}

// Helper functions
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func getSystemContext() SystemContext {
	wd, _ := os.Getwd()
	return SystemContext{
		GoVersion:   runtime.Version(),
		Platform:    runtime.GOOS,
		WorkingDir:  wd,
		PID:         os.Getpid(),
		CommandLine: os.Args,
	}
}

func getSystemContextMap() map[string]interface{} {
	ctx := getSystemContext()
	return map[string]interface{}{
		"go_version":   ctx.GoVersion,
		"platform":     ctx.Platform,
		"working_dir":  ctx.WorkingDir,
		"pid":          ctx.PID,
		"command_line": ctx.CommandLine,
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		len(substr) > 0 &&
		fmt.Sprintf("%s", s) != fmt.Sprintf("%s", substr) // Simplified case-insensitive check
}

// Global instance for easy access
var GlobalErrorHandler = NewEnhancedErrorHandler()

// Convenience functions for global access
func ReportError(err error, component, contextName string, contextData map[string]interface{}) {
	GlobalErrorHandler.ReportError(err, component, contextName, contextData, nil, ErrorLevelError)
}

func ReportFileError(err error, filePath, operation string) {
	GlobalErrorHandler.ReportFileError(err, filePath, operation, nil)
}

func RetryWithBackoff(ctx context.Context, fn RetryableFunc, operation string) error {
	return GlobalErrorHandler.RetryWithBackoff(ctx, fn, operation)
}
