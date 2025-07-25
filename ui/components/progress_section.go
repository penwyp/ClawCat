package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/calculations"
)

// Limits 订阅限制配置
type Limits struct {
	TokenLimit int     // Token 限制
	CostLimit  float64 // 成本限制
}

// ProgressStyles 进度条区域样式
type ProgressStyles struct {
	SectionTitle lipgloss.Style
	Box          lipgloss.Style
	Error        lipgloss.Style
	Warning      lipgloss.Style
}

// DefaultProgressStyles 默认进度条样式
func DefaultProgressStyles() ProgressStyles {
	return ProgressStyles{
		SectionTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")),
		Box:          lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#374151")).Padding(1),
		Error:        lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true),
		Warning:      lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true),
	}
}

// ProgressSection 进度条区域组件
type ProgressSection struct {
	TokenProgress *ProgressBar
	CostProgress  *ProgressBar
	TimeProgress  *ProgressBar
	styles        ProgressStyles
	width         int
	height        int
}

// NewProgressSection 创建进度条区域
func NewProgressSection(width int) *ProgressSection {
	return &ProgressSection{
		width:  width,
		styles: DefaultProgressStyles(),
	}
}

// Update 更新进度条数据
func (ps *ProgressSection) Update(metrics *calculations.RealtimeMetrics, limits Limits) {
	// 更新 Token 进度条
	ps.TokenProgress = ps.createTokenProgress(metrics, limits)

	// 更新成本进度条
	ps.CostProgress = ps.createCostProgress(metrics, limits)

	// 更新时间进度条
	ps.TimeProgress = ps.createTimeProgress(metrics)
}

// createTokenProgress 创建 Token 进度条
func (ps *ProgressSection) createTokenProgress(metrics *calculations.RealtimeMetrics, limits Limits) *ProgressBar {
	pb := NewProgressBar(
		"Token Usage",
		float64(metrics.CurrentTokens),
		float64(limits.TokenLimit),
	)

	// 设置动态颜色
	colorScheme := DefaultColorScheme
	pb.Color = colorScheme.GetProgressColor(pb.Percentage)

	// 自定义显示格式
	pb.Style.ValueStyle = lipgloss.NewStyle().Bold(true)

	// 根据屏幕宽度调整进度条宽度
	barWidth := ps.calculateBarWidth()
	pb.SetWidth(barWidth)

	return pb
}

// createCostProgress 创建成本进度条
func (ps *ProgressSection) createCostProgress(metrics *calculations.RealtimeMetrics, limits Limits) *ProgressBar {
	pb := NewProgressBar(
		"Cost Usage",
		metrics.CurrentCost,
		limits.CostLimit,
	)

	// 设置动态颜色
	colorScheme := DefaultColorScheme
	pb.Color = colorScheme.GetProgressColor(pb.Percentage)

	// 成本显示特殊格式
	pb.Style.ValueStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFD700")) // 金色

	barWidth := ps.calculateBarWidth()
	pb.SetWidth(barWidth)

	return pb
}

// createTimeProgress 创建时间进度条
func (ps *ProgressSection) createTimeProgress(metrics *calculations.RealtimeMetrics) *ProgressBar {
	elapsed := time.Since(metrics.SessionStart)
	total := 5 * time.Hour

	pb := NewProgressBar(
		"Time Elapsed",
		elapsed.Minutes(),
		total.Minutes(),
	)

	// 时间进度条使用渐变颜色
	pb.Color = ps.getTimeProgressColor(pb.Percentage)

	// 自定义时间显示
	pb.ShowValue = false // 使用自定义格式

	barWidth := ps.calculateBarWidth()
	pb.SetWidth(barWidth)

	return pb
}

// Render 渲染进度条区域
func (ps *ProgressSection) Render() string {
	if ps.width == 0 {
		return "Loading progress..."
	}

	// 区域标题
	title := ps.styles.SectionTitle.Render("📊 Progress Overview")

	// 渲染各个进度条
	var progressBars []string

	if ps.TokenProgress != nil {
		progressBars = append(progressBars, ps.renderProgressWithInfo(ps.TokenProgress))
	}

	if ps.CostProgress != nil {
		progressBars = append(progressBars, ps.renderProgressWithInfo(ps.CostProgress))
	}

	if ps.TimeProgress != nil {
		progressBars = append(progressBars, ps.renderTimeProgress())
	}

	// 组合所有元素
	content := strings.Join(append([]string{title}, progressBars...), "\n\n")

	// 添加边框
	return ps.styles.Box.
		Width(ps.width).
		Render(content)
}

// renderProgressWithInfo 渲染带附加信息的进度条
func (ps *ProgressSection) renderProgressWithInfo(pb *ProgressBar) string {
	progressBar := pb.Render()

	// 添加附加信息
	var info string
	switch pb.Label {
	case "Token Usage":
		if pb.Percentage > 90 {
			info = ps.styles.Error.Render("⚠️ Approaching token limit!")
		} else if pb.Percentage > 75 {
			info = ps.styles.Warning.Render("⚡ High token usage")
		}
	case "Cost Usage":
		if pb.Percentage > 90 {
			info = ps.styles.Error.Render("💸 Budget alert!")
		} else if pb.Percentage > 75 {
			info = ps.styles.Warning.Render("💰 Monitor spending")
		}
	}

	if info != "" {
		return progressBar + "\n" + info
	}
	return progressBar
}

// renderTimeProgress 渲染时间进度条（特殊格式）
func (ps *ProgressSection) renderTimeProgress() string {
	if ps.TimeProgress == nil {
		return ""
	}

	// 基础进度条
	bar := ps.TimeProgress.Render()

	// 添加时间显示
	elapsed := time.Duration(ps.TimeProgress.Current) * time.Minute
	remaining := time.Duration(ps.TimeProgress.Max-ps.TimeProgress.Current) * time.Minute

	timeInfo := fmt.Sprintf("%s elapsed, %s remaining",
		formatProgressDuration(elapsed),
		formatProgressDuration(remaining),
	)

	timeStyle := lipgloss.NewStyle().Faint(true)

	return bar + "\n" + timeStyle.Render(timeInfo)
}

// calculateBarWidth 计算进度条宽度
func (ps *ProgressSection) calculateBarWidth() int {
	// 预留空间给标签、数值和边距
	reservedSpace := 40
	availableWidth := ps.width - reservedSpace

	if availableWidth < 20 {
		return 20
	}
	if availableWidth > 60 {
		return 60
	}

	return availableWidth
}

// getTimeProgressColor 获取时间进度条颜色
func (ps *ProgressSection) getTimeProgressColor(percentage float64) lipgloss.Color {
	// 时间进度条使用蓝色系渐变
	if percentage < 25 {
		return lipgloss.Color("#00BFFF") // 深天蓝
	} else if percentage < 50 {
		return lipgloss.Color("#1E90FF") // 道奇蓝
	} else if percentage < 75 {
		return lipgloss.Color("#4169E1") // 皇家蓝
	} else {
		return lipgloss.Color("#6A5ACD") // 石板蓝
	}
}

// formatProgressDuration 格式化时间显示
func formatProgressDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// SetWidth 设置进度条区域宽度
func (ps *ProgressSection) SetWidth(width int) {
	ps.width = width
}

// SetHeight 设置进度条区域高度
func (ps *ProgressSection) SetHeight(height int) {
	ps.height = height
}

// GetSummary 获取进度摘要信息
func (ps *ProgressSection) GetSummary() string {
	if ps.TokenProgress == nil || ps.CostProgress == nil || ps.TimeProgress == nil {
		return "No progress data available"
	}

	return fmt.Sprintf(
		"Tokens: %.1f%% | Cost: %.1f%% | Time: %.1f%%",
		ps.TokenProgress.Percentage,
		ps.CostProgress.Percentage,
		ps.TimeProgress.Percentage,
	)
}

// HasCriticalStatus 检查是否有临界状态
func (ps *ProgressSection) HasCriticalStatus() bool {
	return (ps.TokenProgress != nil && ps.TokenProgress.GetStatus() == "critical") ||
		(ps.CostProgress != nil && ps.CostProgress.GetStatus() == "critical")
}

// GetWorstStatus 获取最严重的状态
func (ps *ProgressSection) GetWorstStatus() string {
	statuses := []string{"normal"}

	if ps.TokenProgress != nil {
		statuses = append(statuses, ps.TokenProgress.GetStatus())
	}
	if ps.CostProgress != nil {
		statuses = append(statuses, ps.CostProgress.GetStatus())
	}
	if ps.TimeProgress != nil {
		statuses = append(statuses, ps.TimeProgress.GetStatus())
	}

	// 按严重程度排序
	statusPriority := map[string]int{
		"critical": 4,
		"warning":  3,
		"moderate": 2,
		"normal":   1,
	}

	worstStatus := "normal"
	worstPriority := 0

	for _, status := range statuses {
		if priority, exists := statusPriority[status]; exists && priority > worstPriority {
			worstStatus = status
			worstPriority = priority
		}
	}

	return worstStatus
}
