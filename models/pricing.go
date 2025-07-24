package models

// ModelPricing defines token pricing for different Claude models
type ModelPricing struct {
	Input         float64 // Per million tokens
	Output        float64 // Per million tokens
	CacheCreation float64 // Per million tokens
	CacheRead     float64 // Per million tokens
}

// Plan represents a subscription plan with token and cost limits
type Plan struct {
	Name       string  `json:"name"`
	TokenLimit int     `json:"token_limit"`
	CostLimit  float64 `json:"cost_limit"`
}

// modelPricingMap stores pricing for all Claude models
var modelPricingMap = map[string]ModelPricing{
	ModelOpus: {
		Input:         15.00, // $15 per million tokens
		Output:        75.00, // $75 per million tokens
		CacheCreation: 18.75, // $18.75 per million tokens
		CacheRead:     1.875, // $1.875 per million tokens
	},
	ModelSonnet: {
		Input:         3.00,  // $3 per million tokens
		Output:        15.00, // $15 per million tokens
		CacheCreation: 3.75,  // $3.75 per million tokens
		CacheRead:     0.30,  // $0.30 per million tokens
	},
	ModelHaiku: {
		Input:         0.80, // $0.80 per million tokens
		Output:        4.00, // $4 per million tokens
		CacheCreation: 1.00, // $1 per million tokens
		CacheRead:     0.08, // $0.08 per million tokens
	},
}

// planMap stores all available subscription plans
var planMap = map[string]Plan{
	PlanPro: {
		Name:       "Claude Pro",
		TokenLimit: -1, // Unlimited
		CostLimit:  20.00,
	},
	PlanMax5: {
		Name:       "Claude Max 5",
		TokenLimit: -1, // Unlimited
		CostLimit:  5.00,
	},
	PlanMax20: {
		Name:       "Claude Max 20",
		TokenLimit: -1, // Unlimited
		CostLimit:  20.00,
	},
}

// GetPricing returns the pricing for a specific model
func GetPricing(model string) ModelPricing {
	if pricing, ok := modelPricingMap[model]; ok {
		return pricing
	}
	// Default to Sonnet pricing if model not found
	return modelPricingMap[ModelSonnet]
}

// GetPlan returns a specific subscription plan
func GetPlan(planName string) Plan {
	if plan, ok := planMap[planName]; ok {
		return plan
	}
	// Default to Pro plan if not found
	return planMap[PlanPro]
}

// GetAllPlans returns all available plans
func GetAllPlans() map[string]Plan {
	// Return a copy to prevent external modification
	result := make(map[string]Plan)
	for k, v := range planMap {
		result[k] = v
	}
	return result
}

// GetAllPricings returns all model pricings
func GetAllPricings() map[string]ModelPricing {
	// Return a copy to prevent external modification
	result := make(map[string]ModelPricing)
	for k, v := range modelPricingMap {
		result[k] = v
	}
	return result
}
