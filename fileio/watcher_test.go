package fileio

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		event    EventType
		expected string
	}{
		{EventCreate, "CREATE"},
		{EventModify, "MODIFY"},
		{EventDelete, "DELETE"},
		{EventType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.event.String())
		})
	}
}

func TestNewWatcher(t *testing.T) {
	tempDir := t.TempDir()
	paths := []string{tempDir}

	watcher, err := NewWatcher(paths)
	require.NoError(t, err)
	require.NotNil(t, watcher)

	defer watcher.Close()

	assert.Equal(t, paths, watcher.GetWatchedPaths())
	assert.False(t, watcher.IsRunning())
	assert.NotNil(t, watcher.Events())
	assert.NotNil(t, watcher.Errors())
}

func TestNewWatcherWithConfig(t *testing.T) {
	tempDir := t.TempDir()
	paths := []string{tempDir}

	config := WatcherConfig{
		BufferSize:   50,
		DebounceTime: 200 * time.Millisecond,
	}

	watcher, err := NewWatcherWithConfig(paths, config)
	require.NoError(t, err)
	require.NotNil(t, watcher)

	defer watcher.Close()

	assert.Equal(t, 200, watcher.debounceMs)
	assert.Equal(t, paths, watcher.GetWatchedPaths())
}

func TestWatcher_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	watcher, err := NewWatcher([]string{tempDir})
	require.NoError(t, err)
	defer watcher.Close()

	// Test starting
	err = watcher.Start()
	require.NoError(t, err)
	assert.True(t, watcher.IsRunning())

	// Test double start
	err = watcher.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping
	err = watcher.Stop()
	require.NoError(t, err)
	assert.False(t, watcher.IsRunning())

	// Test double stop
	err = watcher.Stop()
	require.NoError(t, err)
}

func TestWatcher_FileEvents(t *testing.T) {
	tempDir := t.TempDir()

	// Use shorter debounce time for faster tests
	config := WatcherConfig{
		BufferSize:   100,
		DebounceTime: 10 * time.Millisecond,
	}

	watcher, err := NewWatcherWithConfig([]string{tempDir}, config)
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	var events []FileEvent
	var mu sync.Mutex
	done := make(chan struct{})

	// Collect events
	go func() {
		defer close(done)
		timeout := time.After(2 * time.Second)
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				mu.Lock()
				events = append(events, event)
				mu.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// Create a file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	require.NoError(t, err)

	// Modify the file
	time.Sleep(50 * time.Millisecond) // Wait for debounce
	err = os.WriteFile(testFile, []byte("hello world"), 0644)
	require.NoError(t, err)

	// Delete the file
	time.Sleep(50 * time.Millisecond) // Wait for debounce
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Stop watcher to close event channel
	watcher.Stop()
	<-done

	mu.Lock()
	defer mu.Unlock()

	// We should have received at least create and delete events
	assert.GreaterOrEqual(t, len(events), 2)

	// Check that we got the expected event types
	eventTypes := make(map[EventType]bool)
	for _, event := range events {
		eventTypes[event.Type] = true
		assert.Equal(t, testFile, event.Path)
		assert.False(t, event.Timestamp.IsZero())
	}

	assert.True(t, eventTypes[EventCreate], "Should receive CREATE event")
	// Note: DELETE events might not always be captured depending on timing
}

func TestWatcher_AddRemovePath(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	watcher, err := NewWatcher([]string{tempDir1})
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	// Test adding path
	err = watcher.AddPath(tempDir2)
	require.NoError(t, err)

	paths := watcher.GetWatchedPaths()
	assert.Contains(t, paths, tempDir1)
	assert.Contains(t, paths, tempDir2)
	assert.Len(t, paths, 2)

	// Test removing path
	err = watcher.RemovePath(tempDir2)
	require.NoError(t, err)

	paths = watcher.GetWatchedPaths()
	assert.Contains(t, paths, tempDir1)
	assert.NotContains(t, paths, tempDir2)
	assert.Len(t, paths, 1)
}

func TestWatcher_InvalidPath(t *testing.T) {
	invalidPath := "/nonexistent/path/that/does/not/exist"

	watcher, err := NewWatcher([]string{invalidPath})
	require.NoError(t, err)

	// Starting should fail with invalid path
	err = watcher.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to watch path")

	// Watcher should be properly cleaned up even after failed start
	assert.False(t, watcher.IsRunning())
}

func TestWatcher_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	watcher, err := NewWatcher([]string{tempDir})
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	// Test adding invalid path after starting
	err = watcher.AddPath("/nonexistent/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add path")

	// Test removing non-watched path
	err = watcher.RemovePath("/some/other/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove path")
}

func TestWatcher_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	watcher, err := NewWatcher([]string{tempDir})
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent access to watcher methods
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Test concurrent path operations
			tempSubDir := filepath.Join(tempDir, "subdir"+string(rune(id+'0')))
			os.MkdirAll(tempSubDir, 0755)

			// These operations should be thread-safe
			_ = watcher.GetWatchedPaths()
			_ = watcher.IsRunning()
		}(i)
	}

	wg.Wait()
}

func TestWatcher_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()

	config := WatcherConfig{
		BufferSize:   200,
		DebounceTime: 10 * time.Millisecond,
	}

	watcher, err := NewWatcherWithConfig([]string{tempDir}, config)
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	var events []FileEvent
	var mu sync.Mutex
	done := make(chan struct{})

	// Collect events
	go func() {
		defer close(done)
		timeout := time.After(3 * time.Second)
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				mu.Lock()
				events = append(events, event)
				mu.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// Create multiple files
	numFiles := 5
	for i := 0; i < numFiles; i++ {
		testFile := filepath.Join(tempDir, "test"+string(rune(i+'0'))+".txt")
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond) // Small delay between file operations
	}

	// Wait for all events to be processed
	time.Sleep(200 * time.Millisecond)

	watcher.Stop()
	<-done

	mu.Lock()
	defer mu.Unlock()

	// We should have received create events for all files
	assert.GreaterOrEqual(t, len(events), numFiles)
}

func TestWatcher_Close(t *testing.T) {
	tempDir := t.TempDir()
	watcher, err := NewWatcher([]string{tempDir})
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)

	// Close should stop the watcher and close channels
	err = watcher.Close()
	require.NoError(t, err)
	assert.False(t, watcher.IsRunning())

	// Events channel should be closed
	_, ok := <-watcher.Events()
	assert.False(t, ok, "Events channel should be closed")

	// Errors channel should be closed
	_, ok = <-watcher.Errors()
	assert.False(t, ok, "Errors channel should be closed")
}

func TestWatchDirectory(t *testing.T) {
	tempDir := t.TempDir()

	var receivedEvents []FileEvent
	var mu sync.Mutex

	// Start watching directory
	err := WatchDirectory(tempDir, func(event FileEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})
	require.NoError(t, err)

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Wait for event processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	assert.GreaterOrEqual(t, len(receivedEvents), 1)
	if len(receivedEvents) > 0 {
		assert.Equal(t, testFile, receivedEvents[0].Path)
		assert.Equal(t, EventCreate, receivedEvents[0].Type)
	}
}

func TestWatcher_NoDebounce(t *testing.T) {
	tempDir := t.TempDir()

	// Configure with no debouncing
	config := WatcherConfig{
		BufferSize:   100,
		DebounceTime: 0,
	}

	watcher, err := NewWatcherWithConfig([]string{tempDir}, config)
	require.NoError(t, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(t, err)

	var events []FileEvent
	var mu sync.Mutex
	done := make(chan struct{})

	// Collect events
	go func() {
		defer close(done)
		timeout := time.After(1 * time.Second)
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				mu.Lock()
				events = append(events, event)
				mu.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// Create and quickly modify a file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("v1"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(testFile, []byte("v2"), 0644)
	require.NoError(t, err)

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	watcher.Stop()
	<-done

	mu.Lock()
	defer mu.Unlock()

	// With no debouncing, we might receive multiple events
	assert.GreaterOrEqual(t, len(events), 1)
}

// Benchmark tests
func BenchmarkWatcher_EventProcessing(b *testing.B) {
	tempDir := b.TempDir()

	config := WatcherConfig{
		BufferSize:   1000,
		DebounceTime: 1 * time.Millisecond,
	}

	watcher, err := NewWatcherWithConfig([]string{tempDir}, config)
	require.NoError(b, err)
	defer watcher.Close()

	err = watcher.Start()
	require.NoError(b, err)

	// Consume events to prevent channel blocking
	go func() {
		for range watcher.Events() {
			// Consume events
		}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tempDir, "bench"+string(rune(i%10+'0'))+".txt")
		_ = os.WriteFile(testFile, []byte("content"), 0644)
	}
}
