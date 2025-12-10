package capitan

import (
	"context"
	"time"
)

// Emit dispatches an event with Info severity (default).
// Queues the event for asynchronous processing by the signal's worker goroutine.
// Creates a worker goroutine lazily on first emission to this signal.
// Silently drops events if no listeners are registered for the signal.
// If the context is canceled before the event can be queued, the event is dropped.
func (c *Capitan) Emit(ctx context.Context, signal Signal, fields ...Field) {
	c.emitWithSeverity(ctx, signal, SeverityInfo, fields...)
}

// Debug dispatches an event with Debug severity.
func (c *Capitan) Debug(ctx context.Context, signal Signal, fields ...Field) {
	c.emitWithSeverity(ctx, signal, SeverityDebug, fields...)
}

// Info dispatches an event with Info severity.
func (c *Capitan) Info(ctx context.Context, signal Signal, fields ...Field) {
	c.emitWithSeverity(ctx, signal, SeverityInfo, fields...)
}

// Warn dispatches an event with Warn severity.
func (c *Capitan) Warn(ctx context.Context, signal Signal, fields ...Field) {
	c.emitWithSeverity(ctx, signal, SeverityWarn, fields...)
}

// Error dispatches an event with Error severity.
func (c *Capitan) Error(ctx context.Context, signal Signal, fields ...Field) {
	c.emitWithSeverity(ctx, signal, SeverityError, fields...)
}

// Replay re-emits a historical event, preserving its original timestamp and severity.
// The event is marked as a replay, accessible via Event.IsReplay().
// Replay events are processed synchronously and are not pooled.
//
// Use NewEvent to construct events from stored data:
//
//	e := capitan.NewEvent(signal, severity, timestamp, fields...)
//	c.Replay(ctx, e)
func (c *Capitan) Replay(ctx context.Context, e *Event) {
	// Mark as replay and set context
	e.replay = true
	e.ctx = ctx

	signal := e.signal

	// Ensure signal has listeners (attach observers if needed)
	c.mu.Lock()
	if _, exists := c.registry[signal]; !exists {
		c.registry[signal] = nil
		c.attachObservers(signal)
	}
	c.mu.Unlock()

	// Process synchronously
	c.processEvent(signal, e)
}

// emitWithSeverity dispatches an event with the given severity level.
// Internal function used by public emit methods.
func (c *Capitan) emitWithSeverity(ctx context.Context, signal Signal, severity Severity, fields ...Field) {
	// Capture timestamp immediately to preserve chronological ordering
	timestamp := time.Now()

	// Track emit count and field schema
	c.mu.Lock()
	c.emitCounts[signal]++
	// Capture field schema on first emit
	if _, exists := c.fieldSchemas[signal]; !exists && len(fields) > 0 {
		keys := make([]Key, len(fields))
		for i, field := range fields {
			keys[i] = field.Key()
		}
		c.fieldSchemas[signal] = keys
	}
	c.mu.Unlock()

	// Sync mode: process event directly without workers
	if c.syncMode {
		c.mu.RLock()
		listeners := c.registry[signal]
		c.mu.RUnlock()

		// If no listeners, attach observers
		if len(listeners) == 0 {
			c.mu.Lock()
			_, registryExists := c.registry[signal]
			if !registryExists {
				c.registry[signal] = nil
				c.attachObservers(signal)
			}
			c.mu.Unlock()
		}

		// Create and process event synchronously
		event := newEvent(ctx, signal, severity, timestamp, fields...)
		c.processEvent(signal, event)
		return
	}

	// Fast path: check if worker already exists (read lock)
	c.mu.RLock()
	_, exists := c.workers[signal]
	c.mu.RUnlock()

	if !exists {
		// Slow path: create worker (write lock)
		c.mu.Lock()

		// Double-check: another goroutine may have created it
		_, exists = c.workers[signal]
		if !exists {
			// Check if listeners exist before creating worker
			if len(c.registry[signal]) == 0 {
				// Check if this is a new signal for observers
				_, registryExists := c.registry[signal]
				if !registryExists {
					// Initialize registry entry for this signal
					c.registry[signal] = nil

					// Attach to all active observers
					c.attachObservers(signal)
				}

				// If still no listeners after observer attachment, drop event
				if len(c.registry[signal]) == 0 {
					c.droppedEvents++
					c.mu.Unlock()
					return
				}
			}

			// Check if shutdown in progress before creating worker
			select {
			case <-c.shutdown:
				// Shutdown initiated, don't create new workers
				c.mu.Unlock()
				return
			default:
				// Proceed with worker creation
			}

			// Create worker only if listeners exist
			newWorker := &workerState{
				events: make(chan *Event, c.bufferSize),
				done:   make(chan struct{}),
			}
			c.workers[signal] = newWorker
			c.wg.Add(1)
			go c.processEvents(signal, newWorker)
		}

		c.mu.Unlock()
	}

	// Create event from pool
	event := newEvent(ctx, signal, severity, timestamp, fields...)

	// Capture worker reference atomically to avoid TOCTOU race
	c.mu.RLock()
	worker, workerExists := c.workers[signal]
	c.mu.RUnlock()

	if !workerExists {
		// Worker closed between initial check and now (no listeners)
		eventPool.Put(event)
		return
	}

	// Send to events channel (never closed, so no panic risk)
	select {
	case worker.events <- event:
		// Event queued successfully
	case <-ctx.Done():
		// Context canceled while waiting to queue
		eventPool.Put(event)
	case <-worker.done:
		// Worker shutting down, drop event
		eventPool.Put(event)
	case <-c.shutdown:
		// Global shutdown fired while waiting to send
		eventPool.Put(event)
	}
}

// processEvent invokes all listeners for a signal with the given event.
// Handles panic recovery and returns pooled events to pool.
// Skips processing if the event's context has been canceled.
func (c *Capitan) processEvent(signal Signal, event *Event) {
	// Check if context was canceled while event was queued
	if event.ctx.Err() != nil {
		// Skip canceled events, return pooled events to pool
		if !event.replay {
			eventPool.Put(event)
		}
		return
	}

	// Copy listener slice while holding lock to prevent data race
	c.mu.RLock()
	listeners := make([]*Listener, len(c.registry[signal]))
	copy(listeners, c.registry[signal])
	c.mu.RUnlock()

	// Invoke all listeners with panic recovery
	for _, listener := range listeners {
		func() {
			defer func() {
				if r := recover(); r != nil && c.panicHandler != nil {
					c.panicHandler(signal, r)
				}
			}()
			listener.callback(event.ctx, event)
		}()
	}

	// Return pooled events to pool (replay events are not pooled)
	if !event.replay {
		eventPool.Put(event)
	}
}

// drainEvents processes all remaining events in the queue then returns.
func (c *Capitan) drainEvents(signal Signal, events chan *Event) {
	for {
		select {
		case event := <-events:
			c.processEvent(signal, event)
		default:
			return
		}
	}
}

// processEvents is the worker goroutine for a specific signal.
// Processes events from the queue and invokes all registered listeners.
func (c *Capitan) processEvents(signal Signal, state *workerState) {
	defer c.wg.Done()
	defer func() {
		// Clean up worker state when exiting
		c.mu.Lock()
		delete(c.workers, signal)
		c.mu.Unlock()
	}()

	for {
		select {
		case event := <-state.events:
			c.processEvent(signal, event)

		case <-state.done:
			// Per-worker shutdown: drain remaining events then exit
			c.drainEvents(signal, state.events)
			return

		case <-c.shutdown:
			// Global shutdown: drain remaining events then exit
			c.drainEvents(signal, state.events)
			return
		}
	}
}

// Shutdown gracefully stops all worker goroutines, draining pending events.
// Safe to call multiple times; subsequent calls are no-ops.
func (c *Capitan) Shutdown() {
	c.shutdownOnce.Do(func() {
		// Close shutdown channel while holding mutex to synchronize with worker creation
		c.mu.Lock()
		close(c.shutdown)
		c.mu.Unlock()
	})
	c.wg.Wait()
}
