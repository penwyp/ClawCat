package errors

import (
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	state              State
	failures           int
	successes          int
	consecutiveFailures int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	maxFailures       int
	timeout           time.Duration
	successThreshold  int
	mu                sync.RWMutex
	
	// Callbacks
	onStateChange func(from, to State)
	onOpen        func()
	onHalfOpen    func()
	onClose       func()
}

// CircuitBreakerConfig contains configuration for the circuit breaker
type CircuitBreakerConfig struct {
	MaxFailures      int           `json:"max_failures"`
	Timeout          time.Duration `json:"timeout"`
	SuccessThreshold int           `json:"success_threshold"`
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 3
	}
	
	return &CircuitBreaker{
		state:            StateClosed,
		maxFailures:      config.MaxFailures,
		timeout:          config.Timeout,
		successThreshold: config.SuccessThreshold,
		lastStateChange:  time.Now(),
	}
}

// CanCall checks if the circuit breaker allows a call to proceed
func (cb *CircuitBreaker) CanCall() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == StateOpen && time.Since(cb.lastFailureTime) >= cb.timeout {
				cb.transitionTo(StateHalfOpen)
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.successes++
	cb.consecutiveFailures = 0
	
	switch cb.state {
	case StateHalfOpen:
		if cb.successes >= cb.successThreshold {
			cb.transitionTo(StateClosed)
		}
	case StateOpen:
		// Shouldn't happen, but reset if it does
		cb.transitionTo(StateClosed)
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures++
	cb.consecutiveFailures++
	cb.lastFailureTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		if cb.consecutiveFailures >= cb.maxFailures {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state should open the circuit
		cb.transitionTo(StateOpen)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return CircuitBreakerStats{
		State:               cb.state,
		Failures:            cb.failures,
		Successes:           cb.successes,
		ConsecutiveFailures: cb.consecutiveFailures,
		LastFailureTime:     cb.lastFailureTime,
		LastStateChange:     cb.lastStateChange,
	}
}

// CircuitBreakerStats contains statistics about the circuit breaker
type CircuitBreakerStats struct {
	State               State     `json:"state"`
	Failures            int       `json:"failures"`
	Successes           int       `json:"successes"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastFailureTime     time.Time `json:"last_failure_time"`
	LastStateChange     time.Time `json:"last_state_change"`
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	oldState := cb.state
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.consecutiveFailures = 0
	cb.lastFailureTime = time.Time{}
	cb.lastStateChange = time.Now()
	
	if oldState != StateClosed {
		cb.triggerStateChangeCallback(oldState, StateClosed)
	}
}

// SetOnStateChange sets a callback for state changes
func (cb *CircuitBreaker) SetOnStateChange(callback func(from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = callback
}

// SetOnOpen sets a callback for when the circuit opens
func (cb *CircuitBreaker) SetOnOpen(callback func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onOpen = callback
}

// SetOnHalfOpen sets a callback for when the circuit becomes half-open
func (cb *CircuitBreaker) SetOnHalfOpen(callback func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onHalfOpen = callback
}

// SetOnClose sets a callback for when the circuit closes
func (cb *CircuitBreaker) SetOnClose(callback func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onClose = callback
}

// transitionTo transitions the circuit breaker to a new state (caller must hold write lock)
func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}
	
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	
	// Reset success counter when transitioning to half-open
	if newState == StateHalfOpen {
		cb.successes = 0
	}
	
	// Trigger callbacks asynchronously to avoid blocking
	go cb.triggerStateChangeCallback(oldState, newState)
	
	switch newState {
	case StateOpen:
		go cb.triggerCallback(cb.onOpen)
	case StateHalfOpen:
		go cb.triggerCallback(cb.onHalfOpen)
	case StateClosed:
		go cb.triggerCallback(cb.onClose)
	}
}

// triggerStateChangeCallback triggers the state change callback
func (cb *CircuitBreaker) triggerStateChangeCallback(from, to State) {
	if cb.onStateChange != nil {
		cb.onStateChange(from, to)
	}
}

// triggerCallback triggers a generic callback with panic recovery
func (cb *CircuitBreaker) triggerCallback(callback func()) {
	if callback == nil {
		return
	}
	
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't let it crash the application
			// In a real implementation, you'd use a proper logger here
		}
	}()
	
	callback()
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.CanCall() {
		return ErrCircuitBreakerOpen
	}
	
	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	
	cb.RecordSuccess()
	return nil
}

// ErrCircuitBreakerOpen is returned when the circuit breaker is open
var ErrCircuitBreakerOpen = fmt.Errorf("circuit breaker is open")

// IsHealthy returns true if the circuit breaker is in a healthy state
func (cb *CircuitBreaker) IsHealthy() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return cb.state != StateOpen
}

// GetFailureRate returns the current failure rate as a percentage
func (cb *CircuitBreaker) GetFailureRate() float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	total := cb.failures + cb.successes
	if total == 0 {
		return 0.0
	}
	
	return float64(cb.failures) / float64(total) * 100.0
}

// TimeUntilRetry returns the time until the next retry attempt (for open state)
func (cb *CircuitBreaker) TimeUntilRetry() time.Duration {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if cb.state != StateOpen {
		return 0
	}
	
	elapsed := time.Since(cb.lastFailureTime)
	if elapsed >= cb.timeout {
		return 0
	}
	
	return cb.timeout - elapsed
}