package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/penwyp/claudecat/models"
)

const (
	liteLLMPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	cacheExpiration   = 24 * time.Hour // Cache pricing data for 24 hours
)

// LiteLLMProvider implements PricingProvider by fetching pricing from LiteLLM's repository
type LiteLLMProvider struct {
	mu            sync.RWMutex
	pricing       map[string]models.ModelPricing
	lastFetchTime time.Time
	httpClient    *http.Client
}

// liteLLMModel represents the structure of a model in LiteLLM's pricing data
type liteLLMModel struct {
	InputCostPerToken              *float64 `json:"input_cost_per_token"`
	OutputCostPerToken             *float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost    *float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost        *float64 `json:"cache_read_input_token_cost"`
}

// NewLiteLLMProvider creates a new LiteLLM pricing provider
func NewLiteLLMProvider() *LiteLLMProvider {
	return &LiteLLMProvider{
		pricing: make(map[string]models.ModelPricing),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetPricing returns the pricing for a specific model
func (p *LiteLLMProvider) GetPricing(ctx context.Context, modelName string) (models.ModelPricing, error) {
	// Ensure pricing data is loaded
	if err := p.ensurePricingLoaded(ctx); err != nil {
		return models.ModelPricing{}, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try exact match
	if pricing, ok := p.pricing[modelName]; ok {
		return pricing, nil
	}

	// Try with provider prefix variations
	variations := []string{
		modelName,
		fmt.Sprintf("anthropic/%s", modelName),
		fmt.Sprintf("claude-3-5-%s", modelName),
		fmt.Sprintf("claude-3-%s", modelName),
		fmt.Sprintf("claude-%s", modelName),
	}

	for _, variant := range variations {
		if pricing, ok := p.pricing[variant]; ok {
			return pricing, nil
		}
	}

	// Try partial matches
	modelLower := strings.ToLower(modelName)
	for key, pricing := range p.pricing {
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, modelLower) || strings.Contains(modelLower, keyLower) {
			return pricing, nil
		}
	}

	return models.ModelPricing{}, fmt.Errorf("%w: %s", models.ErrPricingNotFound, modelName)
}

// GetAllPricings returns all available model pricings
func (p *LiteLLMProvider) GetAllPricings(ctx context.Context) (map[string]models.ModelPricing, error) {
	// Ensure pricing data is loaded
	if err := p.ensurePricingLoaded(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]models.ModelPricing)
	for k, v := range p.pricing {
		result[k] = v
	}
	return result, nil
}

// RefreshPricing forces a refresh of pricing data
func (p *LiteLLMProvider) RefreshPricing(ctx context.Context) error {
	return p.fetchPricing(ctx)
}

// GetProviderName returns the name of this pricing provider
func (p *LiteLLMProvider) GetProviderName() string {
	return "litellm"
}

// ensurePricingLoaded checks if pricing data needs to be loaded or refreshed
func (p *LiteLLMProvider) ensurePricingLoaded(ctx context.Context) error {
	p.mu.RLock()
	needsRefresh := time.Since(p.lastFetchTime) > cacheExpiration || len(p.pricing) == 0
	p.mu.RUnlock()

	if needsRefresh {
		return p.fetchPricing(ctx)
	}
	return nil
}

// fetchPricing fetches the latest pricing data from LiteLLM
func (p *LiteLLMProvider) fetchPricing(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, liteLLMPricingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch pricing data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the JSON data
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawData); err != nil {
		return fmt.Errorf("failed to parse pricing data: %w", err)
	}

	// Convert to our pricing format
	newPricing := make(map[string]models.ModelPricing)
	for modelName, rawModel := range rawData {
		var model liteLLMModel
		if err := json.Unmarshal(rawModel, &model); err != nil {
			// Skip models that don't match our expected structure
			continue
		}

		// Only process models with pricing information
		if model.InputCostPerToken == nil || model.OutputCostPerToken == nil {
			continue
		}

		// Convert from cost per token to cost per million tokens
		pricing := models.ModelPricing{
			Input:  *model.InputCostPerToken * 1_000_000,
			Output: *model.OutputCostPerToken * 1_000_000,
		}

		// Add cache pricing if available
		if model.CacheCreationInputTokenCost != nil {
			pricing.CacheCreation = *model.CacheCreationInputTokenCost * 1_000_000
		} else {
			// Default to 1.25x input cost if not specified
			pricing.CacheCreation = pricing.Input * 1.25
		}

		if model.CacheReadInputTokenCost != nil {
			pricing.CacheRead = *model.CacheReadInputTokenCost * 1_000_000
		} else {
			// Default to 0.1x input cost if not specified
			pricing.CacheRead = pricing.Input * 0.1
		}

		newPricing[modelName] = pricing
	}

	// Update the cached pricing
	p.mu.Lock()
	p.pricing = newPricing
	p.lastFetchTime = time.Now()
	p.mu.Unlock()

	return nil
}