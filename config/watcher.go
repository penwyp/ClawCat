package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches configuration files for changes and reloads them
type Watcher struct {
	path      string
	config    *Config
	loader    *Loader
	onChange  func(*Config)
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
	mu        sync.RWMutex
	debouncer *debouncer
}

// NewWatcher creates a new configuration file watcher
func NewWatcher(path string, onChange func(*Config)) (*Watcher, error) {
	// Expand environment variables in path
	expandedPath := os.ExpandEnv(path)

	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create loader for reloading configuration
	loader := NewLoader()
	loader.AddSource(NewFileSource(expandedPath))
	loader.AddValidator(NewStandardValidator())

	w := &Watcher{
		path:      expandedPath,
		loader:    loader,
		onChange:  onChange,
		watcher:   fsWatcher,
		stopCh:    make(chan struct{}),
		debouncer: newDebouncer(500 * time.Millisecond),
	}

	return w, nil
}

// Start starts watching the configuration file
func (w *Watcher) Start() error {
	// Load initial configuration
	cfg, err := w.loader.LoadWithDefaults()
	if err != nil {
		return fmt.Errorf("failed to load initial configuration: %w", err)
	}

	w.mu.Lock()
	w.config = cfg
	w.mu.Unlock()

	// Watch the file and its directory
	if err := w.addWatches(); err != nil {
		return fmt.Errorf("failed to add file watches: %w", err)
	}

	// Start event processing goroutine
	go w.processEvents()

	return nil
}

// Stop stops watching the configuration file
func (w *Watcher) Stop() error {
	close(w.stopCh)
	w.debouncer.stop()
	return w.watcher.Close()
}

// Current returns the current configuration
func (w *Watcher) Current() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// addWatches adds file system watches
func (w *Watcher) addWatches() error {
	// Watch the config file if it exists
	if _, err := os.Stat(w.path); err == nil {
		if err := w.watcher.Add(w.path); err != nil {
			return fmt.Errorf("failed to watch config file %s: %w", w.path, err)
		}
	}

	// Watch the directory containing the config file
	dir := filepath.Dir(w.path)
	if err := w.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch config directory %s: %w", dir, err)
	}

	return nil
}

// processEvents processes file system events
func (w *Watcher) processEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("config watcher error: %v", err)

		case <-w.stopCh:
			return
		}
	}
}

// handleEvent handles a single file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// We're interested in the config file
	if event.Name != w.path {
		return
	}

	// Debounce rapid successive events
	w.debouncer.debounce(func() {
		w.reloadConfig()
	})
}

// reloadConfig reloads the configuration from file
func (w *Watcher) reloadConfig() {
	// Check if file still exists
	if _, err := os.Stat(w.path); os.IsNotExist(err) {
		log.Printf("config file deleted: %s", w.path)
		return
	}

	// Reload configuration
	cfg, err := w.loader.LoadWithDefaults()
	if err != nil {
		log.Printf("failed to reload configuration: %v", err)
		return
	}

	// Update stored configuration
	w.mu.Lock()
	oldConfig := w.config
	w.config = cfg
	w.mu.Unlock()

	// Check if configuration actually changed
	if !w.configChanged(oldConfig, cfg) {
		return
	}

	// Notify about the change
	if w.onChange != nil {
		w.onChange(cfg)
	}

	log.Printf("configuration reloaded from %s", w.path)
}

// configChanged checks if configuration has meaningfully changed
func (w *Watcher) configChanged(old, new *Config) bool {
	if old == nil || new == nil {
		return true
	}

	// Simple comparison - in a real implementation, you might want
	// to do a more sophisticated comparison
	return fmt.Sprintf("%+v", old) != fmt.Sprintf("%+v", new)
}

// debouncer helps debounce rapid successive events
type debouncer struct {
	delay    time.Duration
	timer    *time.Timer
	callback func()
	mu       sync.Mutex
}

// newDebouncer creates a new debouncer
func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		delay: delay,
	}
}

// debounce debounces a function call
func (d *debouncer) debounce(callback func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.callback = callback
	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.callback != nil {
			d.callback()
		}
	})
}

// stop stops the debouncer
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.callback = nil
}

// MultiWatcher watches multiple configuration files
type MultiWatcher struct {
	watchers []*Watcher
	onChange func(*Config)
	stopCh   chan struct{}
}

// NewMultiWatcher creates a new multi-file configuration watcher
func NewMultiWatcher(paths []string, onChange func(*Config)) (*MultiWatcher, error) {
	mw := &MultiWatcher{
		watchers: make([]*Watcher, 0, len(paths)),
		onChange: onChange,
		stopCh:   make(chan struct{}),
	}

	// Create watchers for each path
	for _, path := range paths {
		watcher, err := NewWatcher(path, mw.onConfigChange)
		if err != nil {
			// Clean up already created watchers
			for _, w := range mw.watchers {
				w.Stop()
			}
			return nil, fmt.Errorf("failed to create watcher for %s: %w", path, err)
		}
		mw.watchers = append(mw.watchers, watcher)
	}

	return mw, nil
}

// Start starts all watchers
func (mw *MultiWatcher) Start() error {
	for _, watcher := range mw.watchers {
		if err := watcher.Start(); err != nil {
			// Stop already started watchers
			for _, w := range mw.watchers {
				w.Stop()
			}
			return fmt.Errorf("failed to start watcher: %w", err)
		}
	}
	return nil
}

// Stop stops all watchers
func (mw *MultiWatcher) Stop() error {
	close(mw.stopCh)
	
	var lastErr error
	for _, watcher := range mw.watchers {
		if err := watcher.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Current returns the current merged configuration
func (mw *MultiWatcher) Current() *Config {
	// Create a loader with all sources
	loader := NewLoader()
	for _, watcher := range mw.watchers {
		if watcher.Current() != nil {
			// Add file source for each watcher
			loader.AddSource(NewFileSource(watcher.path))
		}
	}

	cfg, err := loader.LoadWithDefaults()
	if err != nil {
		log.Printf("failed to get current multi-watcher config: %v", err)
		return DefaultConfig()
	}

	return cfg
}

// onConfigChange handles configuration changes from individual watchers
func (mw *MultiWatcher) onConfigChange(*Config) {
	// Get the merged current configuration
	cfg := mw.Current()
	
	// Notify about the change
	if mw.onChange != nil {
		mw.onChange(cfg)
	}
}