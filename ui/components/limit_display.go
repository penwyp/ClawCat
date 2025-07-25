package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/limits"
)

// LimitDisplayStyles 限额显示样式
type LimitDisplayStyles struct {
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Normal   lipgloss.Style
	Bold     lipgloss.Style
	Border   lipgloss.Style
	Faint    lipgloss.Style
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Badge    lipgloss.Style
}

// DefaultLimitDisplayStyles 默认限额显示样式
func DefaultLimitDisplayStyles() LimitDisplayStyles {
	return LimitDisplayStyles{
		Success:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		Warning:  lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		Error:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		Info:     lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		Normal:   lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		Bold:     lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true),
		Border:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1),
		Faint:    lipgloss.NewStyle().Faint(true),
		Title:    lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).MarginBottom(1),
		Subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true),
		Badge:    lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("15")).Padding(0, 1),
	}
}

// DashboardCard 获取仪表板卡片样式
func (lds LimitDisplayStyles) DashboardCard() lipgloss.Style {
	return lds.Border.Copy().Width(30).Height(8)
}

// LimitDisplay 限额显示组件
type LimitDisplay struct {
	status   *limits.LimitStatus
	styles   LimitDisplayStyles
	width    int
	expanded bool
}

// NewLimitDisplay 创建限额显示组件
func NewLimitDisplay() *LimitDisplay {
	return &LimitDisplay{
		styles:   DefaultLimitDisplayStyles(),
		expanded: false,
	}
}

// Update 更新限额状态
func (ld *LimitDisplay) Update(status *limits.LimitStatus) {
	ld.status = status
}

// SetWidth 设置组件宽度
func (ld *LimitDisplay) SetWidth(width int) {
	ld.width = width
}

// SetExpanded 设置是否展开显示
func (ld *LimitDisplay) SetExpanded(expanded bool) {
	ld.expanded = expanded
}

// Render 渲染限额显示
func (ld *LimitDisplay) Render() string {
	if ld.status == nil {
		return ld.styles.Faint.Render("Loading limit status...")
	}

	if ld.expanded {
		return ld.renderExpanded()
	}
	return ld.renderCompact()
}

// renderCompact 渲染紧凑视图
func (ld *LimitDisplay) renderCompact() string {
	// 计划名称和使用百分比
	planBadge := ld.renderPlanBadge()
	usageBar := ld.renderMiniProgressBar()

	// 警告信息
	warning := ""
	if ld.status.WarningLevel != nil {
		style := ld.getWarningStyle(ld.status.WarningLevel.Severity)
		icon := ld.getWarningIcon(ld.status.WarningLevel.Severity)
		warning = style.Render(fmt.Sprintf("%s %.0f%%", icon, ld.status.Percentage))
	}

	parts := []string{planBadge, usageBar}
	if warning != "" {
		parts = append(parts, warning)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// renderExpanded 渲染展开视图
func (ld *LimitDisplay) renderExpanded() string {
	sections := []string{
		ld.renderHeader(),
		ld.renderUsageDetails(),
		ld.renderProgressSection(),
	}

	if ld.status.WarningLevel != nil {
		sections = append(sections, ld.renderWarning())
	}

	if len(ld.status.Recommendations) > 0 {
		sections = append(sections, ld.renderRecommendations())
	}

	content := strings.Join(sections, "\n\n")

	return ld.styles.DashboardCard().
		Width(ld.width).
		Render(content)
}

// renderHeader 渲染标题
func (ld *LimitDisplay) renderHeader() string {
	title := fmt.Sprintf("💳 %s Plan", ld.status.Plan.Name)
	subtitle := fmt.Sprintf("Resets in %s", ld.formatDuration(ld.status.TimeToReset))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ld.styles.Title.Render(title),
		ld.styles.Subtitle.Render(subtitle),
	)
}

// renderUsageDetails 渲染使用详情
func (ld *LimitDisplay) renderUsageDetails() string {
	details := []string{
		fmt.Sprintf("💰 Cost: $%.2f / $%.2f",
			ld.status.CurrentUsage.Cost,
			ld.status.Plan.CostLimit,
		),
	}

	if ld.status.Plan.TokenLimit > 0 {
		details = append(details, fmt.Sprintf("🔢 Tokens: %s / %s",
			ld.formatNumber(ld.status.CurrentUsage.Tokens),
			ld.formatNumber(ld.status.Plan.TokenLimit),
		))
	}

	details = append(details, fmt.Sprintf("📊 Usage: %.1f%%", ld.status.Percentage))

	// 根据使用率应用颜色
	style := ld.styles.Normal
	if ld.status.Percentage > 90 {
		style = ld.styles.Error
	} else if ld.status.Percentage > 75 {
		style = ld.styles.Warning
	}

	return style.Render(strings.Join(details, "\n"))
}

// renderProgressSection 渲染进度条区域
func (ld *LimitDisplay) renderProgressSection() string {
	// 成本进度条
	costProgress := NewProgressBar(
		"Cost Usage",
		ld.status.CurrentUsage.Cost,
		ld.status.Plan.CostLimit,
	)
	costProgress.Color = ld.getProgressColor(ld.status.Percentage)
	costProgress.SetWidth(ld.width - 10)

	progressBars := []string{costProgress.Render()}

	// Token 进度条（如果有限制）
	if ld.status.Plan.TokenLimit > 0 {
		tokenProgress := NewProgressBar(
			"Token Usage",
			float64(ld.status.CurrentUsage.Tokens),
			float64(ld.status.Plan.TokenLimit),
		)
		tokenPercentage := float64(ld.status.CurrentUsage.Tokens) / float64(ld.status.Plan.TokenLimit) * 100
		tokenProgress.Color = ld.getProgressColor(tokenPercentage)
		tokenProgress.SetWidth(ld.width - 10)
		progressBars = append(progressBars, tokenProgress.Render())
	}

	return strings.Join(progressBars, "\n")
}

// renderWarning 渲染警告
func (ld *LimitDisplay) renderWarning() string {
	level := ld.status.WarningLevel
	style := ld.getWarningStyle(level.Severity)

	icon := ld.getWarningIcon(level.Severity)
	message := fmt.Sprintf("%s %s", icon, level.Message)

	return style.
		Padding(1).
		Margin(0, 1).
		Render(message)
}

// renderRecommendations 渲染建议
func (ld *LimitDisplay) renderRecommendations() string {
	title := ld.styles.Subtitle.Render("💡 Recommendations")

	items := []string{}
	for _, rec := range ld.status.Recommendations {
		items = append(items, fmt.Sprintf("• %s", rec))
	}

	content := ld.styles.Faint.Render(strings.Join(items, "\n"))

	return strings.Join([]string{title, content}, "\n")
}

// renderPlanBadge 渲染计划徽章
func (ld *LimitDisplay) renderPlanBadge() string {
	badge := fmt.Sprintf(" %s ", ld.status.Plan.Name)

	style := ld.styles.Badge
	if ld.status.Percentage > 90 {
		style = style.Background(lipgloss.Color("#FF0000"))
	} else if ld.status.Percentage > 75 {
		style = style.Background(lipgloss.Color("#FFA500"))
	}

	return style.Render(badge)
}

// renderMiniProgressBar 渲染迷你进度条
func (ld *LimitDisplay) renderMiniProgressBar() string {
	width := 20
	filled := int(ld.status.Percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := fmt.Sprintf(" %.0f%%", ld.status.Percentage)

	color := ld.getProgressColor(ld.status.Percentage)

	return lipgloss.NewStyle().
		Foreground(color).
		Render(bar + percentage)
}

// getWarningStyle 获取警告样式
func (ld *LimitDisplay) getWarningStyle(severity limits.Severity) lipgloss.Style {
	switch severity {
	case limits.SeverityInfo:
		return ld.styles.Info
	case limits.SeverityWarning:
		return ld.styles.Warning
	case limits.SeverityError:
		return ld.styles.Error
	case limits.SeverityCritical:
		return ld.styles.Error.Bold(true).Blink(true)
	default:
		return ld.styles.Normal
	}
}

// getWarningIcon 获取警告图标
func (ld *LimitDisplay) getWarningIcon(severity limits.Severity) string {
	switch severity {
	case limits.SeverityInfo:
		return "ℹ️"
	case limits.SeverityWarning:
		return "⚠️"
	case limits.SeverityError:
		return "🚨"
	case limits.SeverityCritical:
		return "❌"
	default:
		return "•"
	}
}

// getProgressColor 获取进度条颜色
func (ld *LimitDisplay) getProgressColor(percentage float64) lipgloss.Color {
	if percentage >= 95 {
		return lipgloss.Color("#FF0000") // 红色
	} else if percentage >= 90 {
		return lipgloss.Color("#FF4500") // 橙红色
	} else if percentage >= 75 {
		return lipgloss.Color("#FFA500") // 橙色
	} else if percentage >= 50 {
		return lipgloss.Color("#FFD700") // 金色
	}
	return lipgloss.Color("#00FF00") // 绿色
}

// formatDuration 格式化时间显示
func (ld *LimitDisplay) formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// formatNumber 格式化大数字
func (ld *LimitDisplay) formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// GetStatus 获取当前限额状态
func (ld *LimitDisplay) GetStatus() *limits.LimitStatus {
	return ld.status
}

// IsOverLimit 检查是否超过限额
func (ld *LimitDisplay) IsOverLimit() bool {
	return ld.status != nil && ld.status.Percentage >= 100
}

// IsNearLimit 检查是否接近限额
func (ld *LimitDisplay) IsNearLimit(threshold float64) bool {
	return ld.status != nil && ld.status.Percentage >= threshold
}

// GetUsagePercentage 获取使用百分比
func (ld *LimitDisplay) GetUsagePercentage() float64 {
	if ld.status == nil {
		return 0
	}
	return ld.status.Percentage
}

// GetRemainingBudget 获取剩余预算
func (ld *LimitDisplay) GetRemainingBudget() float64 {
	if ld.status == nil {
		return 0
	}
	remaining := ld.status.Plan.CostLimit - ld.status.CurrentUsage.Cost
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetRemainingTokens 获取剩余 tokens
func (ld *LimitDisplay) GetRemainingTokens() int64 {
	if ld.status == nil || ld.status.Plan.TokenLimit <= 0 {
		return 0
	}
	remaining := ld.status.Plan.TokenLimit - ld.status.CurrentUsage.Tokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RenderQuickStatus 渲染快速状态（单行）
func (ld *LimitDisplay) RenderQuickStatus() string {
	if ld.status == nil {
		return ld.styles.Faint.Render("No limit data")
	}

	planName := ld.status.Plan.Name
	percentage := fmt.Sprintf("%.1f%%", ld.status.Percentage)
	remaining := ld.GetRemainingBudget()

	color := ld.getProgressColor(ld.status.Percentage)
	statusText := fmt.Sprintf("%s: %s ($%.2f left)", planName, percentage, remaining)

	return lipgloss.NewStyle().
		Foreground(color).
		Render(statusText)
}