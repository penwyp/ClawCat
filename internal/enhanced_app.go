package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/penwyp/ClawCat/cache"
	"github.com/penwyp/ClawCat/calculations"
	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/errors"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/orchestrator"
	"github.com/penwyp/ClawCat/sessions"
	"github.com/penwyp/ClawCat/ui"
)

// EnhancedApplication represents the main application orchestrator using the new architecture
type EnhancedApplication struct {
	config       *config.Config
	orchestrator *orchestrator.MonitoringOrchestrator
	metricsCalc  *calculations.EnhancedMetricsCalculator
	cache        *cache.Store
	cacheWarmer  *cache.CacheWarmer
	ui           *ui.App
	errorHandler *errors.EnhancedErrorHandler

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	metrics *Metrics
	logger  logging.LoggerInterface

	// Application state
	running bool
	mu      sync.RWMutex
}

// NewEnhancedApplication creates a new enhanced application instance
func NewEnhancedApplication(cfg *config.Config) (*EnhancedApplication, error) {
	ctx, cancel := context.WithCancel(context.Background())

	app := &EnhancedApplication{
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
		logger:       logging.NewLogger(cfg.App.LogLevel, cfg.App.LogFile),
		errorHandler: errors.NewEnhancedErrorHandler(),
	}

	if err := app.bootstrap(); err != nil {
		cancel()
		return nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	return app, nil
}

// Run starts the enhanced application and blocks until shutdown
func (ea *EnhancedApplication) Run() error {
	ea.mu.Lock()
	if ea.running {
		ea.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	ea.running = true
	ea.mu.Unlock()

	ea.logger.Info("Starting ClawCat enhanced application")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Start all components
	if err := ea.start(); err != nil {
		return ea.errorHandler.RetryWithBackoff(
			ea.ctx,
			func() error { return ea.start() },
			"application_start",
		)
	}

	// Handle signals in a separate goroutine
	ea.wg.Add(1)
	go ea.handleSignals(sigCh)

	// Start the UI (this blocks until the UI exits)
	var err error
	if ea.config.UI.CompactMode {
		err = ea.runBackground()
	} else {
		err = ea.runInteractive()
	}

	// Signal shutdown
	ea.cancel()

	// Wait for all goroutines to finish
	ea.wg.Wait()

	// Perform cleanup
	if shutdownErr := ea.shutdown(); shutdownErr != nil {
		ea.logger.Errorf("Shutdown error: %v", shutdownErr)
		if err == nil {
			err = shutdownErr
		}
	}

	ea.logger.Info("ClawCat enhanced application stopped")
	return err
}

// bootstrap initializes all application components
func (ea *EnhancedApplication) bootstrap() error {
	// Initialize cache with configuration
	ea.cache = cache.NewStore(cache.StoreConfig{
		MaxFileSize:       10 * 1024 * 1024,                // 10MB
		MaxMemory:         ea.config.Cache.MaxMemory,       // From config
		MaxDiskSize:       ea.config.Cache.MaxDiskSize,     // From config
		DiskCacheDir:      ea.config.Cache.Dir,             // From config
		FileCacheTTL:      24 * time.Hour,                  // File cache TTL
		CalcCacheTTL:      1 * time.Hour,                   // Memory cache TTL
		DiskCacheTTL:      ea.config.Cache.TTL,             // Disk cache TTL from config
		CleanupInterval:   ea.config.Cache.CleanupInterval, // Cleanup interval from config
		CompressionLevel:  6,
		EnableMetrics:     true,
		EnableCompression: true,
		EnableDiskCache:   ea.config.Cache.Enabled, // Enable disk cache from config
	})

	// Initialize metrics calculator
	ea.metricsCalc = calculations.NewEnhancedMetricsCalculator(ea.config)

	// Initialize cache warmer
	ea.cacheWarmer = cache.NewCacheWarmer(ea.cache, 4)

	// Start cache warming if enabled
	if ea.config.Cache.Enabled {
		ea.startCacheWarming()
	}

	// Initialize orchestrator with data paths
	dataPath := ea.getDataPath()
	updateInterval := time.Duration(ea.config.UI.RefreshRate)
	if updateInterval <= 0 {
		updateInterval = 10 * time.Second // Default
	}

	ea.orchestrator = orchestrator.NewMonitoringOrchestrator(
		updateInterval,
		dataPath,
		ea.config,
	)

	// Initialize UI
	uiConfig := ui.Config{
		RefreshRate:      ea.config.UI.RefreshRate,
		Theme:            ea.config.UI.Theme,
		ShowSpinner:      true,
		CompactMode:      ea.config.UI.CompactMode,
		ChartHeight:      10,
		TablePageSize:    20,
		SubscriptionPlan: ea.config.Subscription.Plan,
	}
	ea.ui = ui.NewApp(uiConfig)

	// Initialize metrics
	if ea.config.Debug.MetricsPort > 0 {
		ea.metrics = NewMetrics(ea.config.Debug.MetricsPort)
	}

	return nil
}

// start initializes and starts all application components
func (ea *EnhancedApplication) start() error {
	ea.logger.Info("Starting enhanced application components")

	// Register data update callback with orchestrator
	ea.orchestrator.RegisterUpdateCallback(ea.onDataUpdate)

	// Register session change callback
	ea.orchestrator.RegisterSessionCallback(ea.onSessionChange)

	// Set command line arguments for token limit calculation
	// This would be set from the CLI args in a real implementation
	ea.orchestrator.SetArgs(map[string]interface{}{
		"plan": ea.config.Subscription.Plan,
	})

	// Start the orchestrator
	if err := ea.orchestrator.Start(); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	// Start metrics collection if enabled
	if ea.config.Debug.MetricsPort > 0 && ea.metrics != nil {
		ea.wg.Add(1)
		go ea.collectMetrics()
	}

	// Wait for initial data with timeout
	ea.logger.Info("Waiting for initial data...")
	if !ea.orchestrator.WaitForInitialData(10 * time.Second) {
		ea.logger.Warn("Timeout waiting for initial data, continuing anyway")
	} else {
		ea.logger.Info("Initial data received successfully")
	}

	return nil
}

// runInteractive starts the TUI application
func (ea *EnhancedApplication) runInteractive() error {
	ea.logger.Info("Starting interactive TUI mode")

	// The UI data source will be updated via callbacks
	// No need to set it directly since we use the callback mechanism

	// Start the UI application
	return ea.ui.Start()
}

// runBackground runs in background mode without TUI
func (ea *EnhancedApplication) runBackground() error {
	ea.logger.Info("Starting background mode")

	// In background mode, just wait for context cancellation
	<-ea.ctx.Done()
	return nil
}

// onDataUpdate handles data updates from the orchestrator
func (ea *EnhancedApplication) onDataUpdate(data orchestrator.MonitoringData) {
	defer func() {
		if r := recover(); r != nil {
			ea.errorHandler.ReportError(
				fmt.Errorf("panic in data update: %v", r),
				"enhanced_app",
				"data_update_panic",
				map[string]interface{}{
					"panic_value": r,
				},
				nil,
				errors.ErrorLevelError,
			)
		}
	}()

	ea.logger.Debug("=== DATA UPDATE CALLBACK ===")
	ea.logger.Debugf("Received %d blocks from orchestrator", len(data.Data.Blocks))

	// Update metrics calculator with new session blocks
	ea.metricsCalc.UpdateSessionBlocks(data.Data.Blocks)

	// Calculate enhanced metrics
	metrics := ea.metricsCalc.Calculate()
	ea.logger.Debugf("Calculated metrics - Current tokens: %d, Current cost: $%.4f",
		metrics.CurrentTokens, metrics.CurrentCost)

	// Update UI if running interactively
	if ea.ui != nil {
		// Convert the data to the format expected by the UI
		sessions := ea.convertBlocksToSessions(data.Data.Blocks)
		entries := ea.extractEntriesFromBlocks(data.Data.Blocks)
		ea.logger.Infof("Updating UI with %d sessions and %d entries", len(sessions), len(entries))
		ea.ui.UpdateData(sessions, entries)

		// Also send the metrics update
		if metrics != nil {
			// Convert enhanced metrics to realtime metrics for UI
			burnRate := float64(0)
			if metrics.BurnRate != nil {
				burnRate = metrics.BurnRate.TokensPerMinute
			}

			// Convert model distribution
			modelDistribution := make(map[string]calculations.ModelMetrics)
			for model, stats := range metrics.ModelDistribution {
				modelDistribution[model] = calculations.ModelMetrics{
					TokenCount: stats.TotalTokens,
					Cost:       stats.Cost,
				}
			}
			
			realtimeMetrics := &calculations.RealtimeMetrics{
				CurrentTokens:     metrics.CurrentTokens,
				CurrentCost:       metrics.CurrentCost,
				BurnRate:          burnRate,
				SessionStart:      metrics.SessionStart,
				SessionEnd:        metrics.SessionEnd,
				ModelDistribution: modelDistribution,
			}
			ea.ui.SendMessage(ui.RealtimeMetricsMsg{Metrics: realtimeMetrics})
		}
	} else {
		ea.logger.Info("UI is not initialized - skipping UI update")
	}

	// Update application metrics
	ea.updateApplicationMetrics(metrics)

	ea.logger.Debugf("Processed data update with %d blocks", len(data.Data.Blocks))
	ea.logger.Debug("=== END DATA UPDATE ===")
}

// onSessionChange handles session change events
func (ea *EnhancedApplication) onSessionChange(eventType, sessionID string, sessionData interface{}) {
	ea.logger.Infof("Session change: %s for session %s", eventType, sessionID)

	// Handle session changes if needed
	// This could be used for notifications, logging, etc.
}

// convertBlocksToSessions converts session blocks to the format expected by the legacy UI
func (ea *EnhancedApplication) convertBlocksToSessions(blocks []models.SessionBlock) []*sessions.Session {
	var result []*sessions.Session

	for _, block := range blocks {
		if block.IsGap {
			continue // Skip gap blocks
		}

		session := &sessions.Session{
			ID:        block.ID,
			StartTime: block.StartTime,
			EndTime:   block.EndTime,
			IsActive:  block.IsActive,
			Entries:   block.Entries,
			Stats: sessions.SessionStats{
				TotalTokens: block.TokenCounts.TotalTokens(),
				TotalCost:   block.CostUSD,
				// Convert per-model stats if needed
				ModelBreakdown: make(map[string]calculations.ModelStats),
			},
			LastUpdate: time.Now(),
		}

		result = append(result, session)
	}

	return result
}

// extractEntriesFromBlocks extracts all usage entries from session blocks
func (ea *EnhancedApplication) extractEntriesFromBlocks(blocks []models.SessionBlock) []models.UsageEntry {
	var result []models.UsageEntry

	for _, block := range blocks {
		result = append(result, block.Entries...)
	}

	return result
}

// updateApplicationMetrics updates application-level metrics
func (ea *EnhancedApplication) updateApplicationMetrics(metrics *calculations.EnhancedRealtimeMetrics) {
	if ea.metrics == nil {
		return
	}

	// Update metrics with current values
	ea.metrics.TotalTokens = int64(metrics.CurrentTokens)
	ea.metrics.TotalCost = metrics.CurrentCost
	ea.metrics.ActiveSessions = 0
	if metrics.IsActive {
		ea.metrics.ActiveSessions = 1
	}
}

// getDataPath determines the data path to monitor
func (ea *EnhancedApplication) getDataPath() string {
	if len(ea.config.Data.Paths) > 0 {
		path := ea.config.Data.Paths[0]
		ea.logger.Infof("Using configured data path: %s", path)
		return path
	}

	// Try default paths in order of preference
	homeDir, _ := os.UserHomeDir()
	defaultPaths := []string{
		fmt.Sprintf("%s/.claude/projects", homeDir),
		fmt.Sprintf("%s/.config/claude/projects", homeDir),
		fmt.Sprintf("%s/Library/Application Support/claude/projects", homeDir),
	}

	// Also check test data path for development/testing
	testDataPath := "/Users/penwyp/Dat/worktree/claude_data_snapshot/projects"
	if _, err := os.Stat(testDataPath); err == nil {
		defaultPaths = append([]string{testDataPath}, defaultPaths...)
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			ea.logger.Infof("Using discovered data path: %s", path)
			return path
		}
	}

	// Fallback to first default path even if it doesn't exist
	defaultPath := defaultPaths[0]
	ea.logger.Warnf("No existing data paths found, using default: %s", defaultPath)
	ea.logger.Warnf("To specify a custom path, use: clawcat run --paths /path/to/claude/data")
	return defaultPath
}

// handleSignals handles OS signals
func (ea *EnhancedApplication) handleSignals(sigCh <-chan os.Signal) {
	defer ea.wg.Done()

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				ea.logger.Info("Received shutdown signal")
				ea.cancel()
				return

			case syscall.SIGHUP:
				ea.logger.Info("Received SIGHUP, reloading configuration")
				if err := ea.reloadConfig(); err != nil {
					ea.errorHandler.ReportError(
						err,
						"enhanced_app",
						"config_reload_failed",
						nil,
						nil,
						errors.ErrorLevelError,
					)
				}
			}

		case <-ea.ctx.Done():
			return
		}
	}
}

// reloadConfig reloads the configuration
func (ea *EnhancedApplication) reloadConfig() error {
	// This would implement configuration reloading
	ea.logger.Info("Configuration reload not implemented yet")
	return nil
}

// collectMetrics collects and exports metrics
func (ea *EnhancedApplication) collectMetrics() {
	defer ea.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if ea.metrics != nil {
				ea.metrics.Export()
			}

		case <-ea.ctx.Done():
			return
		}
	}
}

// shutdown performs application cleanup
func (ea *EnhancedApplication) shutdown() error {
	ea.logger.Info("Shutting down enhanced application")

	// Stop orchestrator
	if ea.orchestrator != nil {
		ea.orchestrator.Stop()
	}

	// Close metrics calculator
	if ea.metricsCalc != nil {
		ea.metricsCalc.Close()
	}

	// Stop UI
	if ea.ui != nil {
		ea.ui.Stop()
	}

	return nil
}

// GetOrchestrator returns the monitoring orchestrator (for testing/debugging)
func (ea *EnhancedApplication) GetOrchestrator() *orchestrator.MonitoringOrchestrator {
	return ea.orchestrator
}

// startCacheWarming starts asynchronous cache warming
func (ea *EnhancedApplication) startCacheWarming() {
	dataPath := ea.getDataPath()
	if dataPath == "" {
		return
	}

	// Create warmup configuration
	patterns := cache.DefaultWarmupPatterns(dataPath)
	config := cache.WarmupConfig{
		Patterns:       patterns,
		MaxFiles:       100,                     // Warm up to 100 files
		MaxAge:         7 * 24 * time.Hour,     // Only warm files from last 7 days
		WorkerCount:    4,                       // Use 4 workers
		TimeoutPerFile: 5 * time.Second,         // 5 second timeout per file
	}

	// Start warming in background
	ctx := context.Background() // Use separate context so it continues after main ctx
	if err := ea.cacheWarmer.WarmupAsync(ctx, config); err != nil {
		ea.logger.Warnf("Failed to start cache warming: %v", err)
	} else {
		ea.logger.Info("Cache warming started in background")
	}
}

// GetMetricsCalculator returns the metrics calculator (for testing/debugging)
func (ea *EnhancedApplication) GetMetricsCalculator() *calculations.EnhancedMetricsCalculator {
	return ea.metricsCalc
}

// IsRunning returns whether the application is currently running
func (ea *EnhancedApplication) IsRunning() bool {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.running
}
