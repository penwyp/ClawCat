package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/penwyp/ClawCat/errors"
	"github.com/penwyp/ClawCat/sessions"
)

// ExampleUsage 演示如何使用 UI 降级系统
func ExampleUsage() {
	// 1. 创建错误处理器
	errorConfig := errors.RecoveryConfig{
		MaxRetries:       3,
		RetryBackoff:     errors.BackoffExponential,
		EnableAutoRecovery: true,
		RecoveryTimeout:  30 * time.Second,
		ErrorThreshold:   5,
		CircuitBreakerConfig: errors.CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          10 * time.Second,
			SuccessThreshold: 3,
		},
	}
	errorHandler := errors.NewErrorHandler(errorConfig)

	// 2. 创建 UI 配置
	uiConfig := Config{
		RefreshRate:   time.Second,
		Theme:         "dark",
		ShowSpinner:   true,
		CompactMode:   false,
		ChartHeight:   10,
		TablePageSize: 20,
	}

	degradedConfig := DegradedConfig{
		AutoDetectMode:  true,
		FallbackTimeout: 5 * time.Second,
		MaxRetries:      3,
		SafeMode:        false,
	}

	// 3. 创建带降级功能的应用
	app := NewAppWithFallback(uiConfig, degradedConfig, errorHandler)

	// 4. 设置 UI 降级回调
	errorHandler.SetUICallbacks(
		// 降级回调
		func(err error) {
			log.Printf("UI degraded due to error: %v", err)
			// 这里可以通知用户或发送告警
		},
		// 恢复回调
		func() {
			log.Printf("UI recovered from degraded mode")
			// 这里可以通知用户系统已恢复
		},
	)

	// 5. 模拟使用场景
	go simulateUIErrors(errorHandler)

	// 6. 启动应用
	if err := app.Start(); err != nil {
		log.Printf("App failed to start: %v", err)
	}

	// 7. 清理
	defer func() {
		if err := app.Stop(); err != nil {
			log.Printf("Error stopping app: %v", err)
		}
	}()
}

// simulateUIErrors 模拟 UI 错误场景
func simulateUIErrors(errorHandler *errors.ErrorHandler) {
	time.Sleep(5 * time.Second)

	// 模拟 UI 渲染错误
	uiError := &errors.RecoverableError{
		Type:     errors.ErrorTypeUI,
		Severity: errors.SeverityHigh,
		Message:  "Dashboard rendering failed",
		Context: map[string]interface{}{
			"component": "dashboard",
			"view":      "main",
		},
		Timestamp: time.Now(),
		CanRetry:  true,
	}

	context := &errors.ErrorContext{
		Component:   "UI",
		ContextName: "render_dashboard",
		ContextData: map[string]interface{}{
			"trace_id": "ui-test-001",
		},
	}

	// 处理错误，这将触发 UI 降级
	if err := errorHandler.Handle(uiError, context); err != nil {
		log.Printf("Failed to handle UI error: %v", err)
	}

	// 等待一段时间后模拟恢复
	time.Sleep(10 * time.Second)

	// 系统可能会自动恢复，或者可以手动触发恢复
	log.Println("Simulated error recovery period completed")
}

// DemoDegradedRenderer 演示降级渲染器的功能
func DemoDegradedRenderer() {
	renderer := NewDegradedRenderer()
	renderer.Resize(80, 24)

	// 模拟统计数据
	stats := Statistics{
		ActiveSessions:  3,
		SessionCount:    15,
		TotalTokens:     125000,
		TotalCost:       42.50,
		AverageCost:     2.83,
		TopModel:        "claude-3-sonnet",
		CurrentBurnRate: 1250.0,
		TimeToReset:     25 * 24 * time.Hour,
		PlanUsage:       42.5,
	}

	// 模拟会话数据
	sessions := []*sessions.Session{
		{
			ID:        "session_1",
			IsActive:  true,
			StartTime: time.Now().Add(-2 * time.Hour),
			EndTime:   time.Now().Add(3 * time.Hour),
		},
		{
			ID:        "session_2",
			IsActive:  false,
			StartTime: time.Now().Add(-4 * time.Hour),
			EndTime:   time.Now().Add(1 * time.Hour),
		},
		{
			ID:        "session_3",
			IsActive:  true,
			StartTime: time.Now().Add(-30 * time.Minute),
			EndTime:   time.Now().Add(4*time.Hour + 30*time.Minute),
		},
	}

	// 演示不同降级模式
	modes := []DegradedMode{
		DegradedModeMinimal,
		DegradedModeText,
		DegradedModeBasic,
		DegradedModeSafe,
	}

	modeNames := []string{
		"Minimal Mode",
		"Text Mode",
		"Basic Mode",
		"Safe Mode",
	}

	for i, mode := range modes {
		fmt.Printf("\n=== %s ===\n", modeNames[i])
		output := renderer.RenderDashboard(stats, mode)
		fmt.Println(output)
		fmt.Println()

		// 演示会话列表渲染
		if mode != DegradedModeMinimal {
			fmt.Printf("--- Session List ---\n")
			sessionOutput := renderer.RenderSessionList(sessions, mode)
			fmt.Println(sessionOutput)
			fmt.Println()
		}
	}

	// 演示错误渲染
	fmt.Println("=== Error Rendering ===")
	testError := fmt.Errorf("this is a test error with a longer message that demonstrates how errors are displayed in different degraded modes")
	renderer.SetError(testError)

	for i, mode := range modes {
		fmt.Printf("\n--- %s Error Display ---\n", modeNames[i])
		errorOutput := renderer.RenderErrorMessage(testError, mode)
		fmt.Println(errorOutput)
	}
}

// BenchmarkDegradedRenderer 性能测试
func BenchmarkDegradedRenderer() {
	renderer := NewDegradedRenderer()

	stats := Statistics{
		ActiveSessions:  5,
		SessionCount:    50,
		TotalTokens:     500000,
		TotalCost:       150.0,
		AverageCost:     3.0,
		TopModel:        "claude-3-haiku",
		CurrentBurnRate: 2500.0,
		TimeToReset:     20 * 24 * time.Hour,
		PlanUsage:       75.0,
	}

	// 测试渲染性能
	start := time.Now()
	iterations := 1000

	for i := 0; i < iterations; i++ {
		// 测试最轻量的模式
		renderer.RenderDashboard(stats, DegradedModeMinimal)
	}

	minimalTime := time.Since(start)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		// 测试完整的降级模式
		renderer.RenderDashboard(stats, DegradedModeBasic)
	}

	basicTime := time.Since(start)

	fmt.Printf("Performance benchmark (%d iterations):\n", iterations)
	fmt.Printf("Minimal mode: %v (%.2f μs/op)\n", minimalTime, float64(minimalTime.Nanoseconds())/float64(iterations)/1000.0)
	fmt.Printf("Basic mode:   %v (%.2f μs/op)\n", basicTime, float64(basicTime.Nanoseconds())/float64(iterations)/1000.0)
	fmt.Printf("Overhead:     %.1fx\n", float64(basicTime)/float64(minimalTime))
}