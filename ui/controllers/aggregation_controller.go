package controllers

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/ui/components"
)

// AggregationController 聚合控制器
type AggregationController struct {
	mu                sync.RWMutex
	config            *config.Config
	aggregationEngine *calculations.AggregationEngine

	// UI 组件
	dailyView   *components.AggregationView
	weeklyView  *components.AggregationView
	monthlyView *components.AggregationView

	// 状态
	currentView     int // 0: daily, 1: weekly, 2: monthly
	isLoading       bool
	lastUpdated     time.Time
	refreshInterval time.Duration

	// 数据
	dailyData   []calculations.AggregatedData
	weeklyData  []calculations.AggregatedData
	monthlyData []calculations.AggregatedData

	// 时间范围
	timeRange struct {
		start time.Time
		end   time.Time
	}
}

// AggregationMsg 聚合消息类型
type AggregationMsg struct {
	Type string
	Data interface{}
}

// 消息类型
const (
	MsgAggregationUpdated = "aggregation_updated"
	MsgAggregationError   = "aggregation_error"
	MsgViewChanged        = "view_changed"
	MsgRefreshComplete    = "refresh_complete"
)

// NewAggregationController 创建聚合控制器
func NewAggregationController(entries []models.UsageEntry, cfg *config.Config) *AggregationController {
	// 创建聚合引擎
	engine := calculations.NewAggregationEngine(entries, cfg)

	// 创建视图组件
	dailyView := components.NewAggregationView("Daily Usage", calculations.DailyView)
	weeklyView := components.NewAggregationView("Weekly Usage", calculations.WeeklyView)
	monthlyView := components.NewAggregationView("Monthly Usage", calculations.MonthlyView)

	// 设置默认时间范围（最近30天）
	now := time.Now()
	start := now.AddDate(0, 0, -30)

	controller := &AggregationController{
		config:            cfg,
		aggregationEngine: engine,
		dailyView:         dailyView,
		weeklyView:        weeklyView,
		monthlyView:       monthlyView,
		currentView:       0,
		refreshInterval:   5 * time.Minute,
	}

	controller.timeRange.start = start
	controller.timeRange.end = now

	return controller
}

// Init 初始化控制器
func (ac *AggregationController) Init() tea.Cmd {
	return tea.Batch(
		ac.refreshData(),
		ac.startAutoRefresh(),
	)
}

// Update 更新控制器状态
func (ac *AggregationController) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ac.handleKeyPress(msg)
	case AggregationMsg:
		return ac.handleAggregationMsg(msg)
	case tea.WindowSizeMsg:
		return ac.handleResize(msg)
	}

	return ac, nil
}

// View 渲染视图
func (ac *AggregationController) View() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	if ac.isLoading {
		return "Loading aggregation data..."
	}

	// 根据当前视图渲染对应组件
	switch ac.currentView {
	case 0:
		return ac.dailyView.Render()
	case 1:
		return ac.weeklyView.Render()
	case 2:
		return ac.monthlyView.Render()
	default:
		return ac.dailyView.Render()
	}
}

// handleKeyPress 处理按键事件
func (ac *AggregationController) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "right":
		return ac.nextView()
	case "shift+tab", "left":
		return ac.prevView()
	case "r":
		return ac, ac.refreshData()
	case "1":
		return ac.switchView(0)
	case "2":
		return ac.switchView(1)
	case "3":
		return ac.switchView(2)
	}

	return ac, nil
}

// handleAggregationMsg 处理聚合消息
func (ac *AggregationController) handleAggregationMsg(msg AggregationMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case MsgAggregationUpdated:
		ac.mu.Lock()
		ac.isLoading = false
		ac.lastUpdated = time.Now()

		// 更新数据
		if data, ok := msg.Data.(map[string][]calculations.AggregatedData); ok {
			if daily, exists := data["daily"]; exists {
				ac.dailyData = daily
				ac.dailyView.SetData(daily)
			}
			if weekly, exists := data["weekly"]; exists {
				ac.weeklyData = weekly
				ac.weeklyView.SetData(weekly)
			}
			if monthly, exists := data["monthly"]; exists {
				ac.monthlyData = monthly
				ac.monthlyView.SetData(monthly)
			}
		}
		ac.mu.Unlock()

	case MsgAggregationError:
		ac.mu.Lock()
		ac.isLoading = false
		ac.mu.Unlock()
		// TODO: 处理错误显示

	case MsgRefreshComplete:
		// 刷新完成，可以进行下一次刷新
		return ac, ac.scheduleNextRefresh()
	}

	return ac, nil
}

// handleResize 处理窗口大小变化
func (ac *AggregationController) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// 计算每个视图的尺寸
	width := msg.Width
	height := msg.Height - 2 // 为状态栏预留空间

	ac.dailyView.SetSize(width, height)
	ac.weeklyView.SetSize(width, height)
	ac.monthlyView.SetSize(width, height)

	return ac, nil
}

// nextView 切换到下一个视图
func (ac *AggregationController) nextView() (tea.Model, tea.Cmd) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.currentView = (ac.currentView + 1) % 3
	ac.updateViewFocus()

	return ac, tea.Cmd(func() tea.Msg {
		return AggregationMsg{
			Type: MsgViewChanged,
			Data: ac.currentView,
		}
	})
}

// prevView 切换到上一个视图
func (ac *AggregationController) prevView() (tea.Model, tea.Cmd) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.currentView = (ac.currentView - 1 + 3) % 3
	ac.updateViewFocus()

	return ac, tea.Cmd(func() tea.Msg {
		return AggregationMsg{
			Type: MsgViewChanged,
			Data: ac.currentView,
		}
	})
}

// switchView 切换到指定视图
func (ac *AggregationController) switchView(viewIndex int) (tea.Model, tea.Cmd) {
	if viewIndex < 0 || viewIndex > 2 {
		return ac, nil
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.currentView = viewIndex
	ac.updateViewFocus()

	return ac, tea.Cmd(func() tea.Msg {
		return AggregationMsg{
			Type: MsgViewChanged,
			Data: ac.currentView,
		}
	})
}

// updateViewFocus 更新视图焦点状态
func (ac *AggregationController) updateViewFocus() {
	ac.dailyView.SetFocused(ac.currentView == 0)
	ac.weeklyView.SetFocused(ac.currentView == 1)
	ac.monthlyView.SetFocused(ac.currentView == 2)
}

// refreshData 刷新数据
func (ac *AggregationController) refreshData() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		ac.mu.Lock()
		ac.isLoading = true
		start := ac.timeRange.start
		end := ac.timeRange.end
		ac.mu.Unlock()

		// 在后台执行聚合计算
		result := make(map[string][]calculations.AggregatedData)

		// 计算日聚合
		if daily, err := ac.aggregationEngine.Aggregate(calculations.DailyView, start, end); err == nil {
			result["daily"] = daily
		}

		// 计算周聚合
		if weekly, err := ac.aggregationEngine.Aggregate(calculations.WeeklyView, start, end); err == nil {
			result["weekly"] = weekly
		}

		// 计算月聚合
		if monthly, err := ac.aggregationEngine.Aggregate(calculations.MonthlyView, start, end); err == nil {
			result["monthly"] = monthly
		}

		return AggregationMsg{
			Type: MsgAggregationUpdated,
			Data: result,
		}
	})
}

// startAutoRefresh 启动自动刷新
func (ac *AggregationController) startAutoRefresh() tea.Cmd {
	return tea.Tick(ac.refreshInterval, func(time.Time) tea.Msg {
		return AggregationMsg{
			Type: MsgRefreshComplete,
		}
	})
}

// scheduleNextRefresh 安排下次刷新
func (ac *AggregationController) scheduleNextRefresh() tea.Cmd {
	return tea.Tick(ac.refreshInterval, func(time.Time) tea.Msg {
		return AggregationMsg{
			Type: MsgRefreshComplete,
		}
	})
}

// SetTimeRange 设置时间范围
func (ac *AggregationController) SetTimeRange(start, end time.Time) tea.Cmd {
	ac.mu.Lock()
	ac.timeRange.start = start
	ac.timeRange.end = end
	ac.mu.Unlock()

	return ac.refreshData()
}

// GetCurrentViewName 获取当前视图名称
func (ac *AggregationController) GetCurrentViewName() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	switch ac.currentView {
	case 0:
		return "Daily"
	case 1:
		return "Weekly"
	case 2:
		return "Monthly"
	default:
		return "Unknown"
	}
}

// GetStatusLine 获取状态行信息
func (ac *AggregationController) GetStatusLine() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	viewName := ac.GetCurrentViewName()
	timeRange := fmt.Sprintf("%s - %s",
		ac.timeRange.start.Format("2006-01-02"),
		ac.timeRange.end.Format("2006-01-02"))

	if ac.isLoading {
		return fmt.Sprintf("%s View | %s | Loading...", viewName, timeRange)
	}

	lastUpdate := ""
	if !ac.lastUpdated.IsZero() {
		lastUpdate = fmt.Sprintf(" | Updated: %s", ac.lastUpdated.Format("15:04:05"))
	}

	return fmt.Sprintf("%s View | %s%s | Tab: Switch View | R: Refresh",
		viewName, timeRange, lastUpdate)
}

// GetCurrentData 获取当前视图的数据
func (ac *AggregationController) GetCurrentData() []calculations.AggregatedData {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	switch ac.currentView {
	case 0:
		return ac.dailyData
	case 1:
		return ac.weeklyData
	case 2:
		return ac.monthlyData
	default:
		return nil
	}
}

// UpdateEntries 更新原始数据
func (ac *AggregationController) UpdateEntries(entries []models.UsageEntry) tea.Cmd {
	ac.mu.Lock()
	ac.aggregationEngine = calculations.NewAggregationEngine(entries, ac.config)
	ac.mu.Unlock()

	return ac.refreshData()
}

// ExportData 导出数据
func (ac *AggregationController) ExportData() (map[string][]calculations.AggregatedData, error) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	return map[string][]calculations.AggregatedData{
		"daily":   ac.dailyData,
		"weekly":  ac.weeklyData,
		"monthly": ac.monthlyData,
	}, nil
}
