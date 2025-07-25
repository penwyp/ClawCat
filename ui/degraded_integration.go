package ui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/penwyp/ClawCat/errors"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
)

// DegradedApp 降级模式应用
type DegradedApp struct {
	model    *DegradedModel
	program  *tea.Program
	config   Config
	ctx      context.Context
	cancel   context.CancelFunc
	mode     DegradedMode
	errorHandler *errors.ErrorHandler
	mu       sync.RWMutex
}

// DegradedConfig 降级配置
type DegradedConfig struct {
	AutoDetectMode  bool          // 自动检测降级模式
	FallbackTimeout time.Duration // 降级超时时间
	MaxRetries      int           // 最大重试次数
	SafeMode        bool          // 强制安全模式
}

// NewDegradedApp 创建降级应用
func NewDegradedApp(cfg Config, degradedCfg DegradedConfig, errorHandler *errors.ErrorHandler) *DegradedApp {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 根据配置选择降级模式
	mode := DegradedModeBasic
	if degradedCfg.SafeMode {
		mode = DegradedModeSafe
	} else if degradedCfg.AutoDetectMode {
		mode = detectDegradedMode()
	}
	
	model := NewDegradedModel(mode)
	
	app := &DegradedApp{
		model:        model,
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
		mode:         mode,
		errorHandler: errorHandler,
	}
	
	// 创建 Bubble Tea 程序（简化配置）
	app.program = tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)
	
	return app
}

// Start 启动降级应用
func (da *DegradedApp) Start() error {
	_, err := da.program.Run()
	return err
}

// Stop 停止降级应用
func (da *DegradedApp) Stop() error {
	if da.program != nil {
		da.program.Kill()
	}
	if da.cancel != nil {
		da.cancel()
	}
	return nil
}

// UpdateData 更新数据（简化版本）
func (da *DegradedApp) UpdateData(sessions []*sessions.Session, entries []models.UsageEntry) {
	da.mu.Lock()
	defer da.mu.Unlock()
	
	if da.model != nil {
		da.model.SetSessions(sessions)
		
		// 计算简化统计信息
		stats := da.calculateSimpleStats(sessions, entries)
		da.model.SetStats(stats)
	}
}

// SetError 设置错误状态
func (da *DegradedApp) SetError(err error) {
	da.mu.Lock()
	defer da.mu.Unlock()
	
	if da.model != nil {
		da.model.SetError(err)
	}
}

// SwitchMode 切换降级模式
func (da *DegradedApp) SwitchMode(mode DegradedMode) {
	da.mu.Lock()
	defer da.mu.Unlock()
	
	da.mode = mode
	if da.model != nil {
		da.model.mode = mode
	}
}

// calculateSimpleStats 计算简化统计信息
func (da *DegradedApp) calculateSimpleStats(sessions []*sessions.Session, entries []models.UsageEntry) Statistics {
	stats := Statistics{}
	
	// 基础计算，避免复杂操作
	stats.SessionCount = len(sessions)
	
	var totalTokens int64
	var totalCost float64
	
	for _, session := range sessions {
		if session.IsActive {
			stats.ActiveSessions++
		}
		
		// 简化的条目处理
		for _, entry := range session.Entries {
			totalTokens += int64(entry.TotalTokens)
			totalCost += entry.CostUSD
		}
		
		if stats.LastSession == nil || session.StartTime.After(stats.LastSession.StartTime) {
			stats.LastSession = session
		}
	}
	
	stats.TotalTokens = totalTokens
	stats.TotalCost = totalCost
	
	if stats.SessionCount > 0 {
		stats.AverageCost = totalCost / float64(stats.SessionCount)
	}
	
	// 简化的时间计算
	now := time.Now()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	stats.TimeToReset = nextMonth.Sub(now)
	
	// 简化的使用率计算
	stats.PlanUsage = (totalCost / 100.0) * 100 // 假设 $100 计划
	if stats.PlanUsage > 100 {
		stats.PlanUsage = 100
	}
	
	return stats
}

// detectDegradedMode 检测合适的降级模式
func detectDegradedMode() DegradedMode {
	// 简单的终端能力检测
	// 在实际实现中可以检测终端支持的功能
	
	// 检查终端宽度
	// 如果宽度太小，使用最小模式
	// 这里简化为返回基础模式
	return DegradedModeBasic
}

// AppWithFallback 带降级功能的应用包装器
type AppWithFallback struct {
	primaryApp   *App
	degradedApp  *DegradedApp
	errorHandler *errors.ErrorHandler
	isDegraded   bool
	config       Config
	degradedCfg  DegradedConfig
	mu           sync.RWMutex
}

// NewAppWithFallback 创建带降级功能的应用
func NewAppWithFallback(cfg Config, degradedCfg DegradedConfig, errorHandler *errors.ErrorHandler) *AppWithFallback {
	primaryApp := NewApp(cfg)
	degradedApp := NewDegradedApp(cfg, degradedCfg, errorHandler)
	
	return &AppWithFallback{
		primaryApp:   primaryApp,
		degradedApp:  degradedApp,
		errorHandler: errorHandler,
		config:       cfg,
		degradedCfg:  degradedCfg,
	}
}

// Start 启动应用（优先使用主应用）
func (awf *AppWithFallback) Start() error {
	awf.mu.Lock()
	defer awf.mu.Unlock()
	
	if awf.isDegraded {
		return awf.degradedApp.Start()
	}
	
	// 尝试启动主应用
	err := awf.primaryApp.Start()
	if err != nil {
		// 主应用启动失败，切换到降级模式
		awf.enterDegradedMode(err)
		return awf.degradedApp.Start()
	}
	
	return nil
}

// Stop 停止应用
func (awf *AppWithFallback) Stop() error {
	awf.mu.Lock()
	defer awf.mu.Unlock()
	
	var err error
	if awf.isDegraded {
		err = awf.degradedApp.Stop()
	} else {
		err = awf.primaryApp.Stop()
	}
	
	return err
}

// enterDegradedMode 进入降级模式
func (awf *AppWithFallback) enterDegradedMode(err error) {
	awf.isDegraded = true
	
	// 设置错误状态
	awf.degradedApp.SetError(err)
	
	// 记录错误
	if awf.errorHandler != nil {
		context := &errors.ErrorContext{
			Component: "UI",
			Operation: "app_start",
			TraceID:   "ui-degraded",
		}
		awf.errorHandler.Handle(err, context)
	}
}

// exitDegradedMode 退出降级模式
func (awf *AppWithFallback) exitDegradedMode() error {
	awf.mu.Lock()
	defer awf.mu.Unlock()
	
	if !awf.isDegraded {
		return nil
	}
	
	// 停止降级应用
	if err := awf.degradedApp.Stop(); err != nil {
		return err
	}
	
	// 尝试重新启动主应用
	awf.primaryApp = NewApp(awf.config)
	
	awf.isDegraded = false
	return nil
}

// UpdateData 更新数据
func (awf *AppWithFallback) UpdateData(sessions []*sessions.Session, entries []models.UsageEntry) {
	awf.mu.RLock()
	defer awf.mu.RUnlock()
	
	if awf.isDegraded {
		awf.degradedApp.UpdateData(sessions, entries)
	} else {
		awf.primaryApp.UpdateData(sessions, entries)
	}
}

// SetDataSource 设置数据源
func (awf *AppWithFallback) SetDataSource(manager *sessions.Manager) {
	awf.mu.RLock()
	defer awf.mu.RUnlock()
	
	if !awf.isDegraded {
		awf.primaryApp.SetDataSource(manager)
	}
}

// IsRunning 检查是否运行中
func (awf *AppWithFallback) IsRunning() bool {
	awf.mu.RLock()
	defer awf.mu.RUnlock()
	
	if awf.isDegraded {
		return awf.degradedApp.ctx.Err() == nil
	}
	return awf.primaryApp.IsRunning()
}

// IsDegraded 检查是否处于降级模式
func (awf *AppWithFallback) IsDegraded() bool {
	awf.mu.RLock()
	defer awf.mu.RUnlock()
	return awf.isDegraded
}

// TryRecover 尝试恢复到主应用
func (awf *AppWithFallback) TryRecover() error {
	if !awf.IsDegraded() {
		return nil
	}
	
	return awf.exitDegradedMode()
}

// GetCurrentApp 获取当前活动的应用
func (awf *AppWithFallback) GetCurrentApp() interface{} {
	awf.mu.RLock()
	defer awf.mu.RUnlock()
	
	if awf.isDegraded {
		return awf.degradedApp
	}
	return awf.primaryApp
}