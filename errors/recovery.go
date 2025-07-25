package errors

import (
	"sync"
	"time"
)

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	strategies      map[ErrorType][]RecoveryStrategy
	fallbackChain   []FallbackHandler
	circuitBreaker  *CircuitBreaker
	retryPolicy     *RetryPolicy
	errorCollector  *ErrorCollector
	config          RecoveryConfig
	mu              sync.RWMutex
}

// FallbackHandler 降级处理器
type FallbackHandler interface {
	Handle(err error) error
}

// NewRecoveryManager 创建恢复管理器
func NewRecoveryManager(config RecoveryConfig) *RecoveryManager {
	return &RecoveryManager{
		strategies:     make(map[ErrorType][]RecoveryStrategy),
		circuitBreaker: NewCircuitBreaker(config.CircuitBreakerConfig),
		retryPolicy:    NewRetryPolicy(config.MaxRetries, config.RetryBackoff),
		errorCollector: NewErrorCollector(),
		config:         config,
	}
}

// RegisterStrategy 注册恢复策略
func (rm *RecoveryManager) RegisterStrategy(errorType ErrorType, strategy RecoveryStrategy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.strategies[errorType] = append(rm.strategies[errorType], strategy)
}

// GetStrategies 获取恢复策略
func (rm *RecoveryManager) GetStrategies(errorType ErrorType) []RecoveryStrategy {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.strategies[errorType]
}

// CanRecoverFromPanic 检查是否可以从 panic 恢复
func (rm *RecoveryManager) CanRecoverFromPanic(panicErr *PanicError) bool {
	// 简单的检查逻辑
	return panicErr.Context != nil && panicErr.Context.Component != "critical"
}

// RecoverFromPanic 从 panic 恢复
func (rm *RecoveryManager) RecoverFromPanic(panicErr *PanicError) error {
	// 实现 panic 恢复逻辑
	return nil
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	state           State
	failures        int
	successes       int
	lastFailureTime time.Time
	config          CircuitBreakerConfig
	mu              sync.RWMutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: config,
	}
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 检查熔断器状态
	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.config.RecoveryTimeout {
			cb.state = StateHalfOpen
		} else {
			return &RecoverableError{
				Type:     ErrorTypeSystem,
				Severity: SeverityHigh,
				Message:  "circuit breaker is open",
			}
		}
	case StateHalfOpen:
		// 半开状态，尝试执行
	case StateClosed:
		// 正常状态
	}

	// 执行操作
	err := fn()
	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()

		if cb.failures >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}
		return err
	}

	// 执行成功
	cb.successes++
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failures = 0
	}

	return nil
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	maxRetries int
	backoff    BackoffStrategy
	baseDelay  time.Duration
}

// NewRetryPolicy 创建重试策略
func NewRetryPolicy(maxRetries int, backoff BackoffStrategy) *RetryPolicy {
	return &RetryPolicy{
		maxRetries: maxRetries,
		backoff:    backoff,
		baseDelay:  1 * time.Second,
	}
}

// Execute 执行带重试的操作
func (rp *RetryPolicy) Execute(fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rp.maxRetries; attempt++ {
		if attempt > 0 {
			delay := rp.calculateDelay(attempt)
			time.Sleep(delay)
		}

		if err := fn(); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return lastErr
}

// calculateDelay 计算延迟时间
func (rp *RetryPolicy) calculateDelay(attempt int) time.Duration {
	switch rp.backoff {
	case BackoffExponential:
		return rp.baseDelay * time.Duration(1<<uint(attempt))
	case BackoffLinear:
		return rp.baseDelay * time.Duration(attempt)
	default:
		return rp.baseDelay
	}
}

// NewExponentialBackoff 创建指数退避策略
func NewExponentialBackoff(maxRetries int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		maxRetries: maxRetries,
		backoff:    BackoffExponential,
		baseDelay:  baseDelay,
	}
}

// ErrorCollector 错误收集器
type ErrorCollector struct {
	errors []error
	mu     sync.Mutex
}

// NewErrorCollector 创建错误收集器
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{}
}

// Collect 收集错误
func (ec *ErrorCollector) Collect(err error) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = append(ec.errors, err)
}

// GetErrors 获取错误列表
func (ec *ErrorCollector) GetErrors() []error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	result := make([]error, len(ec.errors))
	copy(result, ec.errors)
	return result
}

// Clear 清除错误
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = ec.errors[:0]
}