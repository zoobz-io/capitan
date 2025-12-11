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
func (l *Listener) Close() {
	l.capitan.unregister(l)
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
