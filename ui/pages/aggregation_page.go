package pages

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/ui/controllers"
)

// AggregationPage 聚合分析页面
type AggregationPage struct {
	controller *controllers.AggregationController
	width      int
	height     int
	styles     AggregationPageStyles
	help       HelpView
}

// AggregationPageStyles 页面样式
type AggregationPageStyles struct {
	HeaderStyle  lipgloss.Style
	StatusStyle  lipgloss.Style
	ContentStyle lipgloss.Style
	HelpStyle    lipgloss.Style
}

// HelpView 帮助视图
type HelpView struct {
	visible bool
	content string
}

// NewAggregationPage 创建聚合分析页面
func NewAggregationPage(entries []models.UsageEntry, cfg *config.Config) *AggregationPage {
	controller := controllers.NewAggregationController(entries, cfg)

	return &AggregationPage{
		controller: controller,
		styles:     DefaultAggregationPageStyles(),
		help: HelpView{
			content: buildHelpContent(),
		},
	}
}

// DefaultAggregationPageStyles 默认页面样式
func DefaultAggregationPageStyles() AggregationPageStyles {
	return AggregationPageStyles{
		HeaderStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5C7CFA")).
			Padding(0, 1),
		StatusStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A5A5A5")).
			Italic(true),
		ContentStyle: lipgloss.NewStyle().
			Padding(1, 0),
		HelpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#626262")).
			Padding(1, 2),
	}
}

// Init 初始化页面
func (ap *AggregationPage) Init() tea.Cmd {
	return ap.controller.Init()
}

// Update 更新页面状态
func (ap *AggregationPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "?":
			ap.help.visible = !ap.help.visible
			return ap, nil
		case "q", "esc":
			if ap.help.visible {
				ap.help.visible = false
				return ap, nil
			}
			// 其他情况由父级处理
		}

	case tea.WindowSizeMsg:
		ap.width = msg.Width
		ap.height = msg.Height

		// 为页面组件预留空间
		contentHeight := ap.height - 4 // 头部和状态栏
		if ap.help.visible {
			contentHeight -= 10 // 帮助内容
		}

		// 调用控制器的resize处理
		_, cmd := ap.controller.Update(tea.WindowSizeMsg{
			Width:  ap.width,
			Height: contentHeight,
		})
		return ap, cmd
	}

	// 转发到控制器
	model, cmd := ap.controller.Update(msg)
	if controller, ok := model.(*controllers.AggregationController); ok {
		ap.controller = controller
	}

	return ap, cmd
}

// View 渲染页面
func (ap *AggregationPage) View() string {
	if ap.width == 0 || ap.height == 0 {
		return "Loading aggregation page..."
	}

	// 页面头部
	header := ap.renderHeader()

	// 主要内容
	content := ap.controller.View()

	// 状态栏
	status := ap.renderStatus()

	// 组合页面内容
	pageContent := []string{header}

	if ap.help.visible {
		pageContent = append(pageContent, ap.renderHelp())
	} else {
		pageContent = append(pageContent, ap.styles.ContentStyle.Render(content))
	}

	pageContent = append(pageContent, status)

	return strings.Join(pageContent, "\n")
}

// renderHeader 渲染页面头部
func (ap *AggregationPage) renderHeader() string {
	title := "📊 Usage Analytics & Aggregation"
	viewName := ap.controller.GetCurrentViewName()

	headerText := fmt.Sprintf("%s - %s View", title, viewName)

	return ap.styles.HeaderStyle.
		Width(ap.width - 2).
		Render(headerText)
}

// renderStatus 渲染状态栏
func (ap *AggregationPage) renderStatus() string {
	statusText := ap.controller.GetStatusLine()
	helpHint := "Press 'h' for help"

	fullStatus := fmt.Sprintf("%s | %s", statusText, helpHint)

	return ap.styles.StatusStyle.
		Width(ap.width - 2).
		Render(fullStatus)
}

// renderHelp 渲染帮助内容
func (ap *AggregationPage) renderHelp() string {
	return ap.styles.HelpStyle.
		Width(ap.width - 6).
		Render(ap.help.content)
}

// UpdateEntries 更新数据条目
func (ap *AggregationPage) UpdateEntries(entries []models.UsageEntry) tea.Cmd {
	return ap.controller.UpdateEntries(entries)
}

// SetTimeRange 设置时间范围
func (ap *AggregationPage) SetTimeRange(start, end time.Time) tea.Cmd {
	return ap.controller.SetTimeRange(start, end)
}

// ExportData 导出数据
func (ap *AggregationPage) ExportData() (interface{}, error) {
	return ap.controller.ExportData()
}

// GetCurrentViewName 获取当前视图名称
func (ap *AggregationPage) GetCurrentViewName() string {
	return ap.controller.GetCurrentViewName()
}

// buildHelpContent 构建帮助内容
func buildHelpContent() string {
	help := []string{
		"📊 Usage Analytics & Aggregation - Help",
		"",
		"Navigation:",
		"  Tab / →     Switch to next view (Daily → Weekly → Monthly)",
		"  Shift+Tab / ← Switch to previous view",
		"  1           Switch to Daily view",
		"  2           Switch to Weekly view",
		"  3           Switch to Monthly view",
		"",
		"Actions:",
		"  r           Refresh data",
		"  h / ?       Toggle this help",
		"  q / Esc     Exit help or go back",
		"",
		"View Information:",
		"  • Daily View: Shows usage aggregated by day",
		"  • Weekly View: Shows usage aggregated by week",
		"  • Monthly View: Shows usage aggregated by month",
		"",
		"Data Columns:",
		"  • Tokens: Total tokens used in the period",
		"  • Cost: Total cost in USD for the period",
		"  • Entries: Number of usage entries in the period",
		"  • Trend: ↗ (up) ↘ (down) trend indicators",
		"",
		"Features:",
		"  • Automatic refresh every 5 minutes",
		"  • Trend analysis and pattern detection",
		"  • Model usage distribution",
		"  • Anomaly detection for unusual usage",
		"  • Caching for improved performance",
	}

	return strings.Join(help, "\n")
}

// GetCurrentData 获取当前视图数据
func (ap *AggregationPage) GetCurrentData() interface{} {
	return ap.controller.GetCurrentData()
}

// IsHelpVisible 检查帮助是否可见
func (ap *AggregationPage) IsHelpVisible() bool {
	return ap.help.visible
}

// SetHelpVisible 设置帮助可见性
func (ap *AggregationPage) SetHelpVisible(visible bool) {
	ap.help.visible = visible
}
