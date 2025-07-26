package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/limits"
	"github.com/penwyp/ClawCat/ui/components"
)

// EnhancedDashboardView 增强的 Dashboard 视图，包含进度条、限额显示和统计表格
type EnhancedDashboardView struct {
	stats           Statistics
	metrics         *calculations.RealtimeMetrics
	progressSection *components.ProgressSection
	limitDisplay    *components.LimitDisplay
	statisticsTable *components.StatisticsTable
	limitManager    *limits.LimitManager
	limits          components.Limits
	width           int
	height          int
	config          Config
	styles          Styles
}

// NewEnhancedDashboardView 创建增强的 Dashboard
func NewEnhancedDashboardView(config Config) *EnhancedDashboardView {
	limitDisplay := components.NewLimitDisplay()

	return &EnhancedDashboardView{
		config:          config,
		styles:          NewStyles(GetThemeByName(config.Theme)),
		progressSection: components.NewProgressSection(0),
		limitDisplay:    limitDisplay,
		statisticsTable: components.NewStatisticsTable(0),
		limits:          getLimitsFromConfig(config),
	}
}

// Init 初始化增强的 dashboard 视图
func (d *EnhancedDashboardView) Init() tea.Cmd {
	return nil
}

// Update 处理增强 dashboard 的消息
func (d *EnhancedDashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return d, nil
}

// View 渲染增强的 Dashboard
func (d *EnhancedDashboardView) View() string {
	if d.width == 0 || d.height == 0 {
		// Default reasonable size if not set yet
		d.width = 80
		d.height = 24
	}

	// 更新进度条宽度
	if d.progressSection != nil {
		d.progressSection.SetWidth(d.width - 4)
	}

	// 渲染各个部分
	header := d.renderHeader()
	limits := d.renderLimitsSection()
	progress := d.renderProgressSection()
	metrics := d.renderMetrics()
	statistics := d.renderStatisticsSection()
	charts := d.renderCharts()
	footer := d.renderFooter()

	// 组合所有部分
	sections := []string{header}

	if limits != "" {
		sections = append(sections, limits)
	}

	if progress != "" {
		sections = append(sections, progress)
	}

	sections = append(sections, metrics)

	if statistics != "" {
		sections = append(sections, statistics)
	}

	sections = append(sections, charts, footer)

	content := strings.Join(sections, "\n\n")

	return d.styles.Content.
		Width(d.width - 4).
		Height(d.height - 4).
		Render(content)
}

// UpdateStats 更新 dashboard 统计数据
func (d *EnhancedDashboardView) UpdateStats(stats Statistics) {
	d.stats = stats
}

// UpdateMetrics 更新实时指标和进度条
func (d *EnhancedDashboardView) UpdateMetrics(metrics *calculations.RealtimeMetrics) {
	d.metrics = metrics
	if d.progressSection != nil && metrics != nil {
		d.progressSection.Update(metrics, d.limits)
	}
	if d.statisticsTable != nil && metrics != nil {
		d.statisticsTable.Update(metrics)
	}
}

// SetLimitManager 设置限额管理器
func (d *EnhancedDashboardView) SetLimitManager(lm *limits.LimitManager) {
	d.limitManager = lm
	if d.limitDisplay != nil && lm != nil {
		status := lm.GetStatus()
		d.limitDisplay.Update(status)
	}
}

// Resize 更新 dashboard 尺寸
func (d *EnhancedDashboardView) Resize(width, height int) {
	d.width = width
	d.height = height
	if d.progressSection != nil {
		d.progressSection.SetWidth(width - 4)
	}
	if d.limitDisplay != nil {
		d.limitDisplay.SetWidth(width - 4)
	}
	if d.statisticsTable != nil {
		d.statisticsTable.SetWidth(width - 4)
	}
}

// UpdateConfig 更新 dashboard 配置
func (d *EnhancedDashboardView) UpdateConfig(config Config) {
	d.config = config
	d.styles = NewStyles(GetThemeByName(config.Theme))
	d.limits = getLimitsFromConfig(config)
}

// renderHeader 渲染 dashboard 头部
func (d *EnhancedDashboardView) renderHeader() string {
	title := d.styles.Title.Render("🐱 ClawCat Enhanced Dashboard")
	subtitle := d.styles.Subtitle.Render(
		fmt.Sprintf("Last updated: %s", time.Now().Format("15:04:05")),
	)

	return strings.Join([]string{title, subtitle}, "\n")
}

// renderLimitsSection 渲染限额状态区域
func (d *EnhancedDashboardView) renderLimitsSection() string {
	if d.limitDisplay == nil {
		return ""
	}

	// 如果有限额管理器，更新最新状态
	if d.limitManager != nil {
		status := d.limitManager.GetStatus()
		d.limitDisplay.Update(status)
	}

	// 根据屏幕空间决定是否展开显示
	expanded := d.width > 80 && d.height > 30
	d.limitDisplay.SetExpanded(expanded)

	return d.limitDisplay.Render()
}

// renderProgressSection 渲染进度条区域
func (d *EnhancedDashboardView) renderProgressSection() string {
	if d.progressSection == nil || d.metrics == nil {
		return d.styles.Loading.Render("⏳ Waiting for session data...")
	}

	return d.progressSection.Render()
}

// renderStatisticsSection 渲染统计表格区域
func (d *EnhancedDashboardView) renderStatisticsSection() string {
	if d.statisticsTable == nil || d.metrics == nil {
		return ""
	}

	return d.statisticsTable.Render()
}

// renderMetrics 渲染关键指标卡片
func (d *EnhancedDashboardView) renderMetrics() string {
	// 活跃会话卡片
	activeCard := d.renderMetricCard(
		"Active Sessions",
		fmt.Sprintf("%d", d.stats.ActiveSessions),
		d.styles.Success,
	)

	// 总 tokens 卡片
	tokensCard := d.renderMetricCard(
		"Total Tokens",
		d.formatNumber(d.stats.TotalTokens),
		d.styles.Info,
	)

	// 总成本卡片
	costCard := d.renderMetricCard(
		"Total Cost",
		fmt.Sprintf("$%.2f", d.stats.TotalCost),
		d.styles.Warning,
	)

	// 燃烧率卡片 - 增强显示
	burnRateCard := d.renderBurnRateCard()

	// 横向排列卡片
	return d.arrangeInRow([]string{
		activeCard,
		tokensCard,
		costCard,
		burnRateCard,
	})
}

// renderBurnRateCard 渲染增强的燃烧率卡片
func (d *EnhancedDashboardView) renderBurnRateCard() string {
	var burnRateText, statusStyle string

	if d.metrics != nil {
		burnRate := d.metrics.BurnRate
		if burnRate > 200 {
			statusStyle = "🔥 High"
			burnRateText = fmt.Sprintf("%.1f tok/min", burnRate)
		} else if burnRate > 100 {
			statusStyle = "⚡ Medium"
			burnRateText = fmt.Sprintf("%.1f tok/min", burnRate)
		} else {
			statusStyle = "✅ Normal"
			burnRateText = fmt.Sprintf("%.1f tok/min", burnRate)
		}
	} else {
		statusStyle = "📊 Calculating"
		burnRateText = fmt.Sprintf("%.1f tok/hr", d.stats.CurrentBurnRate)
	}

	// 根据燃烧率选择颜色
	var style lipgloss.Style
	if strings.Contains(statusStyle, "High") {
		style = d.styles.Error
	} else if strings.Contains(statusStyle, "Medium") {
		style = d.styles.Warning
	} else {
		style = d.styles.Success
	}

	cardTitle := d.styles.DashboardLabel().Render("Burn Rate")
	cardValue := style.Render(burnRateText)
	cardStatus := d.styles.Muted.Render(statusStyle)

	content := strings.Join([]string{cardTitle, cardValue, cardStatus}, "\n")
	return d.styles.DashboardCard().Render(content)
}

// renderCharts 渲染 dashboard 图表
func (d *EnhancedDashboardView) renderCharts() string {
	// 使用进度条状态
	usageInfo := d.renderUsageInfo()

	// 时间重置信息
	resetInfo := d.renderResetInfo()

	return strings.Join([]string{usageInfo, resetInfo}, "\n\n")
}

// renderUsageInfo 渲染使用情况信息
func (d *EnhancedDashboardView) renderUsageInfo() string {
	title := d.styles.Subtitle.Render("Resource Usage Status")

	var statusInfo string
	if d.progressSection != nil {
		summary := d.progressSection.GetSummary()
		worstStatus := d.progressSection.GetWorstStatus()

		var statusStyle lipgloss.Style
		var statusIcon string
		switch worstStatus {
		case "critical":
			statusStyle = d.styles.Error
			statusIcon = "🚨"
		case "warning":
			statusStyle = d.styles.Warning
			statusIcon = "⚠️"
		case "moderate":
			statusStyle = d.styles.Info
			statusIcon = "📊"
		default:
			statusStyle = d.styles.Success
			statusIcon = "✅"
		}

		statusInfo = statusStyle.Render(fmt.Sprintf("%s %s", statusIcon, summary))
	} else {
		progressWidth := d.width - 20
		if progressWidth < 20 {
			progressWidth = 20
		}

		progress := d.styles.ProgressStyle(d.stats.PlanUsage, progressWidth)
		percentage := fmt.Sprintf("%.1f%%", d.stats.PlanUsage)
		statusInfo = strings.Join([]string{progress, percentage}, "\n")
	}

	return strings.Join([]string{title, statusInfo}, "\n")
}

// renderFooter 渲染 dashboard 页脚
func (d *EnhancedDashboardView) renderFooter() string {
	status := fmt.Sprintf(
		"Sessions: %d | Top Model: %s | Avg Cost: $%.2f",
		d.stats.SessionCount,
		d.stats.TopModel,
		d.stats.AverageCost,
	)

	// 添加进度条状态摘要
	if d.progressSection != nil && d.progressSection.HasCriticalStatus() {
		status += " | ⚠️ Critical usage levels detected"
	}

	// 添加限额状态摘要
	if d.limitDisplay != nil {
		limitStatus := d.limitDisplay.GetStatus()
		if limitStatus != nil {
			if limitStatus.WarningLevel != nil {
				severityIcon := ""
				switch limitStatus.WarningLevel.Severity {
				case limits.SeverityInfo:
					severityIcon = "ℹ️"
				case limits.SeverityWarning:
					severityIcon = "⚠️"
				case limits.SeverityError:
					severityIcon = "🚨"
				case limits.SeverityCritical:
					severityIcon = "❌"
				}
				status += fmt.Sprintf(" | %s Limit: %.1f%%", severityIcon, limitStatus.Percentage)
			} else if limitStatus.Percentage > 50 {
				status += fmt.Sprintf(" | 📊 Limit: %.1f%%", limitStatus.Percentage)
			}
		}
	}

	return d.styles.Footer.Render(status)
}

// renderResetInfo 渲染重置时间信息
func (d *EnhancedDashboardView) renderResetInfo() string {
	title := d.styles.Subtitle.Render("Time to Reset")

	days := int(d.stats.TimeToReset.Hours() / 24)
	hours := int(d.stats.TimeToReset.Hours()) % 24

	timeText := fmt.Sprintf("%dd %dh", days, hours)

	return strings.Join([]string{
		title,
		d.styles.Normal.Render(timeText),
	}, "\n")
}

// renderMetricCard 渲染单个指标卡片
func (d *EnhancedDashboardView) renderMetricCard(title, value string, style lipgloss.Style) string {
	cardTitle := d.styles.DashboardLabel().Render(title)
	cardValue := style.Render(value)

	content := strings.Join([]string{cardTitle, cardValue}, "\n")

	return d.styles.DashboardCard().Render(content)
}

// arrangeInRow 水平排列多个元素
func (d *EnhancedDashboardView) arrangeInRow(elements []string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, elements...)
}

// formatNumber 格式化大数字显示
func (d *EnhancedDashboardView) formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// getLimitsFromConfig 从配置获取限制值
func getLimitsFromConfig(config Config) components.Limits {
	// 这里暂时使用默认值，实际应该从配置中读取
	return components.Limits{
		TokenLimit: 1000000, // 1M tokens
		CostLimit:  18.00,   // $18 Pro plan default
	}
}

// HasProgressData 检查是否有进度数据
func (d *EnhancedDashboardView) HasProgressData() bool {
	return d.metrics != nil
}

// GetProgressSummary 获取进度摘要
func (d *EnhancedDashboardView) GetProgressSummary() string {
	if d.progressSection != nil {
		return d.progressSection.GetSummary()
	}
	return "No progress data available"
}

// UpdateLimitStatus 更新限额状态
func (d *EnhancedDashboardView) UpdateLimitStatus(tokens int64, cost float64) error {
	if d.limitManager == nil {
		return nil
	}

	err := d.limitManager.UpdateUsage(tokens, cost)
	if err != nil {
		return err
	}

	// 更新限额显示
	if d.limitDisplay != nil {
		status := d.limitManager.GetStatus()
		d.limitDisplay.Update(status)
	}

	return nil
}

// GetLimitStatus 获取当前限额状态
func (d *EnhancedDashboardView) GetLimitStatus() *limits.LimitStatus {
	if d.limitManager == nil {
		return nil
	}
	return d.limitManager.GetStatus()
}

// IsOverLimit 检查是否超过限额
func (d *EnhancedDashboardView) IsOverLimit() bool {
	if d.limitDisplay == nil {
		return false
	}
	return d.limitDisplay.IsOverLimit()
}

// GetQuickLimitStatus 获取快速限额状态
func (d *EnhancedDashboardView) GetQuickLimitStatus() string {
	if d.limitDisplay == nil {
		return "No limit data"
	}
	return d.limitDisplay.RenderQuickStatus()
}
