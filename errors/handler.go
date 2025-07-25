package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// ErrorHandler 全局错误处理器
type ErrorHandler struct {
	recoveryManager *RecoveryManager
	errorLogger     *ErrorLogger
	alertManager    *AlertManager
	metrics         *ErrorMetrics

	// 错误缓冲
	errorBuffer *RingBuffer
	bufferMu    sync.RWMutex

	// 状态跟踪
	healthStatus HealthStatus
	degradedMode bool
	mu           sync.RWMutex
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(config RecoveryConfig) *ErrorHandler {
	handler := &ErrorHandler{
		recoveryManager: NewRecoveryManager(config),
		errorLogger:     NewErrorLogger(LogConfig{}),
		alertManager:    NewAlertManager(),
		metrics:         NewErrorMetrics(),
		errorBuffer:     NewRingBuffer(1000),
		healthStatus:    HealthStatusHealthy,
	}

	// 注册恢复策略
	handler.registerDefaultStrategies()

	// 启动监控
	go handler.monitorHealth()

	return handler
}

// Handle 处理错误
func (eh *ErrorHandler) Handle(err error, context *ErrorContext) error {
	// 分类错误
	classified := eh.classifyError(err)

	// 记录错误
	eh.logError(classified, context)

	// 更新指标
	eh.updateMetrics(classified)

	// 尝试恢复
	if classified.CanRetry {
		if recovered := eh.tryRecover(classified, context); recovered == nil {
			return nil
		}
	}

	// 检查是否需要降级
	if eh.shouldDegrade(classified) {
		eh.enterDegradedMode()
	}

	// 发送告警
	if classified.Severity >= SeverityHigh {
		eh.alertManager.SendAlert(classified, context)
	}

	return classified
}

// HandlePanic 处理 panic
func (eh *ErrorHandler) HandlePanic(context *ErrorContext) {
	if r := recover(); r != nil {
		// 获取堆栈信息
		stack := debug.Stack()

		// 创建 panic 错误
		panicErr := &PanicError{
			Value:     r,
			Stack:     string(stack),
			Timestamp: time.Now(),
			Context:   context,
		}

		// 记录严重错误
		eh.Handle(panicErr, context)

		// 尝试恢复
		if eh.recoveryManager.CanRecoverFromPanic(panicErr) {
			eh.recoveryManager.RecoverFromPanic(panicErr)
		} else {
			// 无法恢复，优雅关闭
			eh.gracefulShutdown(panicErr)
		}
	}
}

// classifyError 分类错误
func (eh *ErrorHandler) classifyError(err error) *RecoverableError {
	// 检查是否已经是分类错误
	if re, ok := err.(*RecoverableError); ok {
		return re
	}

	// 根据错误类型分类
	classified := &RecoverableError{
		Cause:     err,
		Timestamp: time.Now(),
	}

	switch {
	case isPermissionError(err):
		classified.Type = ErrorTypePermission
		classified.Severity = SeverityHigh
		classified.RecoveryHint = "Check file permissions and user privileges"
		classified.CanRetry = false

	case isJSONError(err):
		classified.Type = ErrorTypeDataFormat
		classified.Severity = SeverityMedium
		classified.RecoveryHint = "Skip corrupted entry and continue"
		classified.CanRetry = false

	case isNetworkError(err):
		classified.Type = ErrorTypeNetwork
		classified.Severity = SeverityMedium
		classified.RecoveryHint = "Retry with exponential backoff"
		classified.CanRetry = true
		classified.RetryAfter = 5 * time.Second

	case isResourceError(err):
		classified.Type = ErrorTypeResource
		classified.Severity = SeverityHigh
		classified.RecoveryHint = "Free up system resources"
		classified.CanRetry = true
		classified.RetryAfter = 30 * time.Second

	default:
		classified.Type = ErrorTypeSystem
		classified.Severity = SeverityMedium
		classified.CanRetry = true
	}

	classified.Message = eh.generateErrorMessage(classified)

	return classified
}

// tryRecover 尝试恢复
func (eh *ErrorHandler) tryRecover(err *RecoverableError, context *ErrorContext) error {
	// 获取恢复策略
	strategies := eh.recoveryManager.GetStrategies(err.Type)

	for _, strategy := range strategies {
		if strategy.CanHandle(err) {
			// 使用熔断器保护
			result := eh.recoveryManager.circuitBreaker.Execute(func() error {
				return strategy.Recover(err, context)
			})

			if result == nil {
				// 恢复成功
				eh.metrics.RecoverySuccess.Inc()
				return nil
			}
		}
	}

	// 所有策略都失败
	eh.metrics.RecoveryFailed.Inc()
	return err
}

// registerDefaultStrategies 注册默认恢复策略
func (eh *ErrorHandler) registerDefaultStrategies() {
	// 文件访问错误恢复
	eh.recoveryManager.RegisterStrategy(ErrorTypePermission, &FileAccessRecovery{})

	// JSON 解析错误恢复
	eh.recoveryManager.RegisterStrategy(ErrorTypeDataFormat, &JSONParseRecovery{})

	// 网络错误恢复
	eh.recoveryManager.RegisterStrategy(ErrorTypeNetwork, &NetworkErrorRecovery{
		retryPolicy: NewExponentialBackoff(3, 1*time.Second),
	})

	// 资源错误恢复
	eh.recoveryManager.RegisterStrategy(ErrorTypeResource, &ResourceErrorRecovery{})
}

// shouldDegrade 检查是否应该降级
func (eh *ErrorHandler) shouldDegrade(err *RecoverableError) bool {
	if err.Severity >= SeverityCritical {
		return true
	}

	// 检查错误频率
	recentErrors := eh.errorBuffer.CountRecent(5 * time.Minute)
	if recentErrors > 10 {
		return true
	}

	return false
}

// enterDegradedMode 进入降级模式
func (eh *ErrorHandler) enterDegradedMode() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if !eh.degradedMode {
		eh.degradedMode = true
		eh.healthStatus = HealthStatusDegraded
		// 通知其他组件进入降级模式
		eh.notifyDegradedMode()
	}
}

// monitorHealth 监控健康状态
func (eh *ErrorHandler) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		eh.checkHealth()
	}
}

// checkHealth 检查健康状态
func (eh *ErrorHandler) checkHealth() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	// 检查最近错误数量
	recentErrors := eh.errorBuffer.CountRecent(1 * time.Minute)
	
	if recentErrors == 0 && eh.degradedMode {
		// 恢复正常模式
		eh.degradedMode = false
		eh.healthStatus = HealthStatusHealthy
		eh.notifyRecovery()
	}
}

// gracefulShutdown 优雅关闭
func (eh *ErrorHandler) gracefulShutdown(panicErr *PanicError) {
	// 记录致命错误
	eh.errorLogger.LogFatal(panicErr)

	// 尝试保存状态
	eh.saveState()

	// 退出程序
	os.Exit(1)
}

// 辅助函数
func isPermissionError(err error) bool {
	return os.IsPermission(err)
}

func isJSONError(err error) bool {
	_, ok := err.(*json.SyntaxError)
	if ok {
		return true
	}
	_, ok = err.(*json.UnmarshalTypeError)
	return ok
}

func isNetworkError(err error) bool {
	_, ok := err.(*net.OpError)
	return ok
}

func isResourceError(err error) bool {
	// 检查资源相关错误
	return false // 简化实现
}

func (eh *ErrorHandler) generateErrorMessage(err *RecoverableError) string {
	return fmt.Sprintf("%s error: %s", err.Type, err.Cause.Error())
}

func (eh *ErrorHandler) logError(err *RecoverableError, context *ErrorContext) {
	// 记录到错误缓冲区
	eh.errorBuffer.Add(err)

	// 记录到日志
	eh.errorLogger.LogError(err, context)
}

func (eh *ErrorHandler) updateMetrics(err *RecoverableError) {
	eh.metrics.TotalErrors.Inc()
	eh.metrics.ErrorsByType[err.Type].Inc()
	eh.metrics.ErrorsBySeverity[err.Severity].Inc()
}

func (eh *ErrorHandler) notifyDegradedMode() {
	// 通知其他组件
}

func (eh *ErrorHandler) notifyRecovery() {
	// 通知恢复
}

func (eh *ErrorHandler) saveState() {
	// 保存应用状态
}