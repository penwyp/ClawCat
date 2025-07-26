package sessions

import (
	"sync"
	"time"

	"github.com/penwyp/claudecat/calculations"
	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/models"
)

// SessionWithMetrics 带实时指标的会话
type SessionWithMetrics struct {
	*Session
	calculator *calculations.MetricsCalculator
	metrics    *calculations.RealtimeMetrics
	mu         sync.RWMutex
}

// RealtimeManager 增强的会话管理器，支持实时指标
type RealtimeManager struct {
	*Manager
	config           *config.Config
	activeCalculator *calculations.MetricsCalculator
	currentSession   *SessionWithMetrics
	mu               sync.RWMutex
}

// NewRealtimeManager 创建支持实时指标的会话管理器
func NewRealtimeManager(cfg *config.Config) *RealtimeManager {
	return &RealtimeManager{
		Manager: NewManager(),
		config:  cfg,
	}
}

// NewSessionWithMetrics 创建带实时指标的会话
func NewSessionWithMetrics(session *Session, cfg *config.Config) *SessionWithMetrics {
	calculator := calculations.NewMetricsCalculator(session.StartTime, cfg)

	// 为现有条目初始化计算器
	for _, entry := range session.Entries {
		calculator.UpdateWithNewEntry(entry)
	}

	return &SessionWithMetrics{
		Session:    session,
		calculator: calculator,
	}
}

// UpdateMetrics 更新会话指标
func (s *SessionWithMetrics) UpdateMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = s.calculator.Calculate()
}

// AddEntry 添加新条目并更新指标
func (s *SessionWithMetrics) AddEntry(entry models.UsageEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新基础会话
	s.Session.Entries = append(s.Session.Entries, entry)
	s.Session.LastUpdate = time.Now()

	// 更新计算器
	s.calculator.UpdateWithNewEntry(entry)

	// 立即更新指标
	s.metrics = s.calculator.Calculate()

	// 更新会话统计
	s.updateSessionStats()
}

// GetCurrentMetrics 获取当前指标
func (s *SessionWithMetrics) GetCurrentMetrics() *calculations.RealtimeMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metrics == nil {
		s.mu.RUnlock()
		s.UpdateMetrics()
		s.mu.RLock()
	}
	return s.metrics
}

// updateSessionStats 更新会话基础统计信息
func (s *SessionWithMetrics) updateSessionStats() {
	if s.metrics == nil {
		return
	}

	s.Session.Stats.TotalTokens = s.metrics.CurrentTokens
	s.Session.Stats.TotalCost = s.metrics.CurrentCost
	s.Session.Stats.TimeRemaining = s.metrics.TimeRemaining
	s.Session.Stats.PercentageUsed = s.metrics.SessionProgress

	// 更新模型分布统计
	if s.Session.Stats.ModelBreakdown == nil {
		s.Session.Stats.ModelBreakdown = make(map[string]calculations.ModelStats)
	}

	for model, modelMetrics := range s.metrics.ModelDistribution {
		s.Session.Stats.ModelBreakdown[model] = calculations.ModelStats{
			Model:       model,
			TotalTokens: modelMetrics.TokenCount,
			TotalCost:   modelMetrics.Cost,
			Percentage:  modelMetrics.Percentage,
			EntryCount:  1, // Will be updated properly in full implementation
		}
	}
}

// StartNewSession 开始新的实时会话
func (rm *RealtimeManager) StartNewSession() *SessionWithMetrics {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 创建新的基础会话
	now := time.Now()
	baseSession := &Session{
		ID:         generateSessionID(now),
		StartTime:  now,
		EndTime:    now.Add(SessionDuration),
		IsActive:   true,
		Entries:    make([]models.UsageEntry, 0),
		Stats:      SessionStats{ModelBreakdown: make(map[string]calculations.ModelStats)},
		LastUpdate: now,
	}

	// 创建带指标的会话
	sessionWithMetrics := NewSessionWithMetrics(baseSession, rm.config)

	// 添加到管理器
	rm.Manager.sessions[baseSession.ID] = baseSession
	rm.Manager.activeSessions = append(rm.Manager.activeSessions, baseSession)

	// 设置为当前活动会话
	rm.currentSession = sessionWithMetrics
	rm.activeCalculator = sessionWithMetrics.calculator

	return sessionWithMetrics
}

// GetCurrentSession 获取当前活动会话
func (rm *RealtimeManager) GetCurrentSession() *SessionWithMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.currentSession
}

// AddEntryToCurrentSession 向当前会话添加条目
func (rm *RealtimeManager) AddEntryToCurrentSession(entry models.UsageEntry) error {
	rm.mu.RLock()
	session := rm.currentSession
	rm.mu.RUnlock()

	if session == nil {
		// 自动创建新会话
		session = rm.StartNewSession()
	}

	// 检查会话是否过期
	if time.Since(session.StartTime) > SessionDuration {
		// 结束当前会话并开始新会话
		rm.EndCurrentSession()
		session = rm.StartNewSession()
	}

	session.AddEntry(entry)
	return nil
}

// EndCurrentSession 结束当前会话
func (rm *RealtimeManager) EndCurrentSession() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.currentSession != nil {
		rm.currentSession.Session.IsActive = false
		rm.currentSession.Session.EndTime = time.Now()
		rm.currentSession = nil
		rm.activeCalculator = nil
	}
}

// GetCurrentMetrics 获取当前会话的实时指标
func (rm *RealtimeManager) GetCurrentMetrics() *calculations.RealtimeMetrics {
	rm.mu.RLock()
	session := rm.currentSession
	rm.mu.RUnlock()

	if session == nil {
		return &calculations.RealtimeMetrics{
			ModelDistribution: make(map[string]calculations.ModelMetrics),
		}
	}

	return session.GetCurrentMetrics()
}

// GetAllSessionsWithMetrics 获取所有会话的实时指标
func (rm *RealtimeManager) GetAllSessionsWithMetrics() []*SessionWithMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var sessions []*SessionWithMetrics
	for _, session := range rm.Manager.sessions {
		sessionWithMetrics := NewSessionWithMetrics(session, rm.config)
		sessions = append(sessions, sessionWithMetrics)
	}

	return sessions
}

// GetHistoricalMetrics 获取历史指标数据
func (rm *RealtimeManager) GetHistoricalMetrics(duration time.Duration) []calculations.RealtimeMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.activeCalculator == nil {
		return []calculations.RealtimeMetrics{}
	}

	// 这里可以扩展为返回历史快照
	current := rm.activeCalculator.Calculate()
	return []calculations.RealtimeMetrics{*current}
}

// GetBurnRateHistory 获取燃烧率历史
func (rm *RealtimeManager) GetBurnRateHistory(intervals []time.Duration) map[time.Duration]float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make(map[time.Duration]float64)
	if rm.activeCalculator == nil {
		return result
	}

	for _, interval := range intervals {
		result[interval] = rm.activeCalculator.GetBurnRate(interval)
	}

	return result
}

// IsLimitApproaching 检查是否接近使用限制
func (rm *RealtimeManager) IsLimitApproaching() (bool, float64, string) {
	metrics := rm.GetCurrentMetrics()
	if metrics == nil {
		return false, 0, ""
	}

	limit := rm.getPlanLimit()
	if limit <= 0 {
		return false, 0, "No limit configured"
	}

	currentUsage := metrics.CurrentCost
	percentage := (currentUsage / limit) * 100

	// 检查不同的警告级别
	warnThreshold := rm.config.Subscription.WarnThreshold
	if warnThreshold == 0 {
		warnThreshold = 75.0 // 默认75%警告
	}

	if percentage >= warnThreshold {
		return true, percentage, "approaching_limit"
	}

	return false, percentage, "normal"
}

// getPlanLimit 获取订阅计划限额
func (rm *RealtimeManager) getPlanLimit() float64 {
	switch rm.config.Subscription.Plan {
	case "pro":
		return 18.00
	case "max5":
		return 35.00
	case "max20":
		return 140.00
	case "custom":
		return rm.config.Subscription.CustomCostLimit
	default:
		return 0
	}
}

// generateSessionID 生成会话ID
func generateSessionID(startTime time.Time) string {
	return startTime.Format("20060102-150405")
}

// Cleanup 清理过期数据
func (rm *RealtimeManager) Cleanup() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour) // 保留24小时的历史数据

	// 清理过期会话
	for id, session := range rm.Manager.sessions {
		if session.EndTime.Before(cutoff) && !session.IsActive {
			delete(rm.Manager.sessions, id)
		}
	}

	// 更新活动会话列表
	activeSessions := make([]*Session, 0)
	for _, session := range rm.Manager.activeSessions {
		if session.IsActive {
			activeSessions = append(activeSessions, session)
		}
	}
	rm.Manager.activeSessions = activeSessions
}
