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
	"github.com/penwyp/ClawCat/fileio"
	"github.com/penwyp/ClawCat/logging"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/sessions"
	"github.com/penwyp/ClawCat/ui"
)

// Application represents the main application orchestrator
type Application struct {
	config      *config.Config
	manager     *sessions.Manager
	fileWatcher *fileio.Watcher
	calculator  *calculations.CostCalculator
	cache       *cache.Store
	ui          *ui.App

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	metrics *Metrics
	logger  logging.LoggerInterface

	// Application state
	running bool
	mu      sync.RWMutex
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) (*Application, error) {
	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
		logger: logging.NewLogger(cfg.App.LogLevel, cfg.App.LogFile),
	}

	if err := app.bootstrap(); err != nil {
		cancel()
		return nil, fmt.Errorf("bootstrap failed: %w", err)
	}

	return app, nil
}

// Run starts the application and blocks until shutdown
func (a *Application) Run() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	a.running = true
	a.mu.Unlock()

	a.logger.Info("Starting ClawCat application")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Start all components
	if err := a.start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Handle signals in a separate goroutine
	a.wg.Add(1)
	go a.handleSignals(sigCh)

	// Start the UI (this blocks until the UI exits)
	var err error
	if a.config.UI.CompactMode {
		err = a.runBackground()
	} else {
		err = a.runInteractive()
	}

	// Signal shutdown
	a.cancel()

	// Wait for all goroutines to finish
	a.wg.Wait()

	// Perform cleanup
	if shutdownErr := a.shutdown(); shutdownErr != nil {
		a.logger.Errorf("Shutdown error: %v", shutdownErr)
		if err == nil {
			err = shutdownErr
		}
	}

	a.logger.Info("ClawCat application stopped")
	return err
}

// start initializes and starts all application components
func (a *Application) start() error {
	a.logger.Info("Starting application components")

	// Cache is already initialized in bootstrap, no start method needed

	// Start file watcher
	if a.config.Data.AutoDiscover {
		if err := a.fileWatcher.Start(); err != nil {
			return fmt.Errorf("failed to start file watcher: %w", err)
		}

		// Process watched file events
		a.wg.Add(1)
		go a.processFileEvents()
	}

	// Session manager is already initialized, no start method needed

	// Start metrics collection if enabled
	if a.config.Debug.MetricsPort > 0 {
		a.metrics = NewMetrics(a.config.Debug.MetricsPort)
		a.wg.Add(1)
		go a.collectMetrics()
	}

	// Initial data load
	if err := a.loadInitialData(); err != nil {
		return fmt.Errorf("failed to load initial data: %w", err)
	}

	return nil
}

// runInteractive starts the TUI application
func (a *Application) runInteractive() error {
	a.logger.Info("Starting interactive TUI mode")

	// Set data source for the UI
	a.ui.SetDataSource(a.manager)

	// Start the UI application
	return a.ui.Start()
}

// runBackground runs in background mode without TUI
func (a *Application) runBackground() error {
	a.logger.Info("Starting background mode")

	// In background mode, just wait for context cancellation
	<-a.ctx.Done()
	return nil
}

// processFileEvents processes file change events from the watcher
func (a *Application) processFileEvents() {
	defer a.wg.Done()

	events := a.fileWatcher.Events()
	for {
		select {
		case event := <-events:
			if err := a.handleFileEvent(event); err != nil {
				a.logger.Errorf("Error processing file event: %v", err)
			}

		case <-a.ctx.Done():
			return
		}
	}
}

// handleFileEvent processes a single file change event
func (a *Application) handleFileEvent(event fileio.FileEvent) error {
	switch event.Type {
	case fileio.EventModify, fileio.EventCreate:
		// Read and process new entries
		entries, err := a.readFileEntries(event.Path)
		if err != nil {
			return fmt.Errorf("failed to read file entries: %w", err)
		}

		// Update sessions with new entries
		for _, entry := range entries {
			if err := a.manager.AddEntry(entry); err != nil {
				a.logger.Errorf("Failed to add entry to session: %v", err)
			}
		}

		// Update UI if running interactively (would send a refresh message to UI)

	case fileio.EventDelete:
		a.logger.Infof("File deleted: %s", event.Path)
		// Handle file deletion if needed
	}

	return nil
}

// readFileEntries reads usage entries from a file
func (a *Application) readFileEntries(filePath string) ([]models.UsageEntry, error) {
	reader, err := fileio.NewReader(filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var entries []models.UsageEntry
	entryCh, errCh := reader.ReadEntries()

	for {
		select {
		case entry, ok := <-entryCh:
			if !ok {
				// Channel closed, we're done
				return entries, nil
			}

			// Calculate cost and total tokens
			pricing := models.GetPricing(entry.Model)
			entry.CostUSD = entry.CalculateCost(pricing)
			entry.TotalTokens = entry.CalculateTotalTokens()

			entries = append(entries, entry)

		case err := <-errCh:
			if err != nil {
				return entries, fmt.Errorf("error reading entries: %w", err)
			}

		case <-a.ctx.Done():
			return entries, a.ctx.Err()
		}
	}
}

// loadInitialData loads initial data from configured paths
func (a *Application) loadInitialData() error {
	a.logger.Info("Loading initial data")

	for _, path := range a.config.Data.Paths {
		files, err := fileio.DiscoverFiles(path)
		if err != nil {
			a.logger.Errorf("Failed to discover files in %s: %v", path, err)
			continue
		}

		for _, file := range files {
			entries, err := a.readFileEntries(file)
			if err != nil {
				a.logger.Errorf("Failed to read file %s: %v", file, err)
				continue
			}

			// Add entries to session manager
			for _, entry := range entries {
				if err := a.manager.AddEntry(entry); err != nil {
					a.logger.Errorf("Failed to add entry to session: %v", err)
				}
			}
		}

		// Add path to file watcher if watching is enabled
		if a.config.Data.AutoDiscover {
			if err := a.fileWatcher.AddPath(path); err != nil {
				a.logger.Errorf("Failed to watch path %s: %v", path, err)
			}
		}
	}

	a.logger.Infof("Loaded data from %d paths", len(a.config.Data.Paths))
	return nil
}

// handleSignals handles OS signals
func (a *Application) handleSignals(sigCh <-chan os.Signal) {
	defer a.wg.Done()

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				a.logger.Info("Received shutdown signal")
				a.cancel()
				return

			case syscall.SIGHUP:
				a.logger.Info("Received SIGHUP, reloading configuration")
				if err := a.reloadConfig(); err != nil {
					a.logger.Errorf("Failed to reload config: %v", err)
				}
			}

		case <-a.ctx.Done():
			return
		}
	}
}

// reloadConfig reloads the configuration
func (a *Application) reloadConfig() error {
	// This would be implemented to hot-reload configuration
	a.logger.Info("Configuration reload not implemented yet")
	return nil
}

// collectMetrics collects and exports metrics
func (a *Application) collectMetrics() {
	defer a.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.updateMetrics()
			if a.metrics != nil {
				a.metrics.Export()
			}

		case <-a.ctx.Done():
			return
		}
	}
}

// updateMetrics updates current metrics
func (a *Application) updateMetrics() {
	if a.metrics == nil {
		return
	}

	// Update session metrics
	sessions := a.manager.GetAllActiveSessions()
	a.metrics.ActiveSessions = len(sessions)

	// Calculate totals
	var totalTokens int64
	var totalCost float64

	for _, session := range sessions {
		totalTokens += int64(session.Stats.TotalTokens)
		totalCost += session.Stats.TotalCost
	}

	a.metrics.TotalTokens = totalTokens
	a.metrics.TotalCost = totalCost
}

// GetManager returns the session manager
func (a *Application) GetManager() *sessions.Manager {
	return a.manager
}

// GetCalculator returns the calculator
func (a *Application) GetCalculator() *calculations.CostCalculator {
	return a.calculator
}

// GetCache returns the cache store
func (a *Application) GetCache() *cache.Store {
	return a.cache
}

// IsRunning returns whether the application is currently running
func (a *Application) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
