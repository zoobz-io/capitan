package capitan

import (
	"context"
	"sync"
)

// Observer represents a dynamic subscription that receives events from multiple signals.
//
// Unlike Listener which subscribes to a single signal, Observer can watch all signals
// or a whitelist of specific signals. Observers automatically attach to new signals
// as they are created, making them suitable for cross-cutting concerns like logging.
//
// Call Close to unregister all underlying listeners and stop receiving events.
type Observer struct {
	listeners []*Listener
	callback  EventCallback
	capitan   *Capitan
	active    bool
	signals   map[Signal]struct{} // nil = all signals, non-nil = whitelist
	mu        sync.Mutex
}

// Close removes all individual listeners from the registry.
func (o *Observer) Close() {
	// Lock observer first to mark inactive
	o.mu.Lock()
	if !o.active {
		o.mu.Unlock()
		return // Already closed
	}
	o.active = false
	listeners := o.listeners
	o.listeners = nil
	o.mu.Unlock()

	// Remove from capitan's observer list
	o.capitan.mu.Lock()
	for i, obs := range o.capitan.observers {
		if obs == o {
			// Swap with last element and truncate
			lastIdx := len(o.capitan.observers) - 1
			o.capitan.observers[i] = o.capitan.observers[lastIdx]
			o.capitan.observers = o.capitan.observers[:lastIdx]
			break
		}
	}
	o.capitan.mu.Unlock()

	// Close all listeners
	for _, l := range listeners {
		l.Close()
	}
}

// Drain blocks until all events queued before Drain was called have been processed.
// Unlike Close, the observer remains active after draining.
// Returns an error if the context is canceled before drain completes.
func (o *Observer) Drain(ctx context.Context) error {
	o.mu.Lock()
	if !o.active {
		o.mu.Unlock()
		return nil
	}
	listeners := make([]*Listener, len(o.listeners))
	copy(listeners, o.listeners)
	o.mu.Unlock()

	if len(listeners) == 0 {
		return nil
	}

	// Drain all listeners concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, len(listeners))

	for _, l := range listeners {
		wg.Add(1)
		go func(listener *Listener) {
			defer wg.Done()
			if err := listener.Drain(ctx); err != nil {
				errCh <- err
			}
		}(l)
	}

	// Wait for all drains with context
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Observe registers a callback for all signals on the default instance.
//
// If signals are provided, only those signals will be observed (whitelist mode).
// If no signals are provided, all signals will be observed including future ones.
//
// Returns an Observer that can be closed to unregister all listeners.
//
// Example (logging all events):
//
//	observer := capitan.Observe(func(ctx context.Context, e *capitan.Event) {
//	    log.Printf("[%s] %s: %s",
//	        e.Severity(),
//	        e.Signal().Name(),
//	        e.Signal().Description())
//	})
//	defer observer.Close()
//
// Example (whitelist specific signals):
//
//	observer := capitan.Observe(handler, orderCreated, orderShipped, orderCanceled)
func Observe(callback EventCallback, signals ...Signal) *Observer {
	return defaultInstance().Observe(callback, signals...)
}

// Observe registers a callback for all signals (dynamic).
// If signals are provided, only those signals will be observed (whitelist).
// If no signals are provided, all signals will be observed.
// The observer will receive events from both existing and future signals.
// Returns an Observer that can be closed to unregister all listeners.
func (c *Capitan) Observe(callback EventCallback, signals ...Signal) *Observer {
	c.mu.Lock()
	defer c.mu.Unlock()

	o := &Observer{
		listeners: make([]*Listener, 0, len(c.registry)),
		callback:  callback,
		capitan:   c,
		active:    true,
		signals:   nil, // nil = observe all
	}

	// Build whitelist if signals provided
	if len(signals) > 0 {
		o.signals = make(map[Signal]struct{}, len(signals))
		for _, sig := range signals {
			o.signals[sig] = struct{}{}
		}
	}

	// Hook existing signals (filtered by whitelist if present)
	for signal := range c.registry {
		// Skip if whitelist exists and signal not in it
		if o.signals != nil {
			if _, ok := o.signals[signal]; !ok {
				continue
			}
		}

		listener := &Listener{
			signal:   signal,
			callback: callback,
			capitan:  c,
		}
		c.registry[signal] = append(c.registry[signal], listener)
		c.listenerVersions[signal]++
		o.listeners = append(o.listeners, listener)
	}

	// Add to observers list for future signals
	c.observers = append(c.observers, o)

	return o
}

// attachObservers attaches all active observers to a signal.
// Must be called while holding c.mu write lock.
func (c *Capitan) attachObservers(signal Signal) {
	for _, obs := range c.observers {
		obs.mu.Lock()
		if obs.active {
			// Skip if observer has whitelist and signal not in it
			if obs.signals != nil {
				if _, ok := obs.signals[signal]; !ok {
					obs.mu.Unlock()
					continue
				}
			}

			obsListener := &Listener{
				signal:   signal,
				callback: obs.callback,
				capitan:  c,
			}
			c.registry[signal] = append(c.registry[signal], obsListener)
			c.listenerVersions[signal]++
			obs.listeners = append(obs.listeners, obsListener)
		}
		obs.mu.Unlock()
	}
}
