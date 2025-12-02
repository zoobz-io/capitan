package capitan

import "sync"

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
			obs.listeners = append(obs.listeners, obsListener)
		}
		obs.mu.Unlock()
	}
}
