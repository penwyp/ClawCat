package errors

import (
	"time"
)

// ErrorType 错误类型
type ErrorType string

const (
	// 系统级错误
	ErrorTypeSystem      ErrorType = "system"
	ErrorTypePermission  ErrorType = "permission"
	ErrorTypeResource    ErrorType = "resource"

	// 数据错误
	ErrorTypeDataFormat  ErrorType = "data_format"
	ErrorTypeDataCorrupt ErrorType = "data_corrupt"
	ErrorTypeDataMissing ErrorType = "data_missing"

	// 网络错误
	ErrorTypeNetwork     ErrorType = "network"
	ErrorTypeTimeout     ErrorType = "timeout"

	// 应用错误
	ErrorTypeConfig      ErrorType = "config"
	ErrorTypeLogic       ErrorType = "logic"
	ErrorTypeUI          ErrorType = "ui"
)

// ErrorSeverity 错误严重程度
type ErrorSeverity int

const (
	SeverityLow      ErrorSeverity = iota // 可忽略
	SeverityMedium                        // 功能降级
	SeverityHigh                          // 需要干预
	SeverityCritical                      // 系统停止
)

// RecoverableError 可恢复错误
type RecoverableError struct {
	Type        ErrorType
	Severity    ErrorSeverity
	Message     string
	Cause       error
	Context     map[string]interface{}
	Timestamp   time.Time
	RecoveryHint string
	CanRetry    bool
	RetryAfter  time.Duration
}

func (e *RecoverableError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Message
}

func (e *RecoverableError) Unwrap() error {
	return e.Cause
}


// RecoveryStrategy 恢复策略接口
type RecoveryStrategy interface {
	CanHandle(err error) bool
	Recover(err error, context *ErrorContext) error
	GetFallback() interface{}
}

// PanicError panic 错误
type PanicError struct {
	Value     interface{}
	Stack     string
	Timestamp time.Time
	Context   *ErrorContext
}

func (e *PanicError) Error() string {
	return "panic occurred"
}

// Priority 优先级
type Priority int

const (
	PriorityLow    Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// HealthStatus 健康状态
type HealthStatus int

const (
	HealthStatusHealthy   HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// RecoveryConfig 恢复配置
type RecoveryConfig struct {
	MaxRetries           int
	RetryBackoff         BackoffStrategy
	CircuitBreakerConfig CircuitBreakerConfig
	EnableAutoRecovery   bool
	RecoveryTimeout      time.Duration
	ErrorThreshold       int
}

// BackoffStrategy 退避策略
type BackoffStrategy string

const (
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffExponential BackoffStrategy = "exponential"
	BackoffLinear      BackoffStrategy = "linear"
)


// OverallHealth 总体健康状态
type OverallHealth struct {
	Status    HealthStatus
	Timestamp time.Time
	Checks    map[string]HealthResult
}

// HealthResult 健康检查结果
type HealthResult struct {
	Status    HealthStatus
	Message   string
	Details   map[string]interface{}
	Timestamp time.Time
}

// HealthCheck 健康检查接口
type HealthCheck interface {
	Name() string
	Check() HealthResult
}