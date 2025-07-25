package main

import (
	"fmt"
	"time"

	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/ui/components"
)

// demonstrateProgressBars 展示进度条组件的功能
func demonstrateProgressBars() {
	fmt.Println("🐱 ClawCat Progress Bar Demo")
	fmt.Println("=" * 50)

	// 1. 基础进度条演示
	fmt.Println("\n1. Basic Progress Bar:")
	
	tokenBar := components.NewProgressBar("Token Usage", 7500, 10000)
	tokenBar.SetWidth(40)
	
	// 设置动态颜色
	colorScheme := components.DefaultColorScheme
	tokenBar.SetColor(colorScheme.GetProgressColor(tokenBar.Percentage))
	
	fmt.Println(tokenBar.Render())
	fmt.Printf("Status: %s, Over limit: %v\n", tokenBar.GetStatus(), tokenBar.IsOverLimit())

	// 2. 进度条区域演示
	fmt.Println("\n2. Progress Section:")
	
	// 创建模拟的实时指标
	metrics := &calculations.RealtimeMetrics{
		SessionStart:     time.Now().Add(-2 * time.Hour),
		CurrentTokens:    85000,
		CurrentCost:      15.50,
		SessionProgress:  40.0,
		TokensPerMinute:  125.0,
		CostPerMinute:    0.12,
		BurnRate:         125.0,
		ProjectedTokens:  210000,
		ProjectedCost:    35.50,
		ConfidenceLevel:  85.0,
		ModelDistribution: map[string]calculations.ModelMetrics{
			"claude-3-opus": {
				TokenCount: 60000,
				Cost:       12.00,
				Percentage: 70.6,
				LastUsed:   time.Now().Add(-5 * time.Minute),
			},
			"claude-3-sonnet": {
				TokenCount: 25000,
				Cost:       3.50,
				Percentage: 29.4,
				LastUsed:   time.Now().Add(-1 * time.Minute),
			},
		},
	}

	limits := components.Limits{
		TokenLimit: 100000,
		CostLimit:  18.00,
	}

	section := components.NewProgressSection(100)
	section.Update(metrics, limits)

	fmt.Println(section.Render())
	fmt.Printf("Summary: %s\n", section.GetSummary())
	fmt.Printf("Critical status: %v\n", section.HasCriticalStatus())
	fmt.Printf("Worst status: %s\n", section.GetWorstStatus())

	// 3. 色彩演示
	fmt.Println("\n3. Color Scheme Demo:")
	
	testPercentages := []float64{25, 55, 80, 95}
	for _, pct := range testPercentages {
		bar := components.NewProgressBar(fmt.Sprintf("Test %.0f%%", pct), pct, 100)
		bar.SetWidth(30)
		bar.SetColor(colorScheme.GetProgressColor(pct))
		fmt.Println(bar.Render())
	}

	fmt.Println("\n✅ Demo completed!")
}

func main() {
	demonstrateProgressBars()
}