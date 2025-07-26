package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/penwyp/claudecat/models"
)

// CacheManager handles caching of pricing data for offline use
type CacheManager struct {
	mu        sync.RWMutex
	cacheDir  string
	cacheFile string
}

// PricingCache represents the cached pricing data
type PricingCache struct {
	Source    string                        `json:"source"`
	UpdatedAt time.Time                     `json:"updated_at"`
	Pricing   map[string]models.ModelPricing `json:"pricing"`
}

// NewCacheManager creates a new pricing cache manager
func NewCacheManager(cacheDir string) (*CacheManager, error) {
	// Expand ~ to home directory
	if cacheDir[:2] == "~/" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(homeDir, cacheDir[2:])
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &CacheManager{
		cacheDir:  cacheDir,
		cacheFile: filepath.Join(cacheDir, "pricing_cache.json"),
	}, nil
}

// SavePricing saves pricing data to cache
func (m *CacheManager) SavePricing(ctx context.Context, source string, pricing map[string]models.ModelPricing) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cache := PricingCache{
		Source:    source,
		UpdatedAt: time.Now(),
		Pricing:   pricing,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pricing cache: %w", err)
	}

	// Write to temporary file first
	tmpFile := m.cacheFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Rename to final location (atomic operation)
	if err := os.Rename(tmpFile, m.cacheFile); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	return nil
}

// LoadPricing loads pricing data from cache
func (m *CacheManager) LoadPricing(ctx context.Context) (*PricingCache, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cached pricing data available")
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache PricingCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pricing cache: %w", err)
	}

	return &cache, nil
}

// HasCache checks if cached pricing data exists
func (m *CacheManager) HasCache() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, err := os.Stat(m.cacheFile)
	return err == nil
}

// GetCacheAge returns how old the cached data is
func (m *CacheManager) GetCacheAge() (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, err := os.Stat(m.cacheFile)
	if err != nil {
		return 0, err
	}

	return time.Since(info.ModTime()), nil
}

// ClearCache removes the cached pricing data
func (m *CacheManager) ClearCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := os.Remove(m.cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	return nil
}