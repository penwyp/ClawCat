package limits

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
)

// LimitManager 限额管理器
type LimitManager struct {
	mu             sync.RWMutex
	plan           SubscriptionPlan
	usage          Usage
	history        []HistoricalUsage
	notifier       *Notifier
	config         *config.Config
	warningHistory map[Severity]time.Time
	p90Calculator  *P90Calculator
}

// NewLimitManager 创建限额管理器
func NewLimitManager(cfg *config.Config) (*LimitManager, error) {
	planType := PlanType(cfg.Subscription.Plan)
	
	// 处理自定义计划
	if planType == PlanCustom {
		customLimit := cfg.Subscription.CustomCostLimit
		if customLimit <= 0 {
			return nil, fmt.Errorf("custom plan requires a positive cost limit")
		}
		plan := CreateCustomPlan(customLimit)
		lm := &LimitManager{
			plan:           plan,
			config:         cfg,
			notifier:       NewNotifier(cfg),
			warningHistory: make(map[Severity]time.Time),
			p90Calculator:  NewP90Calculator(),
			usage: Usage{
				StartTime: time.Now(),
			},
		}
		
		// 加载历史使用数据
		if err := lm.loadHistoricalUsage(); err != nil {
			// 非致命错误，记录日志但继续
			fmt.Printf("Warning: Failed to load historical usage: %v\n", err)
		}
		
		return lm, nil
	}
	
	// 处理标准计划
	plan, ok := GetPlanByType(planType)
	if !ok {
		return nil, fmt.Errorf("unknown plan type: %s", planType)
	}

	lm := &LimitManager{
		plan:           plan,
		config:         cfg,
		notifier:       NewNotifier(cfg),
		warningHistory: make(map[Severity]time.Time),
		p90Calculator:  NewP90Calculator(),
		usage: Usage{
			StartTime: time.Now(),
		},
	}

	// 加载历史使用数据
	if err := lm.loadHistoricalUsage(); err != nil {
		// 非致命错误，记录日志但继续
		fmt.Printf("Warning: Failed to load historical usage: %v\n", err)
	}

	return lm, nil
}

// CheckUsage 检查使用情况
func (lm *LimitManager) CheckUsage(entry models.UsageEntry) (*LimitStatus, error) {
	lm.mu.Lock()
	
	// 更新使用量
	lm.usage.Tokens += int64(entry.TotalTokens)
	lm.usage.Cost += entry.CostUSD
	lm.usage.LastUpdate = time.Now()

	// 计算使用百分比
	percentage := 0.0
	if lm.plan.CostLimit > 0 {
		percentage = (lm.usage.Cost / lm.plan.CostLimit) * 100
	}

	// 获取当前状态
	status := &LimitStatus{
		Plan:         lm.plan,
		CurrentUsage: lm.usage,
		Percentage:   percentage,
		TimeToReset:  lm.calculateTimeToReset(),
	}

	// 检查警告级别
	var currentWarningLevel *WarningLevel
	for i := len(lm.plan.WarningLevels) - 1; i >= 0; i-- {
		level := lm.plan.WarningLevels[i]
		if percentage >= level.Threshold {
			currentWarningLevel = &level
			break
		}
	}

	status.WarningLevel = currentWarningLevel
	
	// 在释放锁之前检查是否应该触发警告
	shouldTrigger := false
	if currentWarningLevel != nil {
		// 内部检查，不需要额外的锁
		lastTriggered, exists := lm.warningHistory[currentWarningLevel.Severity]
		if !exists {
			shouldTrigger = true
		} else {
			// 避免频繁警告，每个级别至少间隔一定时间
			cooldown := time.Hour
			switch currentWarningLevel.Severity {
			case SeverityCritical:
				cooldown = 15 * time.Minute
			case SeverityError:
				cooldown = 30 * time.Minute
			case SeverityWarning:
				cooldown = time.Hour
			case SeverityInfo:
				cooldown = 2 * time.Hour
			}
			shouldTrigger = time.Since(lastTriggered) > cooldown
		}
	}
	
	lm.mu.Unlock()

	// 触发警告动作（在锁外执行）
	if currentWarningLevel != nil && shouldTrigger {
		go lm.triggerWarningActions(*currentWarningLevel, status)
	}

	// 生成建议
	status.Recommendations = lm.GetRecommendations()

	return status, nil
}

// UpdateUsage 更新使用量
func (lm *LimitManager) UpdateUsage(tokens int64, cost float64) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.usage.Tokens += tokens
	lm.usage.Cost += cost
	lm.usage.LastUpdate = time.Now()

	return nil
}

// GetStatus 获取当前状态
func (lm *LimitManager) GetStatus() *LimitStatus {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	percentage := 0.0
	if lm.plan.CostLimit > 0 {
		percentage = (lm.usage.Cost / lm.plan.CostLimit) * 100
	}

	status := &LimitStatus{
		Plan:         lm.plan,
		CurrentUsage: lm.usage,
		Percentage:   percentage,
		TimeToReset:  lm.calculateTimeToReset(),
	}

	// 确定当前警告级别
	for i := len(lm.plan.WarningLevels) - 1; i >= 0; i-- {
		level := lm.plan.WarningLevels[i]
		if percentage >= level.Threshold {
			status.WarningLevel = &level
			break
		}
	}

	status.Recommendations = lm.GetRecommendations()

	return status
}

// SetPlan 设置订阅计划
func (lm *LimitManager) SetPlan(planType PlanType) error {
	plan, ok := GetPlanByType(planType)
	if !ok {
		return fmt.Errorf("unknown plan type: %s", planType)
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.plan = plan
	// 清空警告历史，因为阈值可能已更改
	lm.warningHistory = make(map[Severity]time.Time)

	return nil
}

// ResetUsage 重置使用量
func (lm *LimitManager) ResetUsage() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// 保存当前使用量到历史记录
	if lm.usage.Cost > 0 || lm.usage.Tokens > 0 {
		historical := HistoricalUsage{
			Date:   time.Now(),
			Tokens: lm.usage.Tokens,
			Cost:   lm.usage.Cost,
		}
		lm.history = append(lm.history, historical)

		// 限制历史记录数量（保留最近365天）
		if len(lm.history) > 365 {
			lm.history = lm.history[len(lm.history)-365:]
		}
	}

	// 重置当前使用量
	lm.usage = Usage{
		StartTime: time.Now(),
	}

	// 清空警告历史
	lm.warningHistory = make(map[Severity]time.Time)

	return nil
}

// GetRecommendations 获取使用建议
func (lm *LimitManager) GetRecommendations() []string {
	recommendations := []string{}
	percentage := 0.0
	if lm.plan.CostLimit > 0 {
		percentage = (lm.usage.Cost / lm.plan.CostLimit) * 100
	}

	if percentage > 95 {
		recommendations = append(recommendations,
			"🚨 Critical: Consider upgrading your plan immediately",
			"💡 Review your usage patterns to identify optimization opportunities",
			"⚡ Use caching strategies to reduce token consumption",
		)
	} else if percentage > 90 {
		recommendations = append(recommendations,
			"⚠️ Consider upgrading to a higher plan",
			"📊 Review your token usage patterns",
			"💡 Use caching to reduce token consumption",
		)
	} else if percentage > 75 {
		recommendations = append(recommendations,
			"👀 Monitor your usage closely",
			"📅 Plan your remaining tasks carefully",
		)
	} else if percentage > 50 {
		recommendations = append(recommendations,
			"📈 You're on track with your usage",
		)
	}

	// 基于使用模式的建议
	if lm.hasHighBurnRate() {
		recommendations = append(recommendations,
			"🔥 Your burn rate is high - consider spreading usage over time",
		)
	}

	if lm.hasFrequentSpikes() {
		recommendations = append(recommendations,
			"📊 Frequent usage spikes detected - consider batch processing",
		)
	}

	return recommendations
}


// triggerWarningActions 触发警告动作
func (lm *LimitManager) triggerWarningActions(level WarningLevel, status *LimitStatus) {
	lm.mu.Lock()
	lm.warningHistory[level.Severity] = time.Now()
	lm.mu.Unlock()

	for _, action := range level.Actions {
		switch action.Type {
		case ActionNotify:
			if err := lm.notifier.SendNotification(level.Message, level.Severity); err != nil {
				// 通知发送失败，记录日志但继续处理其他动作
				log.Printf("Failed to send notification: %v", err)
			}
		case ActionLog:
			lm.logWarning(level, status)
		case ActionWebhook:
			lm.sendWebhook(action.Config, status)
		case ActionThrottle:
			lm.applyThrottling(action.Config)
		case ActionBlock:
			lm.blockUsage(action.Config)
		}
	}
}

// calculateTimeToReset 计算重置时间
func (lm *LimitManager) calculateTimeToReset() time.Duration {
	now := time.Now()

	switch lm.plan.ResetCycle {
	case ResetCycleDaily:
		tomorrow := now.Add(24 * time.Hour)
		reset := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
		return reset.Sub(now)

	case ResetCycleWeekly:
		// 找到下周一
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		reset := now.AddDate(0, 0, daysUntilMonday)
		reset = time.Date(reset.Year(), reset.Month(), reset.Day(), 0, 0, 0, 0, now.Location())
		return reset.Sub(now)

	case ResetCycleMonthly:
		// 下月1日
		nextMonth := now.AddDate(0, 1, 0)
		reset := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, now.Location())
		return reset.Sub(now)

	default:
		return 0
	}
}

// loadHistoricalUsage 加载历史使用数据
func (lm *LimitManager) loadHistoricalUsage() error {
	// TODO: 从文件或数据库加载历史数据
	// 现在先返回空数据，实际实现时可以从缓存文件读取
	lm.history = []HistoricalUsage{}
	return nil
}

// hasHighBurnRate 检查是否有高燃烧率
func (lm *LimitManager) hasHighBurnRate() bool {
	if lm.usage.Cost == 0 {
		return false
	}

	// 计算当前燃烧率
	elapsed := time.Since(lm.usage.StartTime)
	if elapsed == 0 {
		return false
	}

	currentRate := lm.usage.Cost / elapsed.Hours()
	
	// 如果每小时消耗超过计划限额的10%，认为是高燃烧率
	threshold := lm.plan.CostLimit * 0.1
	return currentRate > threshold
}

// hasFrequentSpikes 检查是否有频繁使用峰值
func (lm *LimitManager) hasFrequentSpikes() bool {
	// 简化实现：如果历史数据中有多个高使用天数，认为有频繁峰值
	if len(lm.history) < 7 {
		return false
	}

	// 计算最近7天的平均使用
	recentDays := lm.history[len(lm.history)-7:]
	total := 0.0
	for _, day := range recentDays {
		total += day.Cost
	}
	average := total / float64(len(recentDays))

	// 如果有超过3天的使用量超过平均值的150%，认为有频繁峰值
	spikes := 0
	for _, day := range recentDays {
		if day.Cost > average*1.5 {
			spikes++
		}
	}

	return spikes >= 3
}

// logWarning 记录警告日志
func (lm *LimitManager) logWarning(level WarningLevel, status *LimitStatus) {
	// TODO: 实现详细的日志记录
	fmt.Printf("[%s] %s - Usage: %.1f%% ($%.2f / $%.2f)\n",
		level.Severity, level.Message, status.Percentage,
		status.CurrentUsage.Cost, status.Plan.CostLimit)
}

// sendWebhook 发送 Webhook
func (lm *LimitManager) sendWebhook(config map[string]interface{}, status *LimitStatus) {
	// TODO: 实现 Webhook 发送
	fmt.Printf("Webhook triggered: %s\n", status.WarningLevel.Message)
}

// applyThrottling 应用限流
func (lm *LimitManager) applyThrottling(config map[string]interface{}) {
	// TODO: 实现使用限流
	fmt.Println("Throttling applied")
}

// blockUsage 阻止使用
func (lm *LimitManager) blockUsage(config map[string]interface{}) {
	// TODO: 实现使用阻止
	fmt.Println("Usage blocked due to limit exceeded")
}