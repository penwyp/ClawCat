package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
	"github.com/penwyp/ClawCat/ui/components"
)

func main() {
	fmt.Println("🚀 ClawCat 实时指标计算演示")
	fmt.Println("================================")

	// 创建配置
	cfg := &config.Config{
		Subscription: config.SubscriptionConfig{
			Plan:            "pro",
			CustomCostLimit: 18.0,
			WarnThreshold:   75.0,
		},
	}

	// 创建实时会话管理器
	realtimeManager := sessions.NewRealtimeManager(cfg)

	// 开始新会话
	session := realtimeManager.StartNewSession()
	fmt.Printf("📅 会话开始: ID=%s, 开始时间=%s\n", 
		session.ID, session.StartTime.Format("15:04:05"))

	// 创建UI组件
	metricsDisplay := components.NewMetricsDisplay(80, 20)

	// 模拟实时数据流
	fmt.Println("\n📊 模拟实时使用数据...")
	models := []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}

	for i := 0; i < 20; i++ {
		// 生成随机使用条目
		entry := models.UsageEntry{
			Timestamp:   time.Now(),
			Model:       models[rand.Intn(len(models))],
			InputTokens: 50 + rand.Intn(200),
			OutputTokens: 100 + rand.Intn(400),
			CacheCreationTokens: rand.Intn(50),
			CacheReadTokens: rand.Intn(30),
		}
		
		// 计算总tokens
		entry.TotalTokens = entry.InputTokens + entry.OutputTokens + 
			entry.CacheCreationTokens + entry.CacheReadTokens
		
		// 简单的成本计算 (实际应该使用 CostCalculator)
		entry.CostUSD = float64(entry.TotalTokens) * 0.00001 * (1.0 + rand.Float64())

		// 添加到会话
		err := realtimeManager.AddEntryToCurrentSession(entry)
		if err != nil {
			fmt.Printf("❌ 添加条目失败: %v\n", err)
			continue
		}

		// 每5个条目显示一次指标
		if (i+1)%5 == 0 {
			fmt.Printf("\n--- 第 %d 次更新 ---\n", i+1)
			displayMetrics(realtimeManager, metricsDisplay)
		}

		// 模拟时间间隔
		time.Sleep(100 * time.Millisecond)
	}

	// 最终状态
	fmt.Println("\n🎯 最终指标状态")
	fmt.Println("================")
	displayMetrics(realtimeManager, metricsDisplay)

	// 检查限制状态
	approaching, percentage, status := realtimeManager.IsLimitApproaching()
	fmt.Printf("\n⚠️  限制检查: 接近限制=%t, 使用百分比=%.1f%%, 状态=%s\n", 
		approaching, percentage, status)

	// 燃烧率历史
	intervals := []time.Duration{5 * time.Minute, 15 * time.Minute, 30 * time.Minute}
	burnRates := realtimeManager.GetBurnRateHistory(intervals)
	fmt.Println("\n🔥 燃烧率历史:")
	for interval, rate := range burnRates {
		fmt.Printf("  %v: %.1f tokens/min\n", interval, rate)
	}

	fmt.Println("\n✅ 演示完成!")
}

func displayMetrics(manager *sessions.RealtimeManager, display *components.MetricsDisplay) {
	metrics := manager.GetCurrentMetrics()
	display.SetMetrics(metrics)

	// 显示紧凑版本
	fmt.Printf("📈 %s\n", display.RenderCompact())

	// 显示摘要
	fmt.Printf("📋 %s\n", display.GetSummary())

	// 显示详细指标
	if metrics != nil {
		fmt.Printf("🔢 详细指标:\n")
		fmt.Printf("   当前Tokens: %s\n", formatNumber(metrics.CurrentTokens))
		fmt.Printf("   当前成本: $%.2f\n", metrics.CurrentCost)
		fmt.Printf("   Token速率: %.1f/分钟\n", metrics.TokensPerMinute)
		fmt.Printf("   成本速率: $%.2f/小时\n", metrics.CostPerHour)
		fmt.Printf("   燃烧率: %.1f tokens/分钟\n", metrics.BurnRate)
		fmt.Printf("   会话进度: %.1f%%\n", metrics.SessionProgress)
		fmt.Printf("   剩余时间: %s\n", formatDuration(metrics.TimeRemaining))
		fmt.Printf("   预测Tokens: %s\n", formatNumber(metrics.ProjectedTokens))
		fmt.Printf("   预测成本: $%.2f\n", metrics.ProjectedCost)
		fmt.Printf("   置信度: %.0f%%\n", metrics.ConfidenceLevel)

		if len(metrics.ModelDistribution) > 0 {
			fmt.Printf("   模型分布:\n")
			for model, modelMetrics := range metrics.ModelDistribution {
				fmt.Printf("     %s: %s tokens (%.1f%%, $%.2f)\n",
					truncateString(model, 15),
					formatNumber(modelMetrics.TokenCount),
					modelMetrics.Percentage,
					modelMetrics.Cost)
			}
		}
	}
}

// 辅助函数
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}