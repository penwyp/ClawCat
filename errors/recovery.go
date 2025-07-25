package errors

import (
	"sync"
)

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	strategies      map[ErrorType][]RecoveryStrategy
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
		retryPolicy:    nil, // RetryPolicy implementation removed
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