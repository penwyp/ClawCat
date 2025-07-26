package pricing

import (
	"context"
	"fmt"

	"github.com/penwyp/claudecat/logging"
	"github.com/penwyp/claudecat/models"
)

// CachedProvider wraps another provider with caching capabilities
type CachedProvider struct {
	provider     models.PricingProvider
	cacheManager *CacheManager
	useOffline   bool
}

// NewCachedProvider creates a new cached pricing provider
func NewCachedProvider(provider models.PricingProvider, cacheManager *CacheManager, useOffline bool) *CachedProvider {
	return &CachedProvider{
		provider:     provider,
		cacheManager: cacheManager,
		useOffline:   useOffline,
	}
}

// GetPricing returns the pricing for a specific model
func (p *CachedProvider) GetPricing(ctx context.Context, modelName string) (models.ModelPricing, error) {
	// If offline mode is requested, try cache first
	if p.useOffline {
		cache, err := p.cacheManager.LoadPricing(ctx)
		if err == nil {
			if pricing, ok := cache.Pricing[modelName]; ok {
				logging.LogDebugf("Using cached pricing for model %s from %s", modelName, cache.Source)
				return pricing, nil
			}
			// Try normalized name
			normalized := models.NormalizeModelName(modelName)
			if pricing, ok := cache.Pricing[normalized]; ok {
				logging.LogDebugf("Using cached pricing for normalized model %s from %s", normalized, cache.Source)
				return pricing, nil
			}
		}
		logging.LogDebugf("Cached pricing not found for model %s, falling back to provider", modelName)
	}

	// Get pricing from the underlying provider
	pricing, err := p.provider.GetPricing(ctx, modelName)
	if err != nil {
		// If provider fails and we have cache, try to use it as fallback
		if !p.useOffline && p.cacheManager.HasCache() {
			logging.LogInfof("Primary pricing provider failed, attempting to use cached data")
			cache, cacheErr := p.cacheManager.LoadPricing(ctx)
			if cacheErr == nil {
				if cachedPricing, ok := cache.Pricing[modelName]; ok {
					logging.LogInfof("Using fallback cached pricing for model %s", modelName)
					return cachedPricing, nil
				}
			}
		}
		return models.ModelPricing{}, err
	}

	// If not in offline mode and provider succeeded, update cache
	if !p.useOffline && p.provider.GetProviderName() != "default" {
		go func() {
			// Update cache in background
			allPricing, err := p.provider.GetAllPricings(context.Background())
			if err == nil {
				if err := p.cacheManager.SavePricing(context.Background(), p.provider.GetProviderName(), allPricing); err != nil {
					logging.LogDebugf("Failed to update pricing cache: %v", err)
				} else {
					logging.LogDebugf("Updated pricing cache from %s provider", p.provider.GetProviderName())
				}
			}
		}()
	}

	return pricing, nil
}

// GetAllPricings returns all available model pricings
func (p *CachedProvider) GetAllPricings(ctx context.Context) (map[string]models.ModelPricing, error) {
	// If offline mode is requested, try cache first
	if p.useOffline {
		cache, err := p.cacheManager.LoadPricing(ctx)
		if err == nil {
			logging.LogDebugf("Using cached pricing data from %s with %d models", cache.Source, len(cache.Pricing))
			return cache.Pricing, nil
		}
		logging.LogDebugf("Failed to load cached pricing: %v", err)
	}

	// Get pricing from the underlying provider
	pricing, err := p.provider.GetAllPricings(ctx)
	if err != nil {
		// If provider fails and we have cache, try to use it as fallback
		if !p.useOffline && p.cacheManager.HasCache() {
			logging.LogInfof("Primary pricing provider failed, attempting to use cached data")
			cache, cacheErr := p.cacheManager.LoadPricing(ctx)
			if cacheErr == nil {
				logging.LogInfof("Using fallback cached pricing data")
				return cache.Pricing, nil
			}
		}
		return nil, err
	}

	// If not in offline mode and provider succeeded, update cache
	if !p.useOffline && p.provider.GetProviderName() != "default" {
		if err := p.cacheManager.SavePricing(ctx, p.provider.GetProviderName(), pricing); err != nil {
			logging.LogDebugf("Failed to update pricing cache: %v", err)
		} else {
			logging.LogDebugf("Updated pricing cache from %s provider", p.provider.GetProviderName())
		}
	}

	return pricing, nil
}

// RefreshPricing forces a refresh of pricing data
func (p *CachedProvider) RefreshPricing(ctx context.Context) error {
	if p.useOffline {
		return fmt.Errorf("cannot refresh pricing in offline mode")
	}

	// Refresh the underlying provider
	if err := p.provider.RefreshPricing(ctx); err != nil {
		return err
	}

	// Update cache
	allPricing, err := p.provider.GetAllPricings(ctx)
	if err != nil {
		return err
	}

	return p.cacheManager.SavePricing(ctx, p.provider.GetProviderName(), allPricing)
}

// GetProviderName returns the name of this pricing provider
func (p *CachedProvider) GetProviderName() string {
	if p.useOffline {
		return fmt.Sprintf("%s-offline", p.provider.GetProviderName())
	}
	return fmt.Sprintf("%s-cached", p.provider.GetProviderName())
}