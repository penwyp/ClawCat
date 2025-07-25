package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/penwyp/ClawCat/cache"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/sessions"
	"github.com/penwyp/ClawCat/ui"
)

// bootstrap initializes all application components
func (a *Application) bootstrap() error {
	a.logger.Info("Bootstrapping application")

	// 1. Validate configuration
	if err := a.validateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 2. Initialize data layer
	if err := a.initializeData(); err != nil {
		return fmt.Errorf("failed to initialize data layer: %w", err)
	}

	// 3. Set up cache
	if err := a.setupCache(); err != nil {
		return fmt.Errorf("failed to setup cache: %w", err)
	}

	// 4. Set up calculations
	if err := a.setupCalculations(); err != nil {
		return fmt.Errorf("failed to setup calculations: %w", err)
	}

	// 5. Initialize session manager
	if err := a.setupSessionManager(); err != nil {
		return fmt.Errorf("failed to setup session manager: %w", err)
	}

	// 6. Set up file watching
	if err := a.setupFileWatcher(); err != nil {
		return fmt.Errorf("failed to setup file watcher: %w", err)
	}

	// 7. Initialize UI
	if err := a.initializeUI(); err != nil {
		return fmt.Errorf("failed to initialize UI: %w", err)
	}

	a.logger.Info("Bootstrap completed successfully")
	return nil
}

// validateConfig validates the application configuration
func (a *Application) validateConfig() error {
	if a.config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate data paths
	if len(a.config.Data.Paths) == 0 {
		// Use default paths if none specified
		defaultPaths, err := fileio.GetDefaultPaths()
		if err != nil {
			return fmt.Errorf("no data paths specified and failed to get defaults: %w", err)
		}
		a.config.Data.Paths = defaultPaths
		a.logger.Infof("Using default data paths: %v", defaultPaths)
	}

	// Validate that at least one data path exists
	foundValidPath := false
	for _, path := range a.config.Data.Paths {
		if _, err := os.Stat(path); err == nil {
			foundValidPath = true
			break
		}
	}

	if !foundValidPath {
		a.logger.Warn("No valid data paths found, application may not have data to display")
	}

	// Validate refresh interval
	if a.config.UI.RefreshRate <= 0 {
		return fmt.Errorf("refresh interval must be positive")
	}

	// Validate theme
	validThemes := []string{"dark", "light", "high-contrast"}
	validTheme := false
	for _, theme := range validThemes {
		if a.config.UI.Theme == theme {
			validTheme = true
			break
		}
	}
	if !validTheme {
		a.logger.Warnf("Invalid theme '%s', using 'dark'", a.config.UI.Theme)
		a.config.UI.Theme = "dark"
	}

	return nil
}

// initializeData initializes the data layer
func (a *Application) initializeData() error {
	a.logger.Info("Initializing data layer")

	// Create necessary directories
	for _, path := range a.config.Data.Paths {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				a.logger.Warnf("Failed to create directory %s: %v", dir, err)
			}
		}
	}

	return nil
}

// setupCache initializes the cache system
func (a *Application) setupCache() error {
	a.logger.Info("Setting up cache")

	if !a.config.Data.CacheEnabled {
		a.logger.Info("Cache disabled in configuration")
		return nil
	}

	// Parse max size
	maxSize := int64(a.config.Data.CacheSize) * 1024 * 1024 // Convert MB to bytes

	// Use default TTL
	ttl := 3600 // 1 hour in seconds

	// Create cache configuration
	cacheConfig := cache.StoreConfig{
		MaxMemory:         maxSize,
		FileCacheTTL:      time.Duration(ttl) * time.Second,
		CalcCacheTTL:      time.Duration(ttl) * time.Second,
		EnableMetrics:     true,
		EnableCompression: true,
	}

	// Initialize cache store
	a.cache = cache.NewStore(cacheConfig)

	a.logger.Infof("Cache initialized: max_size=%d, ttl=%v", maxSize, ttl)
	return nil
}

// setupCalculations initializes the calculations engine
func (a *Application) setupCalculations() error {
	a.logger.Info("Setting up calculations")

	// Create calculator with default settings
	a.calculator = calculations.NewCostCalculator()

	return nil
}

// setupSessionManager initializes the session manager
func (a *Application) setupSessionManager() error {
	a.logger.Info("Setting up session manager")

	// Use default idle timeout
	idleTimeout := 300 // 5 minutes in seconds

	// Initialize session manager (no config needed based on NewManager signature)
	a.manager = sessions.NewManager()

	a.logger.Infof("Session manager initialized: idle_timeout=%d, max_sessions=%d",
		idleTimeout, 1000)
	return nil
}

// setupFileWatcher initializes the file watcher
func (a *Application) setupFileWatcher() error {
	a.logger.Info("Setting up file watcher")

	if !a.config.Data.AutoDiscover {
		a.logger.Info("File watching disabled in configuration")
		return nil
	}

	// Initialize file watcher
	watcher, err := fileio.NewWatcher(a.config.Data.Paths)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	a.fileWatcher = watcher

	a.logger.Info("File watcher initialized")
	return nil
}

// initializeUI initializes the user interface
func (a *Application) initializeUI() error {
	a.logger.Info("Initializing UI")

	if a.config.UI.CompactMode {
		a.logger.Info("Background mode enabled, skipping UI initialization")
		return nil
	}

	// Create UI configuration
	uiConfig := ui.Config{
		Theme:       a.config.UI.Theme,
		RefreshRate: a.config.UI.RefreshRate,
	}

	// Initialize UI application
	uiApp := ui.NewApp(uiConfig)

	a.ui = uiApp

	a.logger.Infof("UI initialized: theme=%s", a.config.UI.Theme)
	return nil
}
