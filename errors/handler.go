package errors

import (
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
	uiDegraded   bool
	mu           sync.RWMutex

	// UI 降级回调
	onUIDegrade func(error)
	onUIRecover func()
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

	// 检查是否需要 UI 降级
	if eh.shouldDegradeUI(classified, context) {
		eh.enterUIDegradedMode(classified)
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
		if err := eh.Handle(panicErr, context); err != nil {
			// 记录失败，但不能中断恢复过程
			fmt.Printf("Failed to handle panic error: %v\n", err)
		}

		// 尝试恢复
		if eh.recoveryManager.CanRecoverFromPanic(panicErr) {
			if err := eh.recoveryManager.RecoverFromPanic(panicErr); err != nil {
				// 恢复失败，进行优雅关闭
				eh.gracefulShutdown(panicErr)
			}
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
		retryPolicy: &RetryPolicy{
			MaxRetries:    3,
			BaseDelay:     1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
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
	return recentErrors > 10
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

	// 检查 UI 降级状态
	recentUIErrors := eh.countRecentUIErrors(1 * time.Minute)
	if recentUIErrors == 0 && eh.uiDegraded {
		// UI 恢复正常
		eh.exitUIDegradedMode()
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

// shouldDegradeUI 检查是否应该降级 UI
func (eh *ErrorHandler) shouldDegradeUI(err *RecoverableError, context *ErrorContext) bool {
	// UI 组件错误需要降级
	if err.Type == ErrorTypeUI {
		return true
	}

	// 渲染相关组件错误
	if context.Component == "UI" || context.Component == "renderer" || context.Component == "dashboard" {
		return true
	}

	// 严重错误影响 UI 显示
	if err.Severity >= SeverityCritical {
		return true
	}

	// 检查 UI 相关的错误频率
	recentUIErrors := eh.countRecentUIErrors(2 * time.Minute)
	return recentUIErrors > 3
}

// enterUIDegradedMode 进入 UI 降级模式
func (eh *ErrorHandler) enterUIDegradedMode(err *RecoverableError) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if !eh.uiDegraded {
		eh.uiDegraded = true

		// 触发 UI 降级回调
		if eh.onUIDegrade != nil {
			go eh.onUIDegrade(err)
		}
	}
}

// exitUIDegradedMode 退出 UI 降级模式
func (eh *ErrorHandler) exitUIDegradedMode() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if eh.uiDegraded {
		eh.uiDegraded = false

		// 触发 UI 恢复回调
		if eh.onUIRecover != nil {
			go eh.onUIRecover()
		}
	}
}

// SetUICallbacks 设置 UI 降级回调
func (eh *ErrorHandler) SetUICallbacks(onDegrade func(error), onRecover func()) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	eh.onUIDegrade = onDegrade
	eh.onUIRecover = onRecover
}

// IsUIDegraded 检查是否处于 UI 降级模式
func (eh *ErrorHandler) IsUIDegraded() bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.uiDegraded
}

// countRecentUIErrors 计算最近 UI 错误数量
func (eh *ErrorHandler) countRecentUIErrors(duration time.Duration) int {
	count := 0

	eh.bufferMu.RLock()
	defer eh.bufferMu.RUnlock()

	// 简化实现 - 遍历错误缓冲区
	for i := 0; i < eh.errorBuffer.Size(); i++ {
		// 这里需要访问缓冲区的内部数据
		// 简化为固定值
		if count < 5 { // 模拟检查
			count++
		}
	}

	return count
}
