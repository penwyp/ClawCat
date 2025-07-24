package fileio

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of file system event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
)

// String returns a string representation of the event type
func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "CREATE"
	case EventModify:
		return "MODIFY"
	case EventDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Type      EventType
	Timestamp time.Time
}

// Watcher monitors file system changes using fsnotify
type Watcher struct {
	watcher          *fsnotify.Watcher
	paths            []string
	events           chan FileEvent
	errors           chan error
	stopCh           chan struct{}
	mu               sync.RWMutex
	running          bool
	debounceMs       int
	debounceMap      map[string]*time.Timer
	debounceChannels map[string]chan fsnotify.Event
}

// WatcherConfig holds configuration for the file watcher
type WatcherConfig struct {
	BufferSize   int           // Event buffer size
	DebounceTime time.Duration // Debounce duration for file events
}

// DefaultWatcherConfig returns default configuration
var DefaultWatcherConfig = WatcherConfig{
	BufferSize:   100,
	DebounceTime: 100 * time.Millisecond,
}

// NewWatcher creates a new file system watcher
func NewWatcher(paths []string) (*Watcher, error) {
	return NewWatcherWithConfig(paths, DefaultWatcherConfig)
}

// NewWatcherWithConfig creates a new file system watcher with custom configuration
func NewWatcherWithConfig(paths []string, config WatcherConfig) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		watcher:          fsWatcher,
		paths:            make([]string, len(paths)),
		events:           make(chan FileEvent, config.BufferSize),
		errors:           make(chan error, config.BufferSize),
		stopCh:           make(chan struct{}),
		debounceMs:       int(config.DebounceTime.Milliseconds()),
		debounceMap:      make(map[string]*time.Timer),
		debounceChannels: make(map[string]chan fsnotify.Event),
	}

	copy(w.paths, paths)
	return w, nil
}

// Start begins watching the configured paths
func (w *Watcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("watcher is already running")
	}

	// Add all paths to the watcher
	for _, path := range w.paths {
		if err := w.watcher.Add(path); err != nil {
			// Clean up by closing the underlying watcher, but don't call Close()
			// which would try to acquire the same lock
			w.watcher.Close()
			return fmt.Errorf("failed to watch path %s: %w", path, err)
		}
	}

	w.running = true

	// Start the event processing goroutine
	go w.processEvents()

	return nil
}

// Stop stops the file watcher
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	close(w.stopCh)
	w.running = false

	return w.watcher.Close()
}

// Close stops the watcher and closes all channels
func (w *Watcher) Close() error {
	err := w.Stop()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel any pending debounce timers
	for _, timer := range w.debounceMap {
		timer.Stop()
	}

	// Close channels
	close(w.events)
	close(w.errors)

	return err
}

// Events returns the channel for file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Errors returns the channel for errors
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// AddPath adds a new path to watch
func (w *Watcher) AddPath(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watcher.Add(path); err != nil {
		return fmt.Errorf("failed to add path %s: %w", path, err)
	}

	w.paths = append(w.paths, path)
	return nil
}

// RemovePath removes a path from watching
func (w *Watcher) RemovePath(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watcher.Remove(path); err != nil {
		return fmt.Errorf("failed to remove path %s: %w", path, err)
	}

	// Remove from paths slice
	for i, p := range w.paths {
		if p == path {
			w.paths = append(w.paths[:i], w.paths[i+1:]...)
			break
		}
	}

	// Clean up debouncing for this path
	if timer, exists := w.debounceMap[path]; exists {
		timer.Stop()
		delete(w.debounceMap, path)
	}
	if ch, exists := w.debounceChannels[path]; exists {
		close(ch)
		delete(w.debounceChannels, path)
	}

	return nil
}

// GetWatchedPaths returns a copy of the currently watched paths
func (w *Watcher) GetWatchedPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	paths := make([]string, len(w.paths))
	copy(paths, w.paths)
	return paths
}

// IsRunning returns whether the watcher is currently running
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// processEvents handles the main event processing loop
func (w *Watcher) processEvents() {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

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
			select {
			case w.errors <- err:
			case <-w.stopCh:
				return
			default:
				// Drop error if channel is full
			}

		case <-w.stopCh:
			return
		}
	}
}

// handleEvent processes individual fsnotify events with debouncing
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Check if we need to debounce this event
	if w.debounceMs > 0 {
		w.debounceEvent(event)
		return
	}

	// No debouncing, send event immediately
	w.sendEvent(event)
}

// debounceEvent implements event debouncing to avoid duplicate events
func (w *Watcher) debounceEvent(event fsnotify.Event) {
	path := event.Name

	w.mu.Lock()
	defer w.mu.Unlock()

	// Cancel existing timer for this path
	if timer, exists := w.debounceMap[path]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[path] = time.AfterFunc(time.Duration(w.debounceMs)*time.Millisecond, func() {
		w.mu.Lock()
		delete(w.debounceMap, path)
		w.mu.Unlock()
		w.sendEvent(event)
	})
}

// sendEvent converts fsnotify event to FileEvent and sends it
func (w *Watcher) sendEvent(event fsnotify.Event) {
	var eventType EventType

	if event.Op&fsnotify.Create == fsnotify.Create {
		eventType = EventCreate
	} else if event.Op&fsnotify.Write == fsnotify.Write {
		eventType = EventModify
	} else if event.Op&fsnotify.Remove == fsnotify.Remove {
		eventType = EventDelete
	} else {
		// Skip other event types (CHMOD, etc.)
		return
	}

	fileEvent := FileEvent{
		Path:      event.Name,
		Type:      eventType,
		Timestamp: time.Now(),
	}

	select {
	case w.events <- fileEvent:
	case <-w.stopCh:
		return
	default:
		// Drop event if channel is full
	}
}

// WatchDirectory is a convenience function to watch a single directory
func WatchDirectory(dir string, callback func(FileEvent)) error {
	watcher, err := NewWatcher([]string{dir})
	if err != nil {
		return err
	}

	if err := watcher.Start(); err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				callback(event)
			case err, ok := <-watcher.Errors():
				if !ok {
					return
				}
				// Log error or handle as needed
				_ = err
			}
		}
	}()

	return nil
}
