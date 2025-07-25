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
	ui           *ui.App
	errorHandler *errors.EnhancedErrorHandler
	
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	
	metrics     *Metrics
	logger      *Logger
	
	// Application state
	running     bool
	mu          sync.RWMutex
}

// NewEnhancedApplication creates a new enhanced application instance
func NewEnhancedApplication(cfg *config.Config) (*EnhancedApplication, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	app := &EnhancedApplication{
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
		logger:       NewLogger(cfg.App.LogLevel),
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
	// Initialize cache
	ea.cache = cache.NewStore(cache.StoreConfig{
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		MaxMemory:         50 * 1024 * 1024, // 50MB
		FileCacheTTL:      24 * time.Hour,
		CalcCacheTTL:      1 * time.Hour,
		CompressionLevel:  6,
		EnableMetrics:     true,
		EnableCompression: true,
	})
	
	// Initialize metrics calculator
	ea.metricsCalc = calculations.NewEnhancedMetricsCalculator(ea.config)
	
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
		RefreshRate:   ea.config.UI.RefreshRate,
		Theme:         ea.config.UI.Theme,
		ShowSpinner:   true,
		CompactMode:   ea.config.UI.CompactMode,
		ChartHeight:   10,
		TablePageSize: 20,
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
	
	fmt.Printf("=== DATA UPDATE CALLBACK ===\n")
	fmt.Printf("Received %d blocks from orchestrator\n", len(data.Data.Blocks))
	
	// Update metrics calculator with new session blocks
	ea.metricsCalc.UpdateSessionBlocks(data.Data.Blocks)
	
	// Calculate enhanced metrics
	metrics := ea.metricsCalc.Calculate()
	fmt.Printf("Calculated metrics - Current tokens: %d, Current cost: $%.4f\n", 
		metrics.CurrentTokens, metrics.CurrentCost)
	
	// Update UI if running interactively
	if ea.ui.IsRunning() {
		// Convert the data to the format expected by the UI
		sessions := ea.convertBlocksToSessions(data.Data.Blocks)
		entries := ea.extractEntriesFromBlocks(data.Data.Blocks)
		fmt.Printf("Updating UI with %d sessions and %d entries\n", len(sessions), len(entries))
		ea.ui.UpdateData(sessions, entries)
	} else {
		fmt.Printf("UI is not running - skipping UI update\n")
	}
	
	// Update application metrics
	ea.updateApplicationMetrics(metrics)
	
	ea.logger.Debugf("Processed data update with %d blocks", len(data.Data.Blocks))
	fmt.Printf("=== END DATA UPDATE ===\n")
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
		return ea.config.Data.Paths[0]
	}
	
	// Use real test data path instead of default
	testDataPath := "/Users/penwyp/Dat/worktree/claude_data_snapshot/projects"
	if _, err := os.Stat(testDataPath); err == nil {
		ea.logger.Infof("Using test data path: %s", testDataPath)
		return testDataPath
	}
	
	// Fallback to default data path
	homeDir, _ := os.UserHomeDir()
	defaultPath := fmt.Sprintf("%s/.claude/projects", homeDir)
	ea.logger.Infof("Using default data path: %s", defaultPath)
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