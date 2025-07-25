package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
)

// AggregationView 聚合视图组件
type AggregationView struct {
	Title     string
	ViewType  calculations.AggregationView
	Data      []calculations.AggregatedData
	Width     int
	Height    int
	Focused   bool
	ShowTrend bool
	style     AggregationViewStyle
}

// AggregationViewStyle 样式配置
type AggregationViewStyle struct {
	TitleStyle     lipgloss.Style
	BorderStyle    lipgloss.Style
	HeaderStyle    lipgloss.Style
	ContentStyle   lipgloss.Style
	TrendUpStyle   lipgloss.Style
	TrendDownStyle lipgloss.Style
	FocusedBorder  lipgloss.Border
	NormalBorder   lipgloss.Border
}

// NewAggregationView 创建新的聚合视图
func NewAggregationView(title string, viewType calculations.AggregationView) *AggregationView {
	return &AggregationView{
		Title:     title,
		ViewType:  viewType,
		Width:     80,
		Height:    20,
		ShowTrend: true,
		style:     DefaultAggregationViewStyle(),
	}
}

// DefaultAggregationViewStyle 默认样式
func DefaultAggregationViewStyle() AggregationViewStyle {
	return AggregationViewStyle{
		TitleStyle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")),
		BorderStyle:    lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#626262")),
		HeaderStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5A5A5")),
		ContentStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")),
		TrendUpStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")),
		TrendDownStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")),
		FocusedBorder:  lipgloss.RoundedBorder(),
		NormalBorder:   lipgloss.NormalBorder(),
	}
}

// SetData 设置数据
func (av *AggregationView) SetData(data []calculations.AggregatedData) {
	av.Data = data
}

// SetSize 设置尺寸
func (av *AggregationView) SetSize(width, height int) {
	av.Width = width
	av.Height = height
}

// SetFocused 设置焦点状态
func (av *AggregationView) SetFocused(focused bool) {
	av.Focused = focused
}

// Render 渲染视图
func (av *AggregationView) Render() string {
	// 选择边框样式
	borderStyle := av.style.BorderStyle
	if av.Focused {
		borderStyle = borderStyle.Border(av.style.FocusedBorder).BorderForeground(lipgloss.Color("#00BFFF"))
	} else {
		borderStyle = borderStyle.Border(av.style.NormalBorder)
	}

	// 标题
	title := av.style.TitleStyle.Render(av.Title)

	// 内容
	content := av.renderContent()

	// 组合内容
	view := lipgloss.JoinVertical(lipgloss.Left, title, "", content)

	// 应用边框和尺寸
	return borderStyle.
		Width(av.Width - 2).
		Height(av.Height - 2).
		Render(view)
}

// renderContent 渲染内容
func (av *AggregationView) renderContent() string {
	if len(av.Data) == 0 {
		return av.style.ContentStyle.Render("No data available")
	}

	var content []string

	// 根据视图类型渲染不同内容
	switch av.ViewType {
	case calculations.DailyView:
		content = append(content, av.renderDailyView())
	case calculations.WeeklyView:
		content = append(content, av.renderWeeklyView())
	case calculations.MonthlyView:
		content = append(content, av.renderMonthlyView())
	default:
		content = append(content, av.renderGenericView())
	}

	// 添加统计摘要
	if len(av.Data) > 0 {
		content = append(content, "", av.renderSummary())
	}

	return strings.Join(content, "\n")
}

// renderDailyView 渲染日视图
func (av *AggregationView) renderDailyView() string {
	var lines []string

	// 表头
	header := av.style.HeaderStyle.Render(
		fmt.Sprintf("%-12s %8s %10s %8s", "Date", "Tokens", "Cost", "Entries"),
	)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("─", av.Width-4))

	// 限制显示条目数
	maxItems := av.Height - 8
	start := 0
	if len(av.Data) > maxItems {
		start = len(av.Data) - maxItems
	}

	for i := start; i < len(av.Data); i++ {
		data := av.Data[i]

		// 格式化日期
		date := data.Period.Start.Format("01-02")

		// 格式化数据
		tokensStr := formatLargeNumber(data.Tokens.Total)
		costStr := fmt.Sprintf("$%.2f", data.Cost.Total)
		entriesStr := fmt.Sprintf("%d", data.Entries)

		// 趋势指示器
		trend := ""
		if av.ShowTrend && i > 0 {
			prevData := av.Data[i-1]
			if data.Cost.Total > prevData.Cost.Total {
				trend = av.style.TrendUpStyle.Render("↗")
			} else if data.Cost.Total < prevData.Cost.Total {
				trend = av.style.TrendDownStyle.Render("↘")
			} else {
				trend = " "
			}
		}

		line := av.style.ContentStyle.Render(
			fmt.Sprintf("%-12s %8s %10s %8s %s",
				date, tokensStr, costStr, entriesStr, trend),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderWeeklyView 渲染周视图
func (av *AggregationView) renderWeeklyView() string {
	var lines []string

	// 表头
	header := av.style.HeaderStyle.Render(
		fmt.Sprintf("%-15s %8s %10s %8s", "Week", "Tokens", "Cost", "Entries"),
	)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("─", av.Width-4))

	// 限制显示条目数
	maxItems := av.Height - 8
	start := 0
	if len(av.Data) > maxItems {
		start = len(av.Data) - maxItems
	}

	for i := start; i < len(av.Data); i++ {
		data := av.Data[i]

		// 格式化周
		week := data.Period.Label
		if len(week) > 15 {
			week = week[:12] + "..."
		}

		// 格式化数据
		tokensStr := formatLargeNumber(data.Tokens.Total)
		costStr := fmt.Sprintf("$%.2f", data.Cost.Total)
		entriesStr := fmt.Sprintf("%d", data.Entries)

		// 趋势指示器
		trend := ""
		if av.ShowTrend && i > 0 {
			prevData := av.Data[i-1]
			if data.Cost.Total > prevData.Cost.Total {
				trend = av.style.TrendUpStyle.Render("↗")
			} else if data.Cost.Total < prevData.Cost.Total {
				trend = av.style.TrendDownStyle.Render("↘")
			} else {
				trend = " "
			}
		}

		line := av.style.ContentStyle.Render(
			fmt.Sprintf("%-15s %8s %10s %8s %s",
				week, tokensStr, costStr, entriesStr, trend),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderMonthlyView 渲染月视图
func (av *AggregationView) renderMonthlyView() string {
	var lines []string

	// 表头
	header := av.style.HeaderStyle.Render(
		fmt.Sprintf("%-15s %8s %10s %8s", "Month", "Tokens", "Cost", "Entries"),
	)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("─", av.Width-4))

	for i, data := range av.Data {
		// 格式化月份
		month := data.Period.Label
		if len(month) > 15 {
			month = month[:12] + "..."
		}

		// 格式化数据
		tokensStr := formatLargeNumber(data.Tokens.Total)
		costStr := fmt.Sprintf("$%.2f", data.Cost.Total)
		entriesStr := fmt.Sprintf("%d", data.Entries)

		// 趋势指示器
		trend := ""
		if av.ShowTrend && i > 0 {
			prevData := av.Data[i-1]
			if data.Cost.Total > prevData.Cost.Total {
				trend = av.style.TrendUpStyle.Render("↗")
			} else if data.Cost.Total < prevData.Cost.Total {
				trend = av.style.TrendDownStyle.Render("↘")
			} else {
				trend = " "
			}
		}

		line := av.style.ContentStyle.Render(
			fmt.Sprintf("%-15s %8s %10s %8s %s",
				month, tokensStr, costStr, entriesStr, trend),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderGenericView 渲染通用视图
func (av *AggregationView) renderGenericView() string {
	var lines []string

	// 表头
	header := av.style.HeaderStyle.Render(
		fmt.Sprintf("%-20s %8s %10s", "Period", "Tokens", "Cost"),
	)
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("─", av.Width-4))

	for _, data := range av.Data {
		period := data.Period.Label
		if len(period) > 20 {
			period = period[:17] + "..."
		}

		tokensStr := formatLargeNumber(data.Tokens.Total)
		costStr := fmt.Sprintf("$%.2f", data.Cost.Total)

		line := av.style.ContentStyle.Render(
			fmt.Sprintf("%-20s %8s %10s", period, tokensStr, costStr),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderSummary 渲染统计摘要
func (av *AggregationView) renderSummary() string {
	if len(av.Data) == 0 {
		return ""
	}

	// 计算总计
	totalTokens := 0
	totalCost := 0.0
	totalEntries := 0

	for _, data := range av.Data {
		totalTokens += data.Tokens.Total
		totalCost += data.Cost.Total
		totalEntries += data.Entries
	}

	// 计算平均值
	avgTokens := float64(totalTokens) / float64(len(av.Data))
	avgCost := totalCost / float64(len(av.Data))

	summary := []string{
		strings.Repeat("─", av.Width-4),
		av.style.HeaderStyle.Render("Summary:"),
		av.style.ContentStyle.Render(fmt.Sprintf("Total: %s tokens, $%.2f",
			formatLargeNumber(totalTokens), totalCost)),
		av.style.ContentStyle.Render(fmt.Sprintf("Average: %s tokens, $%.2f per period",
			formatLargeNumber(int(avgTokens)), avgCost)),
		av.style.ContentStyle.Render(fmt.Sprintf("Periods: %d, Total entries: %d",
			len(av.Data), totalEntries)),
	}

	return strings.Join(summary, "\n")
}

// formatLargeNumber 格式化大数字
func formatLargeNumber(num int) string {
	if num >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(num)/1000000)
	} else if num >= 1000 {
		return fmt.Sprintf("%.1fK", float64(num)/1000)
	}
	return fmt.Sprintf("%d", num)
}

// GetSelectedPeriod 获取选中的时间段（用于交互）
func (av *AggregationView) GetSelectedPeriod(index int) *calculations.TimePeriod {
	if index >= 0 && index < len(av.Data) {
		return &av.Data[index].Period
	}
	return nil
}

// GetModelDistribution 获取模型分布信息
func (av *AggregationView) GetModelDistribution(index int) map[string]calculations.AggregationModelStats {
	if index >= 0 && index < len(av.Data) {
		return av.Data[index].Models
	}
	return nil
}

// RenderModelDistribution 渲染模型分布
func (av *AggregationView) RenderModelDistribution(index int) string {
	models := av.GetModelDistribution(index)
	if len(models) == 0 {
		return "No model distribution data"
	}

	var lines []string
	lines = append(lines, av.style.HeaderStyle.Render("Model Distribution:"))
	lines = append(lines, strings.Repeat("─", 30))

	// 按使用量排序
	type modelUsage struct {
		name   string
		tokens int
		cost   float64
	}

	var modelList []modelUsage
	for name, stats := range models {
		modelList = append(modelList, modelUsage{
			name:   name,
			tokens: stats.Tokens,
			cost:   stats.Cost,
		})
	}

	// 排序（按tokens降序）
	for i := 0; i < len(modelList)-1; i++ {
		for j := i + 1; j < len(modelList); j++ {
			if modelList[i].tokens < modelList[j].tokens {
				modelList[i], modelList[j] = modelList[j], modelList[i]
			}
		}
	}

	for _, model := range modelList {
		tokensStr := formatLargeNumber(model.tokens)
		costStr := fmt.Sprintf("$%.2f", model.cost)

		line := av.style.ContentStyle.Render(
			fmt.Sprintf("%-20s %8s %8s", model.name, tokensStr, costStr),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
