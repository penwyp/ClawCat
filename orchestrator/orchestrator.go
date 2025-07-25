package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/penwyp/ClawCat/config"
	"github.com/penwyp/ClawCat/models"
	"github.com/penwyp/ClawCat/logging"
)

// MonitoringData represents the data structure passed to callbacks
type MonitoringData struct {
	Data        AnalysisResult `json:"data"`
	TokenLimit  int           `json:"token_limit"`
	Args        interface{}   `json:"args,omitempty"`
	SessionID   string        `json:"session_id"`
	SessionCount int          `json:"session_count"`
}

// AnalysisResult represents the processed analysis data
type AnalysisResult struct {
	Blocks    []models.SessionBlock `json:"blocks"`
	Metadata  AnalysisMetadata      `json:"metadata"`
}

// AnalysisMetadata contains metadata about the analysis
type AnalysisMetadata struct {
	GeneratedAt        time.Time `json:"generated_at"`
	HoursAnalyzed      string    `json:"hours_analyzed"`
	EntriesProcessed   int       `json:"entries_processed"`
	BlocksCreated      int       `json:"blocks_created"`
	LimitsDetected     int       `json:"limits_detected"`
	LoadTimeSeconds    float64   `json:"load_time_seconds"`
	TransformTimeSeconds float64 `json:"transform_time_seconds"`
	CacheUsed          bool      `json:"cache_used"`
	QuickStart         bool      `json:"quick_start"`
}

// DataUpdateCallback represents a callback function for data updates
type DataUpdateCallback func(MonitoringData)

// SessionChangeCallback represents a callback function for session changes
type SessionChangeCallback func(eventType, sessionID string, sessionData interface{})

// MonitoringOrchestrator orchestrates monitoring components following SRP
type MonitoringOrchestrator struct {
	updateInterval   time.Duration
	dataPath         string
	config           *config.Config
	
	// Internal components
	dataManager      *DataManager
	sessionMonitor   *SessionMonitor
	
	// State management
	monitoring       bool
	monitorThread    *Goroutine
	stopEvent        context.Context
	stopCancel       context.CancelFunc
	
	// Callbacks
	updateCallbacks  []DataUpdateCallback
	sessionCallbacks []SessionChangeCallback
	
	// Data tracking
	lastValidData    *MonitoringData
	firstDataEvent   chan struct{}
	
	// Args from CLI
	args             interface{}
	
	// Thread safety
	mu               sync.RWMutex
}

// NewMonitoringOrchestrator creates a new monitoring orchestrator
func NewMonitoringOrchestrator(updateInterval time.Duration, dataPath string, cfg *config.Config) *MonitoringOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MonitoringOrchestrator{
		updateInterval:   updateInterval,
		dataPath:         dataPath,
		config:           cfg,
		dataManager:      NewDataManager(5*time.Second, 192, dataPath), // 5-second TTL, 192 hours back
		sessionMonitor:   NewSessionMonitor(),
		monitoring:       false,
		stopEvent:        ctx,
		stopCancel:       cancel,
		updateCallbacks:  make([]DataUpdateCallback, 0),
		sessionCallbacks: make([]SessionChangeCallback, 0),
		firstDataEvent:   make(chan struct{}, 1),
	}
}

// Start begins monitoring
func (mo *MonitoringOrchestrator) Start() error {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	
	if mo.monitoring {
		return fmt.Errorf("monitoring already running")
	}
	
	mo.monitoring = true
	
	// Reset the stop context
	mo.stopEvent, mo.stopCancel = context.WithCancel(context.Background())
	
	// Start monitoring goroutine
	mo.monitorThread = &Goroutine{
		name: "MonitoringThread",
		fn:   mo.monitoringLoop,
		ctx:  mo.stopEvent,
	}
	
	go mo.monitorThread.Run()
	
	return nil
}

// Stop stops monitoring
func (mo *MonitoringOrchestrator) Stop() {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	
	if !mo.monitoring {
		return
	}
	
	mo.monitoring = false
	mo.stopCancel()
	
	// Wait for goroutine to finish with timeout
	if mo.monitorThread != nil {
		select {
		case <-mo.monitorThread.Done():
			// Goroutine finished
		case <-time.After(5 * time.Second):
			// Timeout waiting
		}
		mo.monitorThread = nil
	}
	
	// Clear first data event
	select {
	case <-mo.firstDataEvent:
	default:
	}
}

// SetArgs sets command line arguments for token limit calculation
func (mo *MonitoringOrchestrator) SetArgs(args interface{}) {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	mo.args = args
}

// RegisterUpdateCallback registers a callback for data updates
func (mo *MonitoringOrchestrator) RegisterUpdateCallback(callback DataUpdateCallback) {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	mo.updateCallbacks = append(mo.updateCallbacks, callback)
}

// RegisterSessionCallback registers a callback for session changes
func (mo *MonitoringOrchestrator) RegisterSessionCallback(callback SessionChangeCallback) {
	mo.mu.Lock()
	defer mo.mu.Unlock()
	mo.sessionCallbacks = append(mo.sessionCallbacks, callback)
}

// ForceRefresh forces immediate data refresh
func (mo *MonitoringOrchestrator) ForceRefresh() (*MonitoringData, error) {
	return mo.fetchAndProcessData(true)
}

// WaitForInitialData waits for initial data to be fetched
func (mo *MonitoringOrchestrator) WaitForInitialData(timeout time.Duration) bool {
	select {
	case <-mo.firstDataEvent:
		return true
	case <-time.After(timeout):
		return false
	}
}

// monitoringLoop is the main monitoring loop
func (mo *MonitoringOrchestrator) monitoringLoop() {
	// Initial fetch
	mo.fetchAndProcessData(false)
	
	ticker := time.NewTicker(mo.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mo.stopEvent.Done():
			return
		case <-ticker.C:
			mo.fetchAndProcessData(false)
		}
	}
}

// fetchAndProcessData fetches data and notifies callbacks
func (mo *MonitoringOrchestrator) fetchAndProcessData(forceRefresh bool) (*MonitoringData, error) {
	startTime := time.Now()
	
	// Fetch data using DataManager
	data, err := mo.dataManager.GetData(forceRefresh)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	
	if data == nil {
		return nil, fmt.Errorf("no data fetched")
	}
	
	// Validate and update session tracking
	isValid, errors := mo.sessionMonitor.Update(data)
	if !isValid {
		return nil, fmt.Errorf("data validation failed: %v", errors)
	}
	
	// Calculate token limit
	tokenLimit := mo.calculateTokenLimit(data)
	
	// Prepare monitoring data
	monitoringData := &MonitoringData{
		Data:         *data,
		TokenLimit:   tokenLimit,
		Args:         mo.args,
		SessionID:    mo.sessionMonitor.GetCurrentSessionID(),
		SessionCount: mo.sessionMonitor.GetSessionCount(),
	}
	
	// Store last valid data
	mo.mu.Lock()
	mo.lastValidData = monitoringData
	mo.mu.Unlock()
	
	// Signal that first data has been received
	select {
	case mo.firstDataEvent <- struct{}{}:
	default:
		// Channel already has data
	}
	
	// Notify callbacks
	mo.notifyCallbacks(*monitoringData)
	
	elapsed := time.Since(startTime)
	logging.LogInfof("Data processing completed in %.3fs", elapsed.Seconds())
	
	return monitoringData, nil
}

// calculateTokenLimit calculates token limit based on plan and data
func (mo *MonitoringOrchestrator) calculateTokenLimit(data *AnalysisResult) int {
	// This would implement the same logic as Claude Monitor's token limit calculation
	// For now, return a default value
	// TODO: Implement proper token limit calculation based on plan type
	return 500000 // Default token limit
}

// notifyCallbacks notifies all registered callbacks
func (mo *MonitoringOrchestrator) notifyCallbacks(data MonitoringData) {
	mo.mu.RLock()
	updateCallbacks := make([]DataUpdateCallback, len(mo.updateCallbacks))
	copy(updateCallbacks, mo.updateCallbacks)
	mo.mu.RUnlock()
	
	for _, callback := range updateCallbacks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LogErrorf("Callback panic: %v", r)
				}
			}()
			callback(data)
		}()
	}
}

// Goroutine represents a managed goroutine
type Goroutine struct {
	name string
	fn   func()
	ctx  context.Context
	done chan struct{}
}

// Run runs the goroutine
func (g *Goroutine) Run() {
	g.done = make(chan struct{})
	defer close(g.done)
	g.fn()
}

// Done returns a channel that closes when the goroutine is done
func (g *Goroutine) Done() <-chan struct{} {
	if g.done == nil {
		// If not started yet, return a channel that never closes
		return make(chan struct{})
	}
	return g.done
}