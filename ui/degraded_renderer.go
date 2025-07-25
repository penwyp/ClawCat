package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/sessions"
)

// DegradedRenderer 降级渲染器
type DegradedRenderer struct {
	width    int
	height   int
	fallback bool
	error    error
	message  string
}

// DegradedMode 降级模式类型
type DegradedMode int

const (
	DegradedModeMinimal DegradedMode = iota
	DegradedModeText
	DegradedModeBasic
	DegradedModeSafe
)

// NewDegradedRenderer 创建降级渲染器
func NewDegradedRenderer() *DegradedRenderer {
	return &DegradedRenderer{
		width:   80,
		height:  24,
		message: "System running in degraded mode",
	}
}

// SetError 设置错误信息
func (dr *DegradedRenderer) SetError(err error) {
	dr.error = err
	dr.fallback = true
}

// SetMessage 设置状态信息
func (dr *DegradedRenderer) SetMessage(msg string) {
	dr.message = msg
}

// Resize 调整大小
func (dr *DegradedRenderer) Resize(width, height int) {
	dr.width = width
	dr.height = height
}

// RenderDashboard 渲染降级仪表板
func (dr *DegradedRenderer) RenderDashboard(stats Statistics, mode DegradedMode) string {
	switch mode {
	case DegradedModeMinimal:
		return dr.renderMinimalDashboard(stats)
	case DegradedModeText:
		return dr.renderTextDashboard(stats)
	case DegradedModeBasic:
		return dr.renderBasicDashboard(stats)
	case DegradedModeSafe:
		return dr.renderSafeDashboard(stats)
	default:
		return dr.renderMinimalDashboard(stats)
	}
}

// renderMinimalDashboard 最小化仪表板（仅关键信息）
func (dr *DegradedRenderer) renderMinimalDashboard(stats Statistics) string {
	var lines []string
	
	// 标题
	lines = append(lines, "ClawCat - Minimal Mode")
	lines = append(lines, strings.Repeat("-", 25))
	
	// 关键统计信息
	lines = append(lines, fmt.Sprintf("Sessions: %d active, %d total", 
		stats.ActiveSessions, stats.SessionCount))
	lines = append(lines, fmt.Sprintf("Tokens: %d", stats.TotalTokens))
	lines = append(lines, fmt.Sprintf("Cost: $%.2f", stats.TotalCost))
	
	// 错误信息
	if dr.fallback {
		lines = append(lines, "")
		lines = append(lines, "WARNING: Running in fallback mode")
		if dr.error != nil {
			lines = append(lines, fmt.Sprintf("Error: %s", dr.error.Error()))
		}
	}
	
	// 状态信息
	lines = append(lines, "")
	lines = append(lines, dr.message)
	lines = append(lines, fmt.Sprintf("Last update: %s", time.Now().Format("15:04:05")))
	
	return strings.Join(lines, "\n")
}

// renderTextDashboard 纯文本仪表板
func (dr *DegradedRenderer) renderTextDashboard(stats Statistics) string {
	var lines []string
	
	// 头部信息
	lines = append(lines, "╭─ ClawCat Dashboard (Text Mode) ─╮")
	lines = append(lines, "│                                │")
	
	// 会话信息
	lines = append(lines, fmt.Sprintf("│ Sessions    : %3d active       │", stats.ActiveSessions))
	lines = append(lines, fmt.Sprintf("│ Total       : %3d sessions     │", stats.SessionCount))
	lines = append(lines, "│                                │")
	
	// 使用统计
	lines = append(lines, fmt.Sprintf("│ Tokens      : %-14d │", stats.TotalTokens))
	lines = append(lines, fmt.Sprintf("│ Cost        : $%-13.2f │", stats.TotalCost))
	lines = append(lines, fmt.Sprintf("│ Avg Cost    : $%-13.2f │", stats.AverageCost))
	lines = append(lines, "│                                │")
	
	// 模型信息
	if stats.TopModel != "" {
		lines = append(lines, fmt.Sprintf("│ Top Model   : %-14s │", truncateString(stats.TopModel, 14)))
	}
	
	// 速率信息
	if stats.CurrentBurnRate > 0 {
		lines = append(lines, fmt.Sprintf("│ Burn Rate   : %.1f tok/hr      │", stats.CurrentBurnRate))
	}
	
	// 错误信息
	if dr.fallback {
		lines = append(lines, "│                                │")
		lines = append(lines, "│ ⚠ DEGRADED MODE ACTIVE        │")
		if dr.error != nil {
			errMsg := truncateString(dr.error.Error(), 28)
			lines = append(lines, fmt.Sprintf("│ Error: %-22s │", errMsg))
		}
	}
	
	// 底部
	lines = append(lines, "│                                │")
	lines = append(lines, fmt.Sprintf("│ Updated: %-20s │", time.Now().Format("15:04:05")))
	lines = append(lines, "╰────────────────────────────────╯")
	
	return strings.Join(lines, "\n")
}

// renderBasicDashboard 基础仪表板（简化图表）
func (dr *DegradedRenderer) renderBasicDashboard(stats Statistics) string {
	var lines []string
	
	// 标题栏
	title := "ClawCat Dashboard - Basic Mode"
	lines = append(lines, centerText(title, dr.width))
	lines = append(lines, strings.Repeat("═", dr.width))
	lines = append(lines, "")
	
	// 主要指标
	lines = append(lines, "METRICS")
	lines = append(lines, strings.Repeat("-", 7))
	lines = append(lines, fmt.Sprintf("Sessions: %d active / %d total", 
		stats.ActiveSessions, stats.SessionCount))
	lines = append(lines, fmt.Sprintf("Tokens:   %d", stats.TotalTokens))
	lines = append(lines, fmt.Sprintf("Cost:     $%.2f (avg: $%.2f)", 
		stats.TotalCost, stats.AverageCost))
	
	if stats.TopModel != "" {
		lines = append(lines, fmt.Sprintf("Top Model: %s", stats.TopModel))
	}
	
	if stats.CurrentBurnRate > 0 {
		lines = append(lines, fmt.Sprintf("Burn Rate: %.1f tokens/hour", stats.CurrentBurnRate))
	}
	
	// 简化的使用量进度条
	lines = append(lines, "")
	lines = append(lines, "USAGE")
	lines = append(lines, strings.Repeat("-", 5))
	usageBar := dr.renderSimpleProgressBar("Plan Usage", stats.PlanUsage, 50)
	lines = append(lines, usageBar)
	
	// 时间信息
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Time to reset: %s", 
		formatDuration(stats.TimeToReset)))
	
	// 系统状态
	lines = append(lines, "")
	lines = append(lines, "STATUS")
	lines = append(lines, strings.Repeat("-", 6))
	if dr.fallback {
		lines = append(lines, "⚠️  System running in degraded mode")
		if dr.error != nil {
			lines = append(lines, fmt.Sprintf("    Error: %s", dr.error.Error()))
		}
	} else {
		lines = append(lines, "✅ System operational")
	}
	
	lines = append(lines, dr.message)
	lines = append(lines, fmt.Sprintf("Last update: %s", time.Now().Format("2006-01-02 15:04:05")))
	
	// 帮助信息
	lines = append(lines, "")
	lines = append(lines, "Press 'h' for help, 'q' to quit")
	
	return strings.Join(lines, "\n")
}

// renderSafeDashboard 安全模式仪表板（最高兼容性）
func (dr *DegradedRenderer) renderSafeDashboard(stats Statistics) string {
	var lines []string
	
	lines = append(lines, "ClawCat - Safe Mode")
	lines = append(lines, "==================")
	lines = append(lines, "")
	
	// 基本信息，避免复杂格式化
	lines = append(lines, "SESSION INFORMATION:")
	lines = append(lines, fmt.Sprintf("  Active sessions: %d", stats.ActiveSessions))
	lines = append(lines, fmt.Sprintf("  Total sessions: %d", stats.SessionCount))
	lines = append(lines, "")
	
	lines = append(lines, "USAGE STATISTICS:")
	lines = append(lines, fmt.Sprintf("  Total tokens: %d", stats.TotalTokens))
	lines = append(lines, fmt.Sprintf("  Total cost: %.2f USD", stats.TotalCost))
	
	if stats.SessionCount > 0 {
		lines = append(lines, fmt.Sprintf("  Average cost: %.2f USD", stats.AverageCost))
	}
	
	if stats.TopModel != "" {
		lines = append(lines, fmt.Sprintf("  Most used model: %s", stats.TopModel))
	}
	
	lines = append(lines, "")
	
	// 系统状态
	lines = append(lines, "SYSTEM STATUS:")
	if dr.fallback {
		lines = append(lines, "  WARNING: Degraded mode active")
		lines = append(lines, "  Some features may be unavailable")
		if dr.error != nil {
			lines = append(lines, fmt.Sprintf("  Last error: %s", dr.error.Error()))
		}
	} else {
		lines = append(lines, "  System: Normal operation")
	}
	
	lines = append(lines, fmt.Sprintf("  Status: %s", dr.message))
	lines = append(lines, fmt.Sprintf("  Time: %s", time.Now().Format("2006-01-02 15:04:05")))
	
	lines = append(lines, "")
	lines = append(lines, "Commands: h=help, q=quit, tab=next view")
	
	return strings.Join(lines, "\n")
}

// RenderSessionList 渲染降级会话列表
func (dr *DegradedRenderer) RenderSessionList(sessions []*sessions.Session, mode DegradedMode) string {
	if len(sessions) == 0 {
		return "No sessions available"
	}
	
	var lines []string
	
	switch mode {
	case DegradedModeMinimal:
		lines = append(lines, "Sessions:")
		for i, session := range sessions {
			if i >= 5 { // 限制显示数量
				break
			}
			status := "inactive"
			if session.IsActive {
				status = "active"
			}
			lines = append(lines, fmt.Sprintf("- %s (%s)", 
				session.ID, status))
		}
		
	case DegradedModeText, DegradedModeBasic:
		lines = append(lines, "Session List")
		lines = append(lines, strings.Repeat("-", 12))
		lines = append(lines, "")
		
		for i, session := range sessions {
			if i >= 10 {
				lines = append(lines, fmt.Sprintf("... and %d more sessions", len(sessions)-10))
				break
			}
			
			status := "●"
			if session.IsActive {
				status = "○"
			}
			
			startTime := session.StartTime.Format("15:04")
			duration := formatDuration(session.EndTime.Sub(session.StartTime))
			
			lines = append(lines, fmt.Sprintf("%s %-20s %s (%s)", 
				status, truncateString(session.ID, 20), startTime, duration))
		}
		
	case DegradedModeSafe:
		lines = append(lines, "Active Sessions:")
		for _, session := range sessions {
			if session.IsActive {
				lines = append(lines, fmt.Sprintf("  %s - started %s", 
					session.ID, session.StartTime.Format("15:04")))
			}
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Total sessions: %d", len(sessions)))
	}
	
	return strings.Join(lines, "\n")
}

// RenderErrorMessage 渲染错误信息
func (dr *DegradedRenderer) RenderErrorMessage(err error, mode DegradedMode) string {
	var lines []string
	
	switch mode {
	case DegradedModeMinimal:
		lines = append(lines, "Error occurred")
		lines = append(lines, err.Error())
		
	case DegradedModeText, DegradedModeBasic:
		lines = append(lines, "╭─ ERROR ─╮")
		lines = append(lines, "│         │")
		
		errMsg := err.Error()
		maxWidth := 50
		words := strings.Fields(errMsg)
		currentLine := ""
		
		for _, word := range words {
			if len(currentLine)+len(word)+1 <= maxWidth {
				if currentLine != "" {
					currentLine += " "
				}
				currentLine += word
			} else {
				if currentLine != "" {
					lines = append(lines, fmt.Sprintf("│ %-*s │", maxWidth, currentLine))
					currentLine = word
				} else {
					lines = append(lines, fmt.Sprintf("│ %-*s │", maxWidth, word))
				}
			}
		}
		
		if currentLine != "" {
			lines = append(lines, fmt.Sprintf("│ %-*s │", maxWidth, currentLine))
		}
		
		lines = append(lines, "│         │")
		lines = append(lines, "╰─────────╯")
		
	case DegradedModeSafe:
		lines = append(lines, "An error occurred:")
		lines = append(lines, err.Error())
		lines = append(lines, "")
		lines = append(lines, "The system will attempt to continue in safe mode.")
	}
	
	return strings.Join(lines, "\n")
}

// renderSimpleProgressBar 渲染简单进度条
func (dr *DegradedRenderer) renderSimpleProgressBar(label string, percentage float64, width int) string {
	if width <= 0 {
		width = 20
	}
	
	filled := int(percentage * float64(width) / 100.0)
	if filled > width {
		filled = width
	}
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s: [%s] %.1f%%", label, bar, percentage)
}

// 辅助函数
func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	
	padding := (width - len(text)) / 2
	leftPad := strings.Repeat(" ", padding)
	rightPad := strings.Repeat(" ", width-len(text)-padding)
	
	return leftPad + text + rightPad
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

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}

// DegradedModel 降级模式下的模型
type DegradedModel struct {
	renderer *DegradedRenderer
	mode     DegradedMode
	stats    Statistics
	sessions []*sessions.Session
	error    error
	width    int
	height   int
}

// NewDegradedModel 创建降级模型
func NewDegradedModel(mode DegradedMode) *DegradedModel {
	return &DegradedModel{
		renderer: NewDegradedRenderer(),
		mode:     mode,
		width:    80,
		height:   24,
	}
}

// Init 初始化降级模型
func (dm DegradedModel) Init() tea.Cmd {
	return nil
}

// Update 更新降级模型
func (dm DegradedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		dm.width = msg.Width
		dm.height = msg.Height
		dm.renderer.Resize(msg.Width, msg.Height)
		
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return dm, tea.Quit
		}
	}
	
	return dm, nil
}

// View 渲染降级视图
func (dm DegradedModel) View() string {
	return dm.renderer.RenderDashboard(dm.stats, dm.mode)
}

// SetStats 设置统计信息
func (dm *DegradedModel) SetStats(stats Statistics) {
	dm.stats = stats
}

// SetSessions 设置会话信息
func (dm *DegradedModel) SetSessions(sessions []*sessions.Session) {
	dm.sessions = sessions
}

// SetError 设置错误信息
func (dm *DegradedModel) SetError(err error) {
	dm.error = err
	dm.renderer.SetError(err)
}