package limits

// 预定义订阅计划
var PlanDefinitions = map[PlanType]SubscriptionPlan{
	PlanFree: {
		Name:       "Free",
		Type:       PlanFree,
		CostLimit:  0.00,
		TokenLimit: 0,
		Features:   []string{"Basic usage tracking"},
		WarningLevels: []WarningLevel{
			{
				Threshold: 100,
				Message:   "Free plan usage exceeded",
				Severity:  SeverityInfo,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
				},
			},
		},
		ResetCycle: ResetCycleMonthly,
	},

	PlanPro: {
		Name:       "Pro",
		Type:       PlanPro,
		CostLimit:  18.00,
		TokenLimit: 1000000, // 估算值
		Features:   []string{"5-hour sessions", "All models", "Priority support"},
		WarningLevels: []WarningLevel{
			{
				Threshold: 75,
				Message:   "You've used 75% of your Pro plan limit",
				Severity:  SeverityInfo,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
				},
			},
			{
				Threshold: 90,
				Message:   "⚠️ 90% of limit reached! Consider upgrading",
				Severity:  SeverityWarning,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
					{Type: ActionWebhook, Config: nil},
				},
			},
			{
				Threshold: 95,
				Message:   "🚨 95% limit! Usage will be blocked soon",
				Severity:  SeverityError,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
					{Type: ActionWebhook, Config: nil},
				},
			},
			{
				Threshold: 100,
				Message:   "❌ Limit reached! Upgrade to continue",
				Severity:  SeverityCritical,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
					{Type: ActionWebhook, Config: nil},
					{Type: ActionBlock, Config: nil},
				},
			},
		},
		ResetCycle: ResetCycleMonthly,
	},

	PlanMax5: {
		Name:       "Max-5",
		Type:       PlanMax5,
		CostLimit:  35.00,
		TokenLimit: 2000000,
		Features:   []string{"5-hour sessions", "All models", "Priority support", "Advanced analytics"},
		WarningLevels: createDefaultWarningLevels(35.00),
		ResetCycle:    ResetCycleMonthly,
	},

	PlanMax20: {
		Name:       "Max-20",
		Type:       PlanMax20,
		CostLimit:  140.00,
		TokenLimit: 8000000,
		Features:   []string{"5-hour sessions", "All models", "Priority support", "Advanced analytics", "Team features"},
		WarningLevels: createDefaultWarningLevels(140.00),
		ResetCycle:    ResetCycleMonthly,
	},

	PlanCustom: {
		Name:        "Custom",
		Type:        PlanCustom,
		CostLimit:   0.00, // 将通过 P90 计算设置
		TokenLimit:  0,    // 将基于成本限制估算
		CustomLimit: true,
		Features:    []string{"P90-based limits", "Custom thresholds", "Advanced monitoring"},
		WarningLevels: []WarningLevel{
			{
				Threshold: 80,
				Message:   "80% of your custom limit reached",
				Severity:  SeverityInfo,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
				},
			},
			{
				Threshold: 90,
				Message:   "⚠️ 90% of custom limit reached",
				Severity:  SeverityWarning,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
					{Type: ActionWebhook, Config: nil},
				},
			},
			{
				Threshold: 100,
				Message:   "🚨 Custom limit exceeded!",
				Severity:  SeverityError,
				Actions: []Action{
					{Type: ActionNotify, Config: nil},
					{Type: ActionLog, Config: nil},
					{Type: ActionWebhook, Config: nil},
				},
			},
		},
		ResetCycle: ResetCycleMonthly,
	},
}

// createDefaultWarningLevels 创建默认警告级别
func createDefaultWarningLevels(limit float64) []WarningLevel {
	return []WarningLevel{
		{
			Threshold: 75,
			Message:   "75% of plan limit reached",
			Severity:  SeverityInfo,
			Actions: []Action{
				{Type: ActionNotify, Config: nil},
				{Type: ActionLog, Config: nil},
			},
		},
		{
			Threshold: 90,
			Message:   "⚠️ 90% of plan limit reached",
			Severity:  SeverityWarning,
			Actions: []Action{
				{Type: ActionNotify, Config: nil},
				{Type: ActionLog, Config: nil},
				{Type: ActionWebhook, Config: nil},
			},
		},
		{
			Threshold: 95,
			Message:   "🚨 95% limit reached",
			Severity:  SeverityError,
			Actions: []Action{
				{Type: ActionNotify, Config: nil},
				{Type: ActionLog, Config: nil},
				{Type: ActionWebhook, Config: nil},
			},
		},
		{
			Threshold: 100,
			Message:   "❌ Plan limit exceeded",
			Severity:  SeverityCritical,
			Actions: []Action{
				{Type: ActionNotify, Config: nil},
				{Type: ActionLog, Config: nil},
				{Type: ActionWebhook, Config: nil},
				{Type: ActionBlock, Config: nil},
			},
		},
	}
}

// GetPlanByType 根据类型获取计划
func GetPlanByType(planType PlanType) (SubscriptionPlan, bool) {
	plan, exists := PlanDefinitions[planType]
	return plan, exists
}

// GetAvailablePlans 获取所有可用计划
func GetAvailablePlans() []SubscriptionPlan {
	plans := make([]SubscriptionPlan, 0, len(PlanDefinitions))
	for _, plan := range PlanDefinitions {
		plans = append(plans, plan)
	}
	return plans
}

// CreateCustomPlan 创建自定义计划
func CreateCustomPlan(costLimit float64) SubscriptionPlan {
	plan := PlanDefinitions[PlanCustom]
	plan.CostLimit = costLimit
	
	// 基于成本限制估算 token 限制（假设平均每个 token 成本）
	avgTokenCost := 0.000015 // 大概估算
	plan.TokenLimit = int64(costLimit / avgTokenCost)
	
	return plan
}

// IsValidPlanType 检查计划类型是否有效
func IsValidPlanType(planType string) bool {
	_, exists := PlanDefinitions[PlanType(planType)]
	return exists
}

// GetPlanFeatures 获取计划功能描述
func GetPlanFeatures(planType PlanType) []string {
	if plan, exists := PlanDefinitions[planType]; exists {
		return plan.Features
	}
	return []string{}
}

// ComparePlans 比较两个计划
func ComparePlans(plan1, plan2 PlanType) int {
	// 返回 -1 如果 plan1 < plan2, 0 如果相等, 1 如果 plan1 > plan2
	order := map[PlanType]int{
		PlanFree:   0,
		PlanPro:    1,
		PlanMax5:   2,
		PlanMax20:  3,
		PlanCustom: 4,
	}
	
	val1, exists1 := order[plan1]
	val2, exists2 := order[plan2]
	
	if !exists1 || !exists2 {
		return 0
	}
	
	if val1 < val2 {
		return -1
	} else if val1 > val2 {
		return 1
	}
	return 0
}