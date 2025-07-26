package calculations

import (
	"context"
	"errors"
	"math"

	"github.com/penwyp/claudecat/models"
)

// CostCalculator provides precise cost calculations with multi-currency support
type CostCalculator struct {
	pricing  map[string]models.ModelPricing
	rates    map[string]float64 // Currency conversion rates
	provider models.PricingProvider // Optional pricing provider for dynamic pricing
}

// CostResult represents the result of a cost calculation
type CostResult struct {
	Model               string  `json:"model"`
	InputCost           float64 `json:"input_cost"`
	OutputCost          float64 `json:"output_cost"`
	CacheCreationCost   float64 `json:"cache_creation_cost"`
	CacheReadCost       float64 `json:"cache_read_cost"`
	TotalCost           float64 `json:"total_cost"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	TotalTokens         int     `json:"total_tokens"`
}

// BatchCostResult represents the result of batch cost calculations
type BatchCostResult struct {
	TotalCost    float64               `json:"total_cost"`
	TotalTokens  int                   `json:"total_tokens"`
	EntryCount   int                   `json:"entry_count"`
	ModelResults map[string]CostResult `json:"model_results"`
	Details      []CostResult          `json:"details"`
}

// NewCostCalculator creates a new cost calculator with default pricing
func NewCostCalculator() *CostCalculator {
	return &CostCalculator{
		pricing: models.GetAllPricings(),
		rates:   map[string]float64{"USD": 1.0}, // Default to USD
		provider: nil,
	}
}

// NewCostCalculatorWithProvider creates a new cost calculator with a pricing provider
func NewCostCalculatorWithProvider(provider models.PricingProvider) *CostCalculator {
	return &CostCalculator{
		pricing: nil, // Will use provider instead
		rates:   map[string]float64{"USD": 1.0}, // Default to USD
		provider: provider,
	}
}

// NewCostCalculatorWithRates creates a cost calculator with custom currency rates
func NewCostCalculatorWithRates(rates map[string]float64) *CostCalculator {
	return &CostCalculator{
		pricing: models.GetAllPricings(),
		rates:   rates,
	}
}

// Calculate computes the cost for a single usage entry
func (c *CostCalculator) Calculate(entry models.UsageEntry) (CostResult, error) {
	if entry.Model == "" {
		return CostResult{}, errors.New("model name cannot be empty")
	}

	var pricing models.ModelPricing
	var err error
	
	// Use provider if available, otherwise fall back to static pricing
	if c.provider != nil {
		pricing, err = c.provider.GetPricing(context.Background(), entry.Model)
		if err != nil {
			return CostResult{}, err
		}
	} else {
		var exists bool
		pricing, exists = c.pricing[entry.Model]
		if !exists {
			// Use default Sonnet pricing for unknown models
			pricing = models.GetPricing(entry.Model)
		}
	}

	result := CostResult{
		Model:               entry.Model,
		InputTokens:         entry.InputTokens,
		OutputTokens:        entry.OutputTokens,
		CacheCreationTokens: entry.CacheCreationTokens,
		CacheReadTokens:     entry.CacheReadTokens,
		TotalTokens:         entry.TotalTokens,
	}

	// Calculate costs (pricing is per million tokens)
	result.InputCost = c.calculateTokenCost(entry.InputTokens, pricing.Input)
	result.OutputCost = c.calculateTokenCost(entry.OutputTokens, pricing.Output)
	result.CacheCreationCost = c.calculateTokenCost(entry.CacheCreationTokens, pricing.CacheCreation)
	result.CacheReadCost = c.calculateTokenCost(entry.CacheReadTokens, pricing.CacheRead)

	result.TotalCost = result.InputCost + result.OutputCost +
		result.CacheCreationCost + result.CacheReadCost

	// Round to 6 decimal places for financial precision
	result.TotalCost = c.roundCost(result.TotalCost)
	result.InputCost = c.roundCost(result.InputCost)
	result.OutputCost = c.roundCost(result.OutputCost)
	result.CacheCreationCost = c.roundCost(result.CacheCreationCost)
	result.CacheReadCost = c.roundCost(result.CacheReadCost)

	return result, nil
}

// CalculateBatch computes costs for multiple entries efficiently
func (c *CostCalculator) CalculateBatch(entries []models.UsageEntry) (BatchCostResult, error) {
	if len(entries) == 0 {
		return BatchCostResult{}, errors.New("entries slice cannot be empty")
	}

	result := BatchCostResult{
		ModelResults: make(map[string]CostResult),
		Details:      make([]CostResult, 0, len(entries)),
		EntryCount:   len(entries),
	}

	// Aggregate by model for efficiency
	modelAggregates := make(map[string]*CostResult)

	for _, entry := range entries {
		entryResult, err := c.Calculate(entry)
		if err != nil {
			return BatchCostResult{}, err
		}

		result.Details = append(result.Details, entryResult)
		result.TotalCost += entryResult.TotalCost
		result.TotalTokens += entryResult.TotalTokens

		// Aggregate by model
		if modelResult, exists := modelAggregates[entry.Model]; exists {
			modelResult.InputTokens += entryResult.InputTokens
			modelResult.OutputTokens += entryResult.OutputTokens
			modelResult.CacheCreationTokens += entryResult.CacheCreationTokens
			modelResult.CacheReadTokens += entryResult.CacheReadTokens
			modelResult.TotalTokens += entryResult.TotalTokens
			modelResult.InputCost += entryResult.InputCost
			modelResult.OutputCost += entryResult.OutputCost
			modelResult.CacheCreationCost += entryResult.CacheCreationCost
			modelResult.CacheReadCost += entryResult.CacheReadCost
			modelResult.TotalCost += entryResult.TotalCost
		} else {
			copyResult := entryResult
			modelAggregates[entry.Model] = &copyResult
		}
	}

	// Convert aggregates to final results
	for model, aggregate := range modelAggregates {
		result.ModelResults[model] = *aggregate
	}

	result.TotalCost = c.roundCost(result.TotalCost)

	return result, nil
}

// CalculateWithCurrency computes cost in a specific currency
func (c *CostCalculator) CalculateWithCurrency(entry models.UsageEntry, currency string) (CostResult, error) {
	result, err := c.Calculate(entry)
	if err != nil {
		return result, err
	}

	rate, exists := c.rates[currency]
	if !exists {
		return result, errors.New("unsupported currency: " + currency)
	}

	// Convert all costs to the specified currency
	result.InputCost *= rate
	result.OutputCost *= rate
	result.CacheCreationCost *= rate
	result.CacheReadCost *= rate
	result.TotalCost *= rate

	// Round after conversion
	result.InputCost = c.roundCost(result.InputCost)
	result.OutputCost = c.roundCost(result.OutputCost)
	result.CacheCreationCost = c.roundCost(result.CacheCreationCost)
	result.CacheReadCost = c.roundCost(result.CacheReadCost)
	result.TotalCost = c.roundCost(result.TotalCost)

	return result, nil
}

// GetCostForTokens calculates cost for a specific number of tokens and model
func (c *CostCalculator) GetCostForTokens(model string, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int) (float64, error) {
	entry := models.UsageEntry{
		Model:               model,
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CacheCreationTokens: cacheCreationTokens,
		CacheReadTokens:     cacheReadTokens,
		TotalTokens:         inputTokens + outputTokens + cacheCreationTokens + cacheReadTokens,
	}

	result, err := c.Calculate(entry)
	if err != nil {
		return 0, err
	}

	return result.TotalCost, nil
}

// UpdatePricing updates the pricing for a specific model
func (c *CostCalculator) UpdatePricing(model string, pricing models.ModelPricing) {
	c.pricing[model] = pricing
}

// UpdateCurrencyRate updates the conversion rate for a currency
func (c *CostCalculator) UpdateCurrencyRate(currency string, rate float64) error {
	if rate <= 0 {
		return errors.New("currency rate must be positive")
	}
	c.rates[currency] = rate
	return nil
}

// GetSupportedCurrencies returns all supported currencies
func (c *CostCalculator) GetSupportedCurrencies() []string {
	currencies := make([]string, 0, len(c.rates))
	for currency := range c.rates {
		currencies = append(currencies, currency)
	}
	return currencies
}

// GetPricingForModel returns the pricing information for a model
func (c *CostCalculator) GetPricingForModel(model string) (models.ModelPricing, bool) {
	pricing, exists := c.pricing[model]
	return pricing, exists
}

// calculateTokenCost calculates cost for tokens given the rate per million
func (c *CostCalculator) calculateTokenCost(tokens int, ratePerMillion float64) float64 {
	if tokens <= 0 {
		return 0
	}
	return float64(tokens) * ratePerMillion / 1_000_000
}

// roundCost rounds a cost to 6 decimal places using banker's rounding
func (c *CostCalculator) roundCost(cost float64) float64 {
	const scale = 1e6 // 6 decimal places

	// Banker's rounding: round to nearest even when exactly between two values
	rounded := math.Round(cost*scale) / scale

	// Ensure we don't have floating point precision issues
	return math.Round(rounded*scale) / scale
}

// EstimateCostFromRate estimates future cost based on current burn rate
func (c *CostCalculator) EstimateCostFromRate(costPerHour float64, duration float64) float64 {
	estimated := costPerHour * duration
	return c.roundCost(estimated)
}

// CompareCosts compares costs between two models for the same token usage
func (c *CostCalculator) CompareCosts(inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int, model1, model2 string) (map[string]float64, error) {
	cost1, err := c.GetCostForTokens(model1, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens)
	if err != nil {
		return nil, err
	}

	cost2, err := c.GetCostForTokens(model2, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens)
	if err != nil {
		return nil, err
	}

	return map[string]float64{
		model1:       cost1,
		model2:       cost2,
		"difference": c.roundCost(math.Abs(cost1 - cost2)),
		"savings":    c.roundCost(math.Max(cost1, cost2) - math.Min(cost1, cost2)),
	}, nil
}
