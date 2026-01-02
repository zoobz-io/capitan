// Package testing provides test utilities and helpers for capitan users.
// These utilities help users test their own capitan-based applications.
package testing

import (
	"context"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
)

// CapturedEvent represents a snapshot of an event for testing.
// Stores event data to avoid issues with event pooling.
type CapturedEvent struct {
	Signal    capitan.Signal
	Timestamp time.Time
	Severity  capitan.Severity
	Fields    []capitan.Field
}

// EventCapture captures events for testing and verification.
// Thread-safe for concurrent event capture.
// Stores copies of event data to avoid event pooling issues.
type EventCapture struct {
	events []CapturedEvent
	mu     sync.Mutex
}

// NewEventCapture creates a new EventCapture instance.
func NewEventCapture() *EventCapture {
	return &EventCapture{
		events: make([]CapturedEvent, 0),
	}
}

// Handler returns an EventCallback that captures events.
// Use this with Hook or Observe to capture events for testing.
// Events are copied to avoid issues with event pooling.
func (ec *EventCapture) Handler() capitan.EventCallback {
	return func(_ context.Context, e *capitan.Event) {
		ec.mu.Lock()
		defer ec.mu.Unlock()
		ec.events = append(ec.events, CapturedEvent{
			Signal:    e.Signal(),
			Timestamp: e.Timestamp(),
			Severity:  e.Severity(),
			Fields:    e.Fields(), // Fields() returns a defensive copy
		})
	}
}

// Events returns a copy of all captured events.
func (ec *EventCapture) Events() []CapturedEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	result := make([]CapturedEvent, len(ec.events))
	copy(result, ec.events)
	return result
}

// Count returns the number of captured events.
func (ec *EventCapture) Count() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.events)
}

// Reset clears all captured events.
func (ec *EventCapture) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = ec.events[:0]
}

// WaitForCount blocks until the capture has at least n events or timeout occurs.
// Returns true if count reached, false if timeout.
func (ec *EventCapture) WaitForCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ec.Count() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// EventCounter counts events for testing.
// Thread-safe for concurrent counting.
type EventCounter struct {
	count int64
	mu    sync.Mutex
}

// NewEventCounter creates a new EventCounter instance.
func NewEventCounter() *EventCounter {
	return &EventCounter{}
}

// Handler returns an EventCallback that increments the counter.
func (ec *EventCounter) Handler() capitan.EventCallback {
	return func(_ context.Context, _ *capitan.Event) {
		ec.mu.Lock()
		defer ec.mu.Unlock()
		ec.count++
	}
}

// Count returns the current count.
func (ec *EventCounter) Count() int64 {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.count
}

// Reset resets the counter to zero.
func (ec *EventCounter) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.count = 0
}

// WaitForCount blocks until the counter reaches at least n or timeout occurs.
// Returns true if count reached, false if timeout.
func (ec *EventCounter) WaitForCount(n int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ec.Count() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// PanicRecorder records panics from listeners for testing.
// Useful for testing panic recovery behavior.
type PanicRecorder struct {
	panics []PanicRecord
	mu     sync.Mutex
}

// PanicRecord contains information about a recovered panic.
type PanicRecord struct {
	Signal    capitan.Signal
	Recovered any
	Timestamp time.Time
}

// NewPanicRecorder creates a new PanicRecorder instance.
func NewPanicRecorder() *PanicRecorder {
	return &PanicRecorder{
		panics: make([]PanicRecord, 0),
	}
}

// Handler returns a PanicHandler for use with WithPanicHandler.
func (pr *PanicRecorder) Handler() func(signal capitan.Signal, recovered any) {
	return func(signal capitan.Signal, recovered any) {
		pr.mu.Lock()
		defer pr.mu.Unlock()
		pr.panics = append(pr.panics, PanicRecord{
			Signal:    signal,
			Recovered: recovered,
			Timestamp: time.Now(),
		})
	}
}

// Panics returns a copy of all recorded panics.
func (pr *PanicRecorder) Panics() []PanicRecord {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	result := make([]PanicRecord, len(pr.panics))
	copy(result, pr.panics)
	return result
}

// Count returns the number of recorded panics.
func (pr *PanicRecorder) Count() int {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return len(pr.panics)
}

// Reset clears all recorded panics.
func (pr *PanicRecorder) Reset() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.panics = pr.panics[:0]
}

// FieldExtractor is a utility for extracting typed field values in tests.
type FieldExtractor struct{}

// NewFieldExtractor creates a new FieldExtractor instance.
func NewFieldExtractor() *FieldExtractor {
	return &FieldExtractor{}
}

// GetString extracts a string field value or returns empty string if not found.
func (*FieldExtractor) GetString(e *capitan.Event, key capitan.StringKey) string {
	val, _ := key.From(e)
	return val
}

// GetInt extracts an int field value or returns 0 if not found.
func (*FieldExtractor) GetInt(e *capitan.Event, key capitan.IntKey) int {
	val, _ := key.From(e)
	return val
}

// GetBool extracts a bool field value or returns false if not found.
func (*FieldExtractor) GetBool(e *capitan.Event, key capitan.BoolKey) bool {
	val, _ := key.From(e)
	return val
}

// GetFloat64 extracts a float64 field value or returns 0.0 if not found.
func (*FieldExtractor) GetFloat64(e *capitan.Event, key capitan.Float64Key) float64 {
	val, _ := key.From(e)
	return val
}

// TestCapitan creates a Capitan instance configured for testing.
// Uses async mode (real behavior) - use listener.Close() to ensure events are processed.
func TestCapitan(opts ...capitan.Option) *capitan.Capitan {
	return capitan.New(opts...)
}

// StatsWaiter polls stats until a condition is met or timeout occurs.
// Useful for testing async worker creation and queue draining.
type StatsWaiter struct {
	capitan *capitan.Capitan
}

// NewStatsWaiter creates a new StatsWaiter for the given Capitan instance.
func NewStatsWaiter(c *capitan.Capitan) *StatsWaiter {
	return &StatsWaiter{capitan: c}
}

// WaitForWorkers blocks until the number of active workers matches n or timeout occurs.
func (sw *StatsWaiter) WaitForWorkers(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stats := sw.capitan.Stats()
		if stats.ActiveWorkers == n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// WaitForEmptyQueues blocks until all queues are empty or timeout occurs.
func (sw *StatsWaiter) WaitForEmptyQueues(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stats := sw.capitan.Stats()
		allEmpty := true
		for _, depth := range stats.QueueDepths {
			if depth > 0 {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// WaitForEmitCount blocks until a signal has been emitted at least n times or timeout occurs.
func (sw *StatsWaiter) WaitForEmitCount(signal capitan.Signal, n uint64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stats := sw.capitan.Stats()
		if count, ok := stats.EmitCounts[signal]; ok && count >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
