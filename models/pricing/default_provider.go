package pricing

import (
	"context"
	"strings"

	"github.com/penwyp/claudecat/models"
)

// DefaultProvider implements PricingProvider using hardcoded pricing data
type DefaultProvider struct {
	pricing map[string]models.ModelPricing
}

// NewDefaultProvider creates a new default pricing provider with hardcoded values
func NewDefaultProvider() *DefaultProvider {
	return &DefaultProvider{
		pricing: map[string]models.ModelPricing{
			models.ModelOpus: {
				Input:         15.00, // $15 per million tokens
				Output:        75.00, // $75 per million tokens
				CacheCreation: 18.75, // $18.75 per million tokens
				CacheRead:     1.875, // $1.875 per million tokens
			},
			models.ModelSonnet: {
				Input:         3.00,  // $3 per million tokens
				Output:        15.00, // $15 per million tokens
				CacheCreation: 3.75,  // $3.75 per million tokens
				CacheRead:     0.30,  // $0.30 per million tokens
			},
			models.ModelHaiku: {
				Input:         0.80, // $0.80 per million tokens
				Output:        4.00, // $4 per million tokens
				CacheCreation: 1.00, // $1 per million tokens
				CacheRead:     0.08, // $0.08 per million tokens
			},
		},
	}
}

// GetPricing returns the pricing for a specific model
func (p *DefaultProvider) GetPricing(ctx context.Context, modelName string) (models.ModelPricing, error) {
	// Try exact match first
	if pricing, ok := p.pricing[modelName]; ok {
		return pricing, nil
	}

	// Try normalized model name
	normalized := models.NormalizeModelName(modelName)
	if pricing, ok := p.pricing[normalized]; ok {
		return pricing, nil
	}

	// Fallback based on model type in name
	modelLower := strings.ToLower(modelName)
	if strings.Contains(modelLower, "opus") {
		return p.pricing[models.ModelOpus], nil
	}
	if strings.Contains(modelLower, "sonnet") {
		return p.pricing[models.ModelSonnet], nil
	}
	if strings.Contains(modelLower, "haiku") {
		return p.pricing[models.ModelHaiku], nil
	}

	// Default to Sonnet pricing if model not found
	return p.pricing[models.ModelSonnet], nil
}

// GetAllPricings returns all available model pricings
func (p *DefaultProvider) GetAllPricings(ctx context.Context) (map[string]models.ModelPricing, error) {
	// Return a copy to prevent external modification
	result := make(map[string]models.ModelPricing)
	for k, v := range p.pricing {
		result[k] = v
	}
	return result, nil
}

// RefreshPricing is a no-op for the default provider
func (p *DefaultProvider) RefreshPricing(ctx context.Context) error {
	// No-op for hardcoded pricing
	return nil
}

// GetProviderName returns the name of this pricing provider
func (p *DefaultProvider) GetProviderName() string {
	return "default"
}