package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Metrics contains application metrics
type Metrics struct {
	// Application metrics
	StartTime       time.Time `json:"start_time"`
	ProcessedFiles  int64     `json:"processed_files"`
	ProcessedBytes  int64     `json:"processed_bytes"`
	ActiveSessions  int       `json:"active_sessions"`
	
	// Performance metrics
	CPUUsage        float64 `json:"cpu_usage"`
	MemoryUsage     uint64  `json:"memory_usage"`
	GoroutineCount  int     `json:"goroutine_count"`
	
	// Business metrics
	TotalTokens     int64   `json:"total_tokens"`
	TotalCost       float64 `json:"total_cost"`
	ErrorCount      int64   `json:"error_count"`
	
	// Internal
	server *http.Server
	port   int
	mu     sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics(port int) *Metrics {
	m := &Metrics{
		StartTime: time.Now(),
		port:      port,
	}
	
	// Start HTTP server for metrics endpoint
	m.startServer()
	
	return m
}

// startServer starts the metrics HTTP server
func (m *Metrics) startServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", m.handleMetrics)
	mux.HandleFunc("/health", m.handleHealth)
	
	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}
	
	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash the application
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()
}

// handleMetrics handles the metrics endpoint
func (m *Metrics) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Update runtime metrics
	m.updateRuntimeMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleHealth handles the health check endpoint
func (m *Metrics) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// updateRuntimeMetrics updates runtime-specific metrics
func (m *Metrics) updateRuntimeMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	m.MemoryUsage = memStats.Alloc
	m.GoroutineCount = runtime.NumGoroutine()
}

// IncrementProcessedFiles increments the processed files counter
func (m *Metrics) IncrementProcessedFiles() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessedFiles++
}

// AddProcessedBytes adds to the processed bytes counter
func (m *Metrics) AddProcessedBytes(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessedBytes += bytes
}

// IncrementErrorCount increments the error counter
func (m *Metrics) IncrementErrorCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorCount++
}

// UpdateActiveSession updates the active session count
func (m *Metrics) UpdateActiveSessions(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveSessions = count
}

// UpdateTotalTokens updates the total tokens counter
func (m *Metrics) UpdateTotalTokens(tokens int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalTokens = tokens
}

// UpdateTotalCost updates the total cost
func (m *Metrics) UpdateTotalCost(cost float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalCost = cost
}

// Export exports current metrics (placeholder for future metric exporters)
func (m *Metrics) Export() {
	// This could be extended to export to various systems like:
	// - Prometheus
	// - StatsD
	// - CloudWatch
	// - etc.
}

// Stop stops the metrics server
func (m *Metrics) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// ForceStop forcibly stops the metrics server
func (m *Metrics) ForceStop() {
	if m.server != nil {
		m.server.Close()
	}
}