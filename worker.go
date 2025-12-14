package capitan

import (
	"context"
	"sync"
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
		cfg := c.resolveConfig(signal)
		c.mu.RUnlock()

		// Check disabled
		if cfg.Disabled {
			c.mu.Lock()
			c.droppedEvents++
			c.mu.Unlock()
			return
		}

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
		c.processEventWithConfig(signal, event, cfg)
		return
	}

	// Fast path: check if worker already exists (read lock)
	c.mu.RLock()
	worker, exists := c.workers[signal]
	c.mu.RUnlock()

	if !exists {
		// Slow path: create worker (write lock)
		c.mu.Lock()

		// Double-check: another goroutine may have created it
		worker, exists = c.workers[signal]
		if !exists {
			// Check if listeners exist before creating worker
			if len(c.registry[signal]) == 0 {
				_, registryExists := c.registry[signal]
				if !registryExists {
					c.registry[signal] = nil
					c.attachObservers(signal)
				}

				if len(c.registry[signal]) == 0 {
					c.droppedEvents++
					c.mu.Unlock()
					return
				}
			}

			// Check if shutdown in progress
			select {
			case <-c.shutdown:
				c.mu.Unlock()
				return
			default:
			}

			// Resolve config once for this worker
			cfg := c.resolveConfig(signal)

			// Don't create worker for disabled signals
			if cfg.Disabled {
				c.droppedEvents++
				c.mu.Unlock()
				return
			}

			// Determine buffer size
			bufSize := c.bufferSize
			if cfg.BufferSize > 0 {
				bufSize = cfg.BufferSize
			}

			// Initialize rate limiter with burst capacity
			rl := rateLimiter{}
			if cfg.RateLimit > 0 {
				burst := cfg.BurstSize
				if burst <= 0 {
					burst = 1
				}
				rl.tokens = float64(burst)
				rl.lastCheck = nanotime()
			}

			// Create worker with config baked in
			worker = &workerState{
				events:      make(chan *Event, bufSize),
				done:        make(chan struct{}),
				markers:     make(chan chan struct{}),
				config:      cfg,
				rateLimiter: rl,
			}
			c.workers[signal] = worker
			c.wg.Add(1)
			go c.processEvents(signal, worker)
		}

		c.mu.Unlock()
	}

	// Create event from pool
	event := newEvent(ctx, signal, severity, timestamp, fields...)

	// Send to events channel based on worker's drop policy
	if worker.config.DropPolicy == DropPolicyDropNewest {
		// Non-blocking send - drop if buffer full
		select {
		case worker.events <- event:
			// Queued
		default:
			eventPool.Put(event)
			c.mu.Lock()
			c.droppedEvents++
			c.mu.Unlock()
		}
	} else {
		// Blocking send (default)
		select {
		case worker.events <- event:
			// Queued
		case <-ctx.Done():
			eventPool.Put(event)
		case <-worker.done:
			eventPool.Put(event)
		case <-c.shutdown:
			eventPool.Put(event)
		}
	}
}

// processEvent invokes all listeners for a signal with the given event.
// Handles panic recovery and returns pooled events to pool.
// Skips processing if the event's context has been canceled.
func (c *Capitan) processEvent(signal Signal, event *Event) {
	c.processEventWithConfig(signal, event, SignalConfig{})
}

// processEventWithConfig processes an event with explicit config (used for sync mode).
func (c *Capitan) processEventWithConfig(signal Signal, event *Event, cfg SignalConfig) {
	// Check if context was canceled while event was queued
	if event.ctx.Err() != nil {
		if !event.replay {
			eventPool.Put(event)
		}
		return
	}

	// Check minimum severity filter
	if cfg.MinSeverity != "" && !severityAtLeast(event.severity, cfg.MinSeverity) {
		c.mu.Lock()
		c.droppedEvents++
		c.mu.Unlock()
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

// drainEventsWithState processes all remaining events in the queue then returns.
func (c *Capitan) drainEventsWithState(signal Signal, state *workerState) {
	for {
		select {
		case event := <-state.events:
			c.processWorkerEvent(signal, state, event)
		default:
			return
		}
	}
}

// processWorkerEvent processes an event using the worker's config.
func (c *Capitan) processWorkerEvent(signal Signal, state *workerState, event *Event) {
	// Check if context was canceled
	if event.ctx.Err() != nil {
		if !event.replay {
			eventPool.Put(event)
		}
		return
	}

	// Check minimum severity filter
	if state.config.MinSeverity != "" && !severityAtLeast(event.severity, state.config.MinSeverity) {
		c.mu.Lock()
		c.droppedEvents++
		c.mu.Unlock()
		if !event.replay {
			eventPool.Put(event)
		}
		return
	}

	// Check rate limit
	if state.config.RateLimit > 0 {
		if !c.checkWorkerRateLimit(state) {
			c.mu.Lock()
			c.droppedEvents++
			c.mu.Unlock()
			if !event.replay {
				eventPool.Put(event)
			}
			return
		}
	}

	// Copy listener slice while holding lock
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

	if !event.replay {
		eventPool.Put(event)
	}
}

// checkWorkerRateLimit checks if an event is allowed under the worker's rate limit.
// Uses token bucket algorithm. Returns true if allowed.
func (*Capitan) checkWorkerRateLimit(state *workerState) bool {
	now := nanotime()
	rl := &state.rateLimiter

	// Calculate elapsed time and add tokens
	elapsed := float64(now-rl.lastCheck) / 1e9 // seconds
	rl.lastCheck = now

	burst := state.config.BurstSize
	if burst <= 0 {
		burst = 1
	}

	rl.tokens += elapsed * state.config.RateLimit
	if rl.tokens > float64(burst) {
		rl.tokens = float64(burst)
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// processEvents is the worker goroutine for a specific signal.
// Processes events from the queue and invokes all registered listeners.
func (c *Capitan) processEvents(signal Signal, state *workerState) {
	defer c.wg.Done()
	defer func() {
		// Clean up worker state when exiting
		// Only delete if we're still the current worker (prevents old worker
		// from deleting new worker during config-triggered rebuild)
		c.mu.Lock()
		if c.workers[signal] == state {
			delete(c.workers, signal)
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case event := <-state.events:
			c.processWorkerEvent(signal, state, event)

		case marker := <-state.markers:
			c.drainEventsWithState(signal, state)
			close(marker)

		case <-state.done:
			c.drainEventsWithState(signal, state)
			return

		case <-c.shutdown:
			c.drainEventsWithState(signal, state)
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

// IsShutdown reports whether Shutdown has been called on this instance.
func (c *Capitan) IsShutdown() bool {
	select {
	case <-c.shutdown:
		return true
	default:
		return false
	}
}

// Drain blocks until all currently queued events have been processed.
// Unlike Shutdown, workers remain active after draining.
// Returns an error if the context is canceled before drain completes.
func (c *Capitan) Drain(ctx context.Context) error {
	// Snapshot current workers
	c.mu.RLock()
	workers := make(map[Signal]*workerState, len(c.workers))
	for sig, w := range c.workers {
		workers[sig] = w
	}
	c.mu.RUnlock()

	if len(workers) == 0 {
		return nil
	}

	// Inject markers into all workers concurrently
	var wg sync.WaitGroup

	for _, worker := range workers {
		wg.Add(1)
		go func(w *workerState) {
			defer wg.Done()
			marker := make(chan struct{})
			select {
			case w.markers <- marker:
				select {
				case <-marker:
					// Drained successfully
				case <-ctx.Done():
					// Context canceled while waiting for drain
				}
			case <-w.done:
				// Worker already shutting down
			case <-c.shutdown:
				// Global shutdown
			case <-ctx.Done():
				// Context canceled
			}
		}(worker)
	}

	// Wait for all markers with context
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
