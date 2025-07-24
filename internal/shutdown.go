package internal

import (
	"context"
	"fmt"
	"time"
)

// shutdown performs graceful shutdown of all application components
func (a *Application) shutdown() error {
	a.logger.Info("Initiating graceful shutdown...")
	
	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Update running state
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	
	// Stop components in reverse order of initialization
	shutdownSteps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Metrics Server", a.stopMetrics},
		{"UI", a.stopUI},
		{"File Watcher", a.stopFileWatcher},
		{"Session Manager", a.stopSessionManager},
		{"Cache", a.stopCache},
	}
	
	var errs []error
	for _, step := range shutdownSteps {
		a.logger.Infof("Stopping %s...", step.name)
		if err := step.fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", step.name, err))
			a.logger.Errorf("Failed to stop %s: %v", step.name, err)
		} else {
			a.logger.Infof("%s stopped successfully", step.name)
		}
	}
	
	// Wait for context timeout or completion
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			a.logger.Warn("Shutdown timeout exceeded, forcing exit")
		}
	default:
		a.logger.Info("All components stopped within timeout")
	}
	
	// Aggregate errors
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	
	a.logger.Info("Graceful shutdown completed")
	return nil
}

// stopMetrics stops the metrics collection
func (a *Application) stopMetrics(ctx context.Context) error {
	if a.metrics == nil {
		return nil
	}
	
	return a.metrics.Stop()
}

// stopUI stops the user interface
func (a *Application) stopUI(ctx context.Context) error {
	if a.ui == nil {
		return nil
	}
	
	// Stop the UI
	return a.ui.Stop()
}

// stopFileWatcher stops the file watcher
func (a *Application) stopFileWatcher(ctx context.Context) error {
	if a.fileWatcher == nil {
		return nil
	}
	
	return a.fileWatcher.Stop()
}

// stopSessionManager stops the session manager
func (a *Application) stopSessionManager(ctx context.Context) error {
	if a.manager == nil {
		return nil
	}
	
	// Session manager doesn't need explicit stopping
	return nil
}

// stopCache stops the cache
func (a *Application) stopCache(ctx context.Context) error {
	if a.cache == nil {
		return nil
	}
	
	// Cache doesn't need explicit stopping
	return nil
}

// emergencyShutdown performs emergency shutdown without timeouts
func (a *Application) emergencyShutdown() {
	a.logger.Warn("Performing emergency shutdown")
	
	// Force stop all components immediately
	if a.fileWatcher != nil {
		a.fileWatcher.Stop()
	}
	
	if a.ui != nil {
		a.ui.Stop()
	}
	
	if a.metrics != nil {
		a.metrics.Stop()
	}
	
	a.logger.Warn("Emergency shutdown completed")
}