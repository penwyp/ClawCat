package pricing

import (
	"fmt"

	"github.com/penwyp/claudecat/config"
	"github.com/penwyp/claudecat/models"
)

// CreatePricingProvider creates a pricing provider based on configuration
func CreatePricingProvider(cfg *config.DataConfig, cacheDir string) (models.PricingProvider, error) {
	// Create base provider based on source
	var baseProvider models.PricingProvider

	switch cfg.PricingSource {
	case "default", "":
		baseProvider = NewDefaultProvider()
	case "litellm":
		baseProvider = NewLiteLLMProvider()
	default:
		return nil, fmt.Errorf("unknown pricing source: %s", cfg.PricingSource)
	}

	// If offline mode or non-default provider, wrap with caching
	if cfg.PricingOfflineMode || cfg.PricingSource != "default" {
		cacheManager, err := NewCacheManager(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache manager: %w", err)
		}

		return NewCachedProvider(baseProvider, cacheManager, cfg.PricingOfflineMode), nil
	}

	return baseProvider, nil
}
