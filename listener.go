package capitan

import (
	"context"
	"sync"
)

// EventCallback is a function that handles an Event.
// The context is inherited from the Emit call and can be used for cancellation,
// timeouts, and accessing request-scoped values.
// Handlers are responsible for their own error handling and logging.
// Events must not be modified by listeners.
type EventCallback func(context.Context, *Event)

// Listener represents an active subscription to a signal.
// Call Close() to unregister the listener and prevent further callbacks.
type Listener struct {
	signal   Signal
	callback EventCallback
	capitan  *Capitan
}

// Close removes this listener from the registry, preventing future callbacks.
// Blocks until all events queued before Close was called have been processed.
func (l *Listener) Close() {
	c := l.capitan

	// In async mode, wait for queued events to drain before unregistering
	if !c.syncMode {
		c.mu.RLock()
		worker, exists := c.workers[l.signal]
		c.mu.RUnlock()

		if exists {
			marker := make(chan struct{})
			select {
			case worker.markers <- marker:
				<-marker // Block until worker processes marker
			case <-worker.done:
				// Worker already shutting down
			case <-c.shutdown:
				// Global shutdown in progress
			}
		}
	}

	c.unregister(l)
}

// Drain blocks until all events queued before Drain was called have been processed.
// Unlike Close, the listener remains active after draining.
// Returns an error if the context is canceled before drain completes.
func (l *Listener) Drain(ctx context.Context) error {
	c := l.capitan

	// Sync mode has no queue to drain
	if c.syncMode {
		return nil
	}

	c.mu.RLock()
	worker, exists := c.workers[l.signal]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	marker := make(chan struct{})
	select {
	case worker.markers <- marker:
		select {
		case <-marker:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-worker.done:
		return nil
	case <-c.shutdown:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HookOnce registers a callback that fires only once, then automatically unregisters.
// Returns a Listener that can be closed early to prevent the callback from firing.
//
// Example:
//
//	listener := capitan.HookOnce(orderCreated, func(ctx context.Context, e *capitan.Event) {
//	    // This handler runs at most once
//	    fmt.Println("First order received!")
//	})
func HookOnce(signal Signal, callback EventCallback) *Listener {
	return defaultInstance().HookOnce(signal, callback)
}

// HookOnce registers a callback that fires only once, then automatically unregisters.
// Returns a Listener that can be closed early to prevent the callback from firing.
func (c *Capitan) HookOnce(signal Signal, callback EventCallback) *Listener {
	var listener *Listener
	var once sync.Once
	listener = c.Hook(signal, func(ctx context.Context, e *Event) {
		once.Do(func() {
			callback(ctx, e)
			// Defer close to avoid unregistering while holding lock
			go listener.Close()
		})
	})
	return listener
}
