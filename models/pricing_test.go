package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPricing(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  ModelPricing
	}{
		{
			name:  "opus pricing",
			model: ModelOpus,
			want: ModelPricing{
				Input:         15.00,
				Output:        75.00,
				CacheCreation: 18.75,
				CacheRead:     1.875,
			},
		},
		{
			name:  "sonnet pricing",
			model: ModelSonnet,
			want: ModelPricing{
				Input:         3.00,
				Output:        15.00,
				CacheCreation: 3.75,
				CacheRead:     0.30,
			},
		},
		{
			name:  "haiku pricing",
			model: ModelHaiku,
			want: ModelPricing{
				Input:         0.80,
				Output:        4.00,
				CacheCreation: 1.00,
				CacheRead:     0.08,
			},
		},
		{
			name:  "unknown model defaults to sonnet",
			model: "unknown-model",
			want: ModelPricing{
				Input:         3.00,
				Output:        15.00,
				CacheCreation: 3.75,
				CacheRead:     0.30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPricing(tt.model)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPlan(t *testing.T) {
	tests := []struct {
		name     string
		planName string
		want     Plan
	}{
		{
			name:     "pro plan",
			planName: PlanPro,
			want: Plan{
				Name:       "Claude Pro",
				TokenLimit: -1,
				CostLimit:  20.00,
			},
		},
		{
			name:     "max5 plan",
			planName: PlanMax5,
			want: Plan{
				Name:       "Claude Max 5",
				TokenLimit: -1,
				CostLimit:  5.00,
			},
		},
		{
			name:     "max20 plan",
			planName: PlanMax20,
			want: Plan{
				Name:       "Claude Max 20",
				TokenLimit: -1,
				CostLimit:  20.00,
			},
		},
		{
			name:     "unknown plan defaults to pro",
			planName: "unknown-plan",
			want: Plan{
				Name:       "Claude Pro",
				TokenLimit: -1,
				CostLimit:  20.00,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPlan(tt.planName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetAllPlans(t *testing.T) {
	plans := GetAllPlans()

	// Check we have all expected plans
	assert.Len(t, plans, 3)
	assert.Contains(t, plans, PlanPro)
	assert.Contains(t, plans, PlanMax5)
	assert.Contains(t, plans, PlanMax20)

	// Verify it's a copy by modifying the returned map
	plans[PlanPro] = Plan{Name: "Modified"}
	
	// Original should be unchanged
	originalPlan := GetPlan(PlanPro)
	assert.Equal(t, "Claude Pro", originalPlan.Name)
}

func TestGetAllPricings(t *testing.T) {
	pricings := GetAllPricings()

	// Check we have all expected models
	assert.Len(t, pricings, 3)
	assert.Contains(t, pricings, ModelOpus)
	assert.Contains(t, pricings, ModelSonnet)
	assert.Contains(t, pricings, ModelHaiku)

	// Verify it's a copy by modifying the returned map
	pricings[ModelSonnet] = ModelPricing{Input: 999.99}
	
	// Original should be unchanged
	originalPricing := GetPricing(ModelSonnet)
	assert.Equal(t, 3.00, originalPricing.Input)
}

func TestPricingConsistency(t *testing.T) {
	// Verify that output is more expensive than input for all models
	pricings := GetAllPricings()
	
	for model, pricing := range pricings {
		assert.Greater(t, pricing.Output, pricing.Input, 
			"Output should be more expensive than input for model %s", model)
		
		// Cache creation should be more expensive than cache read
		assert.Greater(t, pricing.CacheCreation, pricing.CacheRead,
			"Cache creation should be more expensive than cache read for model %s", model)
	}
}

func TestPlanConsistency(t *testing.T) {
	plans := GetAllPlans()
	
	for planID, plan := range plans {
		// All current plans have unlimited tokens
		assert.Equal(t, -1, plan.TokenLimit,
			"Plan %s should have unlimited tokens", planID)
		
		// Cost limit should be positive
		assert.Greater(t, plan.CostLimit, 0.0,
			"Plan %s should have positive cost limit", planID)
		
		// Name should not be empty
		assert.NotEmpty(t, plan.Name,
			"Plan %s should have a name", planID)
	}
}