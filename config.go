package capitan

import (
	"path"
	"sync"
	"time"
)

// timeNow is a variable for testing purposes.
var timeNow = time.Now

var (
	defaultOptions []Option
	defaultOptMu   sync.Mutex
)

// DropPolicy defines behavior when the event buffer is full.
type DropPolicy string

const (
	// DropPolicyBlock waits for space in the buffer (default behavior).
	DropPolicyBlock DropPolicy = "block"
	// DropPolicyDropNewest drops incoming events if the buffer is full.
	DropPolicyDropNewest DropPolicy = "drop_newest"
)

// SignalConfig holds per-signal configuration options.
// Zero values indicate "use default" for all fields.
type SignalConfig struct {
	// BufferSize sets the event queue size for this signal.
	// Default: instance-level bufferSize (typically 16).
	BufferSize int `json:"bufferSize,omitempty"`

	// Disabled suppresses all events for this signal when true.
	Disabled bool `json:"disabled,omitempty"`

	// MinSeverity filters events below this severity level.
	// Events with severity < MinSeverity are silently dropped.
	MinSeverity Severity `json:"minSeverity,omitempty"`

	// MaxListeners caps the number of listeners for this signal.
	// Hook() returns nil when the limit is reached. 0 = unlimited.
	MaxListeners int `json:"maxListeners,omitempty"`

	// DropPolicy controls behavior when the event buffer is full.
	// Default: DropPolicyBlock (wait for space).
	DropPolicy DropPolicy `json:"dropPolicy,omitempty"`

	// RateLimit sets the maximum events per second for this signal.
	// Events exceeding the rate are silently dropped. 0 = unlimited.
	RateLimit float64 `json:"rateLimit,omitempty"`

	// BurstSize sets the burst allowance above the rate limit.
	// Allows short bursts before rate limiting kicks in.
	BurstSize int `json:"burstSize,omitempty"`
}

// Config is the serializable configuration for per-signal settings.
// Supports exact signal names and glob patterns (e.g., "order.*").
type Config struct {
	// Signals maps signal names or glob patterns to their configuration.
	// Glob patterns use path.Match syntax: *, ?, [...]
	Signals map[string]SignalConfig `json:"signals"`
}

// Validate checks the configuration for errors.
// Implements the Validator interface for flux compatibility.
func (c Config) Validate() error {
	for pattern, cfg := range c.Signals {
		// Validate glob pattern syntax
		if isGlobPattern(pattern) {
			if _, err := path.Match(pattern, ""); err != nil {
				return err
			}
		}

		// Validate buffer size
		if cfg.BufferSize < 0 {
			return &ConfigError{Pattern: pattern, Field: "bufferSize", Reason: "must be non-negative"}
		}

		// Validate max listeners
		if cfg.MaxListeners < 0 {
			return &ConfigError{Pattern: pattern, Field: "maxListeners", Reason: "must be non-negative"}
		}

		// Validate rate limit
		if cfg.RateLimit < 0 {
			return &ConfigError{Pattern: pattern, Field: "rateLimit", Reason: "must be non-negative"}
		}

		// Validate burst size
		if cfg.BurstSize < 0 {
			return &ConfigError{Pattern: pattern, Field: "burstSize", Reason: "must be non-negative"}
		}

		// Validate drop policy
		if cfg.DropPolicy != "" && cfg.DropPolicy != DropPolicyBlock && cfg.DropPolicy != DropPolicyDropNewest {
			return &ConfigError{Pattern: pattern, Field: "dropPolicy", Reason: "must be 'block' or 'drop_newest'"}
		}

		// Validate min severity
		if cfg.MinSeverity != "" && !isValidSeverity(cfg.MinSeverity) {
			return &ConfigError{Pattern: pattern, Field: "minSeverity", Reason: "must be DEBUG, INFO, WARN, or ERROR"}
		}
	}
	return nil
}

// ConfigError represents a validation error in the configuration.
type ConfigError struct {
	Pattern string
	Field   string
	Reason  string
}

func (e *ConfigError) Error() string {
	return "config error for " + e.Pattern + ": " + e.Field + " " + e.Reason
}

// isGlobPattern returns true if the pattern contains glob characters.
func isGlobPattern(pattern string) bool {
	for _, c := range pattern {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

// isValidSeverity returns true if s is a valid severity level.
func isValidSeverity(s Severity) bool {
	return s == SeverityDebug || s == SeverityInfo || s == SeverityWarn || s == SeverityError
}

// configEqual returns true if two SignalConfigs are equivalent.
func configEqual(a, b SignalConfig) bool {
	return a.BufferSize == b.BufferSize &&
		a.Disabled == b.Disabled &&
		a.MinSeverity == b.MinSeverity &&
		a.MaxListeners == b.MaxListeners &&
		a.DropPolicy == b.DropPolicy &&
		a.RateLimit == b.RateLimit &&
		a.BurstSize == b.BurstSize
}

// resolveConfigFrom resolves the configuration for a signal from a Config.
// Uses specificity rules: exact matches win, then longest glob pattern, then alphabetical.
func resolveConfigFrom(signal Signal, cfg Config) SignalConfig {
	name := signal.Name()

	// Exact match wins
	if c, ok := cfg.Signals[name]; ok {
		return c
	}

	// Find best glob match by specificity
	var bestPattern string
	var bestConfig SignalConfig
	for pattern, c := range cfg.Signals {
		if !isGlobPattern(pattern) {
			continue
		}
		if matched, err := path.Match(pattern, name); err == nil && matched {
			// Longer pattern = more specific
			if len(pattern) > len(bestPattern) ||
				(len(pattern) == len(bestPattern) && pattern > bestPattern) {
				bestPattern = pattern
				bestConfig = c
			}
		}
	}
	return bestConfig
}

// ApplyConfig applies per-signal configuration from a Config.
// Replaces the entire configuration - signals not in the new config revert to defaults.
// Supports exact signal names and glob patterns.
//
// Resolution uses specificity rules:
//   - Exact matches always win over globs
//   - Among globs, longest pattern wins (more specific)
//   - Ties: alphabetical order (deterministic)
//
// Only rebuilds workers whose resolved config actually changed.
//
// Example:
//
//	cfg := capitan.Config{
//	    Signals: map[string]capitan.SignalConfig{
//	        "order.*":        {BufferSize: 32, MinSeverity: capitan.SeverityInfo},
//	        "order.created":  {BufferSize: 64},  // Exact match overrides glob
//	    },
//	}
//	cap.ApplyConfig(cfg)
func (c *Capitan) ApplyConfig(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Store old config for comparison
	oldConfig := c.config

	// Replace with new config
	c.config = cfg

	// Check each active worker for config changes
	for signal, worker := range c.workers {
		oldCfg := resolveConfigFrom(signal, oldConfig)
		newCfg := resolveConfigFrom(signal, cfg)

		if !configEqual(oldCfg, newCfg) {
			// Config changed - rebuild worker
			// Compare against worker's actual config in case it differs from oldCfg
			if !configEqual(worker.config, newCfg) {
				c.rebuildWorkerLocked(signal)
			}
		}
	}

	return nil
}

// rebuildWorkerLocked drains and rebuilds a worker with updated config.
// If the new config disables the signal, the worker is destroyed without replacement.
// Must be called while holding c.mu write lock.
func (c *Capitan) rebuildWorkerLocked(signal Signal) {
	oldWorker, exists := c.workers[signal]
	if !exists {
		return
	}

	// Resolve the new config
	cfg := c.resolveConfig(signal)

	// If disabled, just destroy the worker
	if cfg.Disabled {
		delete(c.workers, signal)
		close(oldWorker.done)
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

	// Create new worker with config baked in
	newWorker := &workerState{
		events:      make(chan *Event, bufSize),
		done:        make(chan struct{}),
		markers:     make(chan chan struct{}),
		config:      cfg,
		rateLimiter: rl,
	}

	// Atomic swap - new events go to new worker immediately
	c.workers[signal] = newWorker
	c.wg.Add(1)
	go c.processEvents(signal, newWorker)

	// Signal old worker to drain and exit
	close(oldWorker.done)
}

// ApplyConfig applies configuration on the default instance.
func ApplyConfig(cfg Config) error {
	return defaultInstance().ApplyConfig(cfg)
}

// resolveConfig returns the configuration for a signal from the stored Config.
// Uses specificity rules: exact matches win, then longest glob pattern, then alphabetical.
// Returns an empty SignalConfig if no config is found.
// Must be called while holding c.mu (read or write) lock.
func (c *Capitan) resolveConfig(signal Signal) SignalConfig {
	return resolveConfigFrom(signal, c.config)
}

// nanotime returns the current time in nanoseconds.
// Using time.Now().UnixNano() for simplicity.
func nanotime() int64 {
	return timeNow().UnixNano()
}

// Option configures a Capitan instance.
type Option func(*Capitan)

// PanicHandler is called when a listener panics during event processing.
// Receives the signal being processed and the recovered panic value.
type PanicHandler func(signal Signal, recovered any)

// Configure sets options for the default Capitan instance.
// Must be called before any module-level functions (Hook, Emit, Observe, Shutdown).
// Subsequent calls have no effect once the default instance is created.
func Configure(opts ...Option) {
	defaultOptMu.Lock()
	defaultOptions = opts
	defaultOptMu.Unlock()
}

// WithBufferSize sets the event queue buffer size for each signal's worker.
// Default is 16. Larger buffers reduce backpressure but increase memory usage.
func WithBufferSize(size int) Option {
	return func(c *Capitan) {
		if size > 0 {
			c.bufferSize = size
		}
	}
}

// WithPanicHandler sets a callback to be invoked when a listener panics.
// The handler receives the signal and the recovered panic value.
// By default, panics are recovered silently to prevent system crashes.
func WithPanicHandler(handler PanicHandler) Option {
	return func(c *Capitan) {
		c.panicHandler = handler
	}
}

// WithSyncMode enables synchronous event processing for testing.
// When enabled, Emit() calls listeners directly instead of queueing to workers.
// This eliminates timing dependencies and makes tests deterministic.
// Should only be used in tests, not production code.
func WithSyncMode() Option {
	return func(c *Capitan) {
		c.syncMode = true
	}
}
