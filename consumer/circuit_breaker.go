// Package main provides circuit breaker functionality for resilient service communication.
// This module implements the circuit breaker pattern to prevent cascading failures
// when communicating with external services like the liquidity service.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int32

const (
	StateClosed   CircuitState = iota // Normal operation, requests pass through
	StateHalfOpen                     // Testing if service has recovered
	StateOpen                         // Circuit is open, requests fail fast
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyFailures = errors.New("too many consecutive failures")
)

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
// It prevents cascading failures by temporarily stopping requests to failing services.
type CircuitBreaker struct {
	name              string        // Circuit breaker identifier
	maxFailures       int32         // Threshold for opening circuit
	resetTimeout      time.Duration // Time before attempting reset
	halfOpenSuccess   int32         // Successes needed in half-open state
	state             int32         // Current circuit state (atomic)
	failures          int32         // Consecutive failure count (atomic)
	lastFailureTime   int64         // Last failure timestamp (atomic)
	halfOpenSuccesses int32         // Success count in half-open state (atomic)
	mu                sync.RWMutex  // Protects non-atomic fields
}

// NewCircuitBreaker creates a new circuit breaker with specified parameters.
// The circuit starts in the closed state, allowing requests to pass through.
func NewCircuitBreaker(name string, maxFailures int32, resetTimeout time.Duration, halfOpenSuccess int32) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		maxFailures:     maxFailures,
		resetTimeout:    resetTimeout,
		halfOpenSuccess: halfOpenSuccess,
		state:           int32(StateClosed),
		failures:        0,
		lastFailureTime: 0,
	}
}

// Call executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.canExecute() {
		return ErrCircuitOpen
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// canExecute checks if the circuit breaker allows execution based on current state.
// In closed state: allows execution. In open state: checks timeout for half-open transition.
// In half-open state: allows limited execution for testing service recovery.
func (cb *CircuitBreaker) canExecute() bool {
	state := CircuitState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should transition to half-open
		lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
		if time.Since(time.Unix(0, lastFailure)) > cb.resetTimeout {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateOpen), int32(StateHalfOpen)) {
				atomic.StoreInt32(&cb.halfOpenSuccesses, 0)
				fmt.Printf("[CircuitBreaker:%s] Transitioning from Open to Half-Open\n", cb.name)
			}
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordFailure records a failure and potentially opens the circuit.
// It increments failure count and transitions to open state if threshold exceeded.
func (cb *CircuitBreaker) recordFailure() {
	state := CircuitState(atomic.LoadInt32(&cb.state))
	failures := atomic.AddInt32(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())

	switch state {
	case StateClosed:
		if failures >= cb.maxFailures {
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateClosed), int32(StateOpen)) {
				fmt.Printf("[CircuitBreaker:%s] Opening circuit after %d failures\n", cb.name, failures)
			}
		}
	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateOpen)) {
			atomic.StoreInt32(&cb.failures, 0)
			fmt.Printf("[CircuitBreaker:%s] Reopening circuit from Half-Open after failure\n", cb.name)
		}
	}
}

// recordSuccess records a successful execution and handles state transitions.
// In closed state: resets failure count. In half-open state: counts successes
// and transitions back to closed when threshold is reached.
func (cb *CircuitBreaker) recordSuccess() {
	state := CircuitState(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		// Reset failure count on success
		atomic.StoreInt32(&cb.failures, 0)
	case StateHalfOpen:
		successes := atomic.AddInt32(&cb.halfOpenSuccesses, 1)
		if successes >= cb.halfOpenSuccess {
			// Transition back to closed
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateClosed)) {
				atomic.StoreInt32(&cb.failures, 0)
				atomic.StoreInt32(&cb.halfOpenSuccesses, 0)
				fmt.Printf("[CircuitBreaker:%s] Closing circuit after %d successes in Half-Open\n", cb.name, successes)
			}
		}
	}
}

// GetState returns the current state of the circuit breaker.
// Returns one of: StateClosed, StateHalfOpen, or StateOpen.
func (cb *CircuitBreaker) GetState() CircuitState {
	return CircuitState(atomic.LoadInt32(&cb.state))
}

// GetFailureCount returns the current failure count.
// This is the number of consecutive failures that have occurred.
func (cb *CircuitBreaker) GetFailureCount() int32 {
	return atomic.LoadInt32(&cb.failures)
}

// RetryConfig holds configuration for retry with backoff and circuit breaker.
// It defines retry parameters and optionally includes a circuit breaker for fault tolerance.
type RetryConfig struct {
	MaxAttempts    int             // Maximum number of retry attempts
	InitialDelay   time.Duration   // Initial delay between retries
	MaxDelay       time.Duration   // Maximum delay between retries
	Multiplier     float64         // Exponential backoff multiplier
	CircuitBreaker *CircuitBreaker // Optional circuit breaker for fault tolerance
}

// DefaultRetryConfig returns a retry config with sensible defaults.
// It includes a circuit breaker configured for typical service communication patterns.
func DefaultRetryConfig(name string) RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
		CircuitBreaker: NewCircuitBreaker(
			name,
			5,              // maxFailures
			30*time.Second, // resetTimeout
			2,              // halfOpenSuccess
		),
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check circuit breaker if configured
		if config.CircuitBreaker != nil {
			if !config.CircuitBreaker.canExecute() {
				return fmt.Errorf("circuit breaker open: %w", ErrCircuitOpen)
			}
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success - record with circuit breaker
			if config.CircuitBreaker != nil {
				config.CircuitBreaker.recordSuccess()
			}
			return nil
		}

		lastErr = err

		// Record failure with circuit breaker
		if config.CircuitBreaker != nil {
			config.CircuitBreaker.recordFailure()
		}

		// Don't retry if we've exhausted attempts
		if attempt >= config.MaxAttempts {
			break
		}

		// Log retry attempt
		fmt.Printf("[Retry] Attempt %d/%d failed: %v, retrying in %v\n",
			attempt, config.MaxAttempts, err, delay)

		// Wait with backoff, respecting context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxAttempts, lastErr)
}
