package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
)

// MetricsDisplay 实时指标显示组件
type MetricsDisplay struct {
	metrics *calculations.RealtimeMetrics
	width   int
	height  int
}

// Styles contains styling for metrics display
type Styles struct {
	// Cards
	MetricCard      lipgloss.Style
	MetricCardTitle lipgloss.Style
	MetricValue     lipgloss.Style
	MetricLabel     lipgloss.Style

	// Status colors
	Normal    lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Success   lipgloss.Style
	Muted     lipgloss.Style

	// Progress bars
	ProgressBar       lipgloss.Style
	ProgressBarFilled lipgloss.Style
	ProgressBarEmpty  lipgloss.Style

	// Model distribution
	ModelItem    lipgloss.Style
	ModelName    lipgloss.Style
	ModelValue   lipgloss.Style
	ModelPercent lipgloss.Style
}

// NewMetricsDisplay 创建新的指标显示组件
func NewMetricsDisplay(width, height int) *MetricsDisplay {
	return &MetricsDisplay{
		width:  width,
		height: height,
	}
}

// SetMetrics 设置要显示的指标数据
func (md *MetricsDisplay) SetMetrics(metrics *calculations.RealtimeMetrics) {
	md.metrics = metrics
}

// SetSize 设置组件尺寸
func (md *MetricsDisplay) SetSize(width, height int) {
	md.width = width
	md.height = height
}

// Render 渲染指标显示
func (md *MetricsDisplay) Render() string {
	if md.metrics == nil {
		return md.renderNoData()
	}

	styles := md.getStyles()

	// 计算卡片布局
	cardWidth := (md.width - 6) / 2 // 2列布局，考虑边距
	if cardWidth < 20 {
		cardWidth = 20
	}

	// 渲染各个指标卡片
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		md.renderTokenCard(styles, cardWidth),
		" ",
		md.renderCostCard(styles, cardWidth),
	)

	middleRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		md.renderBurnRateCard(styles, cardWidth),
		" ",
		md.renderProjectionCard(styles, cardWidth),
	)

	// 如果有足够空间，显示模型分布
	result := lipgloss.JoinVertical(lipgloss.Left, topRow, "", middleRow)

	if md.height > 15 {
		modelDistribution := md.renderModelDistribution(styles)
		if modelDistribution != "" {
			result = lipgloss.JoinVertical(lipgloss.Left, result, "", modelDistribution)
		}
	}

	return result
}

// renderTokenCard 渲染 Token 使用卡片
func (md *MetricsDisplay) renderTokenCard(styles Styles, width int) string {
	current := formatNumber(md.metrics.CurrentTokens)
	projected := formatNumber(md.metrics.ProjectedTokens)
	rate := fmt.Sprintf("%.1f/min", md.metrics.TokensPerMinute)

	// 计算增长趋势
	trend := ""
	if md.metrics.ProjectedTokens > md.metrics.CurrentTokens {
		increase := md.metrics.ProjectedTokens - md.metrics.CurrentTokens
		trend = fmt.Sprintf(" (+%s)", formatNumber(increase))
	}

	title := styles.MetricCardTitle.Render("📊 Tokens")
	currentLine := fmt.Sprintf("Current: %s", styles.MetricValue.Render(current))
	projectedLine := fmt.Sprintf("Projected: %s%s", styles.MetricValue.Render(projected), styles.Muted.Render(trend))
	rateLine := fmt.Sprintf("Rate: %s", styles.MetricLabel.Render(rate))

	content := lipgloss.JoinVertical(lipgloss.Left, title, currentLine, projectedLine, rateLine)
	return styles.MetricCard.Width(width).Render(content)
}

// renderCostCard 渲染成本卡片
func (md *MetricsDisplay) renderCostCard(styles Styles, width int) string {
	current := fmt.Sprintf("$%.2f", md.metrics.CurrentCost)
	projected := fmt.Sprintf("$%.2f", md.metrics.ProjectedCost)
	rate := fmt.Sprintf("$%.2f/hr", md.metrics.CostPerHour)

	// 计算成本增长
	trend := ""
	if md.metrics.ProjectedCost > md.metrics.CurrentCost {
		increase := md.metrics.ProjectedCost - md.metrics.CurrentCost
		trend = fmt.Sprintf(" (+$%.2f)", increase)
	}

	// 根据成本水平选择颜色
	style := styles.Normal
	if md.metrics.CurrentCost > 10 {
		style = styles.Warning
	}
	if md.metrics.CurrentCost > 15 {
		style = styles.Error
	}

	title := styles.MetricCardTitle.Render("💰 Cost")
	currentLine := fmt.Sprintf("Current: %s", style.Render(current))
	projectedLine := fmt.Sprintf("Projected: %s%s", styles.MetricValue.Render(projected), styles.Muted.Render(trend))
	rateLine := fmt.Sprintf("Rate: %s", styles.MetricLabel.Render(rate))

	content := lipgloss.JoinVertical(lipgloss.Left, title, currentLine, projectedLine, rateLine)
	return styles.MetricCard.Width(width).Render(content)
}

// renderBurnRateCard 渲染燃烧率卡片
func (md *MetricsDisplay) renderBurnRateCard(styles Styles, width int) string {
	burnRate := fmt.Sprintf("%.1f tok/min", md.metrics.BurnRate)
	costRate := fmt.Sprintf("$%.2f/hr", md.metrics.CostPerHour)

	// 根据燃烧率设置颜色
	style := styles.Success
	icon := "🟢"
	status := "Normal"

	if md.metrics.BurnRate > 100 {
		style = styles.Warning
		icon = "🟡"
		status = "High"
	}
	if md.metrics.BurnRate > 200 {
		style = styles.Error
		icon = "🔴"
		status = "Very High"
	}

	title := styles.MetricCardTitle.Render(fmt.Sprintf("%s Burn Rate", icon))
	burnLine := fmt.Sprintf("Tokens: %s", style.Render(burnRate))
	costLine := fmt.Sprintf("Cost: %s", styles.MetricValue.Render(costRate))
	statusLine := fmt.Sprintf("Status: %s", style.Render(status))

	content := lipgloss.JoinVertical(lipgloss.Left, title, burnLine, costLine, statusLine)
	return styles.MetricCard.Width(width).Render(content)
}

// renderProjectionCard 渲染预测卡片
func (md *MetricsDisplay) renderProjectionCard(styles Styles, width int) string {
	// 会话进度
	progress := fmt.Sprintf("%.1f%%", md.metrics.SessionProgress)
	remaining := formatDuration(md.metrics.TimeRemaining)
	confidence := fmt.Sprintf("%.0f%%", md.metrics.ConfidenceLevel)

	// 预测结束时间
	endTime := "N/A"
	if !md.metrics.PredictedEndTime.IsZero() {
		endTime = md.metrics.PredictedEndTime.Format("15:04")
	}

	// 根据置信度设置样式
	confStyle := styles.Success
	if md.metrics.ConfidenceLevel < 50 {
		confStyle = styles.Warning
	}
	if md.metrics.ConfidenceLevel < 25 {
		confStyle = styles.Error
	}

	title := styles.MetricCardTitle.Render("🎯 Projections")
	progressLine := fmt.Sprintf("Progress: %s", styles.MetricValue.Render(progress))
	remainingLine := fmt.Sprintf("Time Left: %s", styles.MetricLabel.Render(remaining))
	endLine := fmt.Sprintf("Est. End: %s", styles.MetricLabel.Render(endTime))
	confLine := fmt.Sprintf("Confidence: %s", confStyle.Render(confidence))

	content := lipgloss.JoinVertical(lipgloss.Left, title, progressLine, remainingLine, endLine, confLine)
	return styles.MetricCard.Width(width).Render(content)
}

// renderModelDistribution 渲染模型分布
func (md *MetricsDisplay) renderModelDistribution(styles Styles) string {
	if len(md.metrics.ModelDistribution) == 0 {
		return ""
	}

	title := styles.MetricCardTitle.Render("🤖 Model Distribution")
	var items []string

	for model, metrics := range md.metrics.ModelDistribution {
		tokens := formatNumber(metrics.TokenCount)
		cost := fmt.Sprintf("$%.2f", metrics.Cost)
		percent := fmt.Sprintf("%.1f%%", metrics.Percentage)

		// 创建进度条
		progressBar := md.renderProgressBar(metrics.Percentage, 20, styles)

		modelLine := fmt.Sprintf(
			"%s %s %s %s",
			styles.ModelName.Render(truncateString(model, 15)),
			styles.ModelValue.Render(tokens),
			styles.ModelValue.Render(cost),
			styles.ModelPercent.Render(percent),
		)

		item := lipgloss.JoinVertical(lipgloss.Left, modelLine, progressBar)
		items = append(items, item)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(items, "\n"))
	return styles.MetricCard.Width(md.width-2).Render(content)
}

// renderProgressBar 渲染进度条
func (md *MetricsDisplay) renderProgressBar(percentage float64, width int, styles Styles) string {
	if width <= 0 {
		return ""
	}

	filled := int((percentage / 100.0) * float64(width))
	if filled > width {
		filled = width
	}

	filledPart := strings.Repeat("█", filled)
	emptyPart := strings.Repeat("░", width-filled)

	// 根据百分比选择颜色
	fillStyle := styles.Success
	if percentage > 60 {
		fillStyle = styles.Warning
	}
	if percentage > 80 {
		fillStyle = styles.Error
	}

	return fillStyle.Render(filledPart) + styles.Muted.Render(emptyPart)
}

// renderNoData 渲染无数据状态
func (md *MetricsDisplay) renderNoData() string {
	styles := md.getStyles()
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"📊",
		"No metrics data available",
		"Start using Claude to see real-time metrics",
	)
	return styles.MetricCard.
		Width(md.width-4).
		Height(md.height-4).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

// getStyles 获取样式配置
func (md *MetricsDisplay) getStyles() Styles {
	return Styles{
		MetricCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1).
			Margin(0, 1),

		MetricCardTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1),

		MetricValue: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("46")),

		MetricLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),

		ModelName: lipgloss.NewStyle().
			Width(15).
			Foreground(lipgloss.Color("39")),

		ModelValue: lipgloss.NewStyle().
			Width(8).
			Align(lipgloss.Right).
			Foreground(lipgloss.Color("252")),

		ModelPercent: lipgloss.NewStyle().
			Width(6).
			Align(lipgloss.Right).
			Foreground(lipgloss.Color("243")),
	}
}

// Helper functions

// formatNumber 格式化数字显示
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

// formatDuration 格式化时间显示
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

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// RenderCompact 渲染紧凑版本的指标显示
func (md *MetricsDisplay) RenderCompact() string {
	if md.metrics == nil {
		return "No data"
	}

	current := formatNumber(md.metrics.CurrentTokens)
	cost := fmt.Sprintf("$%.2f", md.metrics.CurrentCost)
	rate := fmt.Sprintf("%.0f/min", md.metrics.TokensPerMinute)
	progress := fmt.Sprintf("%.0f%%", md.metrics.SessionProgress)

	return fmt.Sprintf(
		"📊 %s tokens | 💰 %s | ⚡ %s | 🎯 %s",
		current, cost, rate, progress,
	)
}

// GetSummary 获取指标摘要信息
func (md *MetricsDisplay) GetSummary() string {
	if md.metrics == nil {
		return "No metrics available"
	}

	var parts []string

	// Token信息
	if md.metrics.CurrentTokens > 0 {
		parts = append(parts, fmt.Sprintf("%s tokens", formatNumber(md.metrics.CurrentTokens)))
	}

	// 成本信息
	if md.metrics.CurrentCost > 0 {
		parts = append(parts, fmt.Sprintf("$%.2f cost", md.metrics.CurrentCost))
	}

	// 燃烧率
	if md.metrics.BurnRate > 0 {
		parts = append(parts, fmt.Sprintf("%.0f tok/min", md.metrics.BurnRate))
	}

	// 会话进度
	if md.metrics.SessionProgress > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%% complete", md.metrics.SessionProgress))
	}

	if len(parts) == 0 {
		return "Session starting..."
	}

	return strings.Join(parts, " • ")
}