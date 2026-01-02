package capitan

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestWithBufferSize verifies custom buffer size configuration.
func TestWithBufferSize(t *testing.T) {
	c := New(WithBufferSize(128))

	if c.bufferSize != 128 {
		t.Errorf("expected bufferSize=128, got %d", c.bufferSize)
	}

	c.Shutdown()
}

// TestWithBufferSizeDefault verifies default buffer size.
func TestWithBufferSizeDefault(t *testing.T) {
	c := New()

	if c.bufferSize != 16 {
		t.Errorf("expected default bufferSize=16, got %d", c.bufferSize)
	}

	c.Shutdown()
}

// TestWithBufferSizeInvalid verifies invalid buffer size is rejected.
func TestWithBufferSizeInvalid(t *testing.T) {
	c := New(WithBufferSize(-1))

	if c.bufferSize != 16 {
		t.Errorf("expected bufferSize to remain default (16), got %d", c.bufferSize)
	}

	c.Shutdown()
}

// TestWithPanicHandler verifies panic handler is called on listener panic.
func TestWithPanicHandler(t *testing.T) {
	var panicSignal Signal
	var panicValue any

	handler := func(sig Signal, recovered any) {
		panicSignal = sig
		panicValue = recovered
	}

	c := New(WithPanicHandler(handler), WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.panic", "Test panic signal")
	key := NewStringKey("value")

	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("test panic")
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if panicSignal != sig {
		t.Errorf("expected panicSignal=%q, got %q", sig, panicSignal)
	}

	if panicValue != "test panic" {
		t.Errorf("expected panicValue=%q, got %v", "test panic", panicValue)
	}
}

// TestWithPanicHandlerNotSet verifies silent recovery when no handler set.
func TestWithPanicHandlerNotSet(t *testing.T) {
	c := New(WithSyncMode()) // No panic handler
	defer c.Shutdown()

	sig := NewSignal("test.panic.silent", "Test panic silent signal")
	key := NewStringKey("value")
	var called bool

	// First listener panics
	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("silent panic")
	})

	// Second listener should still run
	c.Hook(sig, func(_ context.Context, _ *Event) {
		called = true
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if !called {
		t.Error("second listener should have been called despite first listener panic")
	}
}

// TestStats verifies Stats() returns correct metrics.
func TestStats(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig1 := NewSignal("test.stats.1", "Test stats signal 1")
	sig2 := NewSignal("test.stats.2", "Test stats signal 2")
	key := NewStringKey("value")

	// Hook listeners
	c.Hook(sig1, func(_ context.Context, _ *Event) {
		time.Sleep(50 * time.Millisecond) // Slow listener
	})
	c.Hook(sig1, func(_ context.Context, _ *Event) {
		time.Sleep(50 * time.Millisecond)
	})
	c.Hook(sig2, func(_ context.Context, _ *Event) {
		time.Sleep(50 * time.Millisecond)
	})

	// Emit events to create workers
	c.Emit(context.Background(), sig1, key.Field("test1"))
	c.Emit(context.Background(), sig2, key.Field("test2"))

	// Give workers time to start
	time.Sleep(10 * time.Millisecond)

	stats := c.Stats()

	if stats.ActiveWorkers != 2 {
		t.Errorf("expected 2 active workers, got %d", stats.ActiveWorkers)
	}

	if stats.ListenerCounts[sig1] != 2 {
		t.Errorf("expected 2 listeners for sig1, got %d", stats.ListenerCounts[sig1])
	}

	if stats.ListenerCounts[sig2] != 1 {
		t.Errorf("expected 1 listener for sig2, got %d", stats.ListenerCounts[sig2])
	}

	// QueueDepths should be 0 or small (events being processed)
	if _, exists := stats.QueueDepths[sig1]; !exists {
		t.Error("expected QueueDepths to contain sig1")
	}
	if _, exists := stats.QueueDepths[sig2]; !exists {
		t.Error("expected QueueDepths to contain sig2")
	}

	// Emit counts should track emissions
	if stats.EmitCounts[sig1] != 1 {
		t.Errorf("expected emit count 1 for sig1, got %d", stats.EmitCounts[sig1])
	}
	if stats.EmitCounts[sig2] != 1 {
		t.Errorf("expected emit count 1 for sig2, got %d", stats.EmitCounts[sig2])
	}

	// Field schemas should be captured
	if len(stats.FieldSchemas[sig1]) != 1 {
		t.Errorf("expected 1 field in schema for sig1, got %d", len(stats.FieldSchemas[sig1]))
	}
	if len(stats.FieldSchemas[sig2]) != 1 {
		t.Errorf("expected 1 field in schema for sig2, got %d", len(stats.FieldSchemas[sig2]))
	}
	if stats.FieldSchemas[sig1][0].Name() != "value" {
		t.Errorf("expected field name 'value', got %q", stats.FieldSchemas[sig1][0].Name())
	}
}

// TestMultipleOptions verifies multiple options can be combined.
func TestMultipleOptions(t *testing.T) {
	var handlerCalled bool

	c := New(
		WithBufferSize(256),
		WithPanicHandler(func(_ Signal, _ any) {
			handlerCalled = true
		}),
		WithSyncMode(),
	)
	defer c.Shutdown()

	if c.bufferSize != 256 {
		t.Errorf("expected bufferSize=256, got %d", c.bufferSize)
	}

	if c.panicHandler == nil {
		t.Error("expected panicHandler to be set")
	}

	// Verify handler works
	sig := NewSignal("test.multi", "Test multi signal")
	key := NewStringKey("value")

	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("test")
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if !handlerCalled {
		t.Error("expected panic handler to be called")
	}
}

// TestWithSyncMode verifies synchronous event processing.
func TestWithSyncMode(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	if !c.syncMode {
		t.Error("expected syncMode to be true")
	}

	// Verify events are processed synchronously (no timing needed)
	sig := NewSignal("test.sync", "Test sync signal")
	key := NewStringKey("value")
	var called bool

	c.Hook(sig, func(_ context.Context, e *Event) {
		called = true
		val, ok := key.From(e)
		if !ok || val != "sync-test" {
			t.Errorf("expected value='sync-test', got %v, %v", val, ok)
		}
	})

	c.Emit(context.Background(), sig, key.Field("sync-test"))

	// No sleep needed - should be processed immediately
	if !called {
		t.Error("expected listener to be called synchronously")
	}

	// Verify no workers were created
	stats := c.Stats()
	if stats.ActiveWorkers != 0 {
		t.Errorf("expected 0 active workers in sync mode, got %d", stats.ActiveWorkers)
	}
}

// ========== Per-Signal Configuration Tests ==========

// TestConfigValidateBufferSize verifies buffer size validation.
func TestConfigValidateBufferSize(t *testing.T) {
	cfg := Config{
		Signals: map[string]SignalConfig{
			"test.signal": {BufferSize: -1},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative buffer size")
	}
}

// TestConfigValidateInvalidGlob verifies invalid glob pattern validation.
func TestConfigValidateInvalidGlob(t *testing.T) {
	cfg := Config{
		Signals: map[string]SignalConfig{
			"test.[": {BufferSize: 32}, // Invalid glob
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

// TestConfigValidateInvalidDropPolicy verifies drop policy validation.
func TestConfigValidateInvalidDropPolicy(t *testing.T) {
	cfg := Config{
		Signals: map[string]SignalConfig{
			"test.signal": {DropPolicy: "invalid"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid drop policy")
	}
}

// TestConfigValidateInvalidMinSeverity verifies min severity validation.
func TestConfigValidateInvalidMinSeverity(t *testing.T) {
	cfg := Config{
		Signals: map[string]SignalConfig{
			"test.signal": {MinSeverity: "INVALID"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid min severity")
	}
}

// TestConfigValidateValid verifies valid config passes validation.
func TestConfigValidateValid(t *testing.T) {
	cfg := Config{
		Signals: map[string]SignalConfig{
			"order.*":       {BufferSize: 32, MinSeverity: SeverityInfo},
			"order.created": {BufferSize: 64, DropPolicy: DropPolicyDropNewest},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestApplyConfigExactName verifies applying config to exact signal name.
func TestApplyConfigExactName(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("order.created", "Order created")
	key := NewStringKey("id")
	var received bool

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received = true
	})

	cfg := Config{
		Signals: map[string]SignalConfig{
			"order.created": {BufferSize: 64},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Verify config is stored
	c.mu.RLock()
	storedCfg := c.resolveConfig(sig)
	c.mu.RUnlock()

	if storedCfg.BufferSize != 64 {
		t.Errorf("expected BufferSize=64, got %d", storedCfg.BufferSize)
	}

	// Verify signal still works
	c.Emit(context.Background(), sig, key.Field("123"))
	if !received {
		t.Error("expected event to be received")
	}
}

// TestApplyConfigGlobPattern verifies applying config with glob pattern.
func TestApplyConfigGlobPattern(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig1 := NewSignal("order.created", "Order created")
	sig2 := NewSignal("order.shipped", "Order shipped")
	key := NewStringKey("id")

	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	cfg := Config{
		Signals: map[string]SignalConfig{
			"order.*": {BufferSize: 128, MinSeverity: SeverityWarn},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Verify both signals got the config
	c.mu.RLock()
	cfg1 := c.resolveConfig(sig1)
	cfg2 := c.resolveConfig(sig2)
	c.mu.RUnlock()

	if cfg1.BufferSize != 128 {
		t.Errorf("expected order.created BufferSize=128, got %d", cfg1.BufferSize)
	}
	if cfg2.BufferSize != 128 {
		t.Errorf("expected order.shipped BufferSize=128, got %d", cfg2.BufferSize)
	}

	// Emit to verify signals work
	c.Emit(context.Background(), sig1, key.Field("1"))
	c.Emit(context.Background(), sig2, key.Field("2"))
}

// TestApplyConfigGlobFutureSignal verifies glob applies to future signals.
func TestApplyConfigGlobFutureSignal(t *testing.T) {
	c := New()
	defer c.Shutdown()

	cfg := Config{
		Signals: map[string]SignalConfig{
			"future.*": {BufferSize: 256},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Create a new signal that matches the pattern
	sig := NewSignal("future.signal", "Future signal")
	key := NewStringKey("value")
	received := make(chan struct{})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		close(received)
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	// Wait for event
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the worker was created with the correct buffer size
	c.mu.RLock()
	resolvedCfg := c.resolveConfig(sig)
	c.mu.RUnlock()

	if resolvedCfg.BufferSize != 256 {
		t.Errorf("expected BufferSize=256, got %d", resolvedCfg.BufferSize)
	}
}

// TestApplyConfigDisabled verifies disabled signals drop events.
func TestApplyConfigDisabled(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("disabled.signal", "Disabled signal")
	key := NewStringKey("value")
	var received bool

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received = true
	})

	cfg := Config{
		Signals: map[string]SignalConfig{
			"disabled.signal": {Disabled: true},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	c.Emit(context.Background(), sig, key.Field("test"))

	if received {
		t.Error("event should have been dropped for disabled signal")
	}

	// Check dropped events counter
	stats := c.Stats()
	if stats.DroppedEvents == 0 {
		t.Error("expected DroppedEvents > 0")
	}
}

// TestApplyConfigMinSeverity verifies min severity filtering.
func TestApplyConfigMinSeverity(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("severity.test", "Severity test")
	key := NewStringKey("value")
	var receivedCount int

	c.Hook(sig, func(_ context.Context, _ *Event) {
		receivedCount++
	})

	cfg := Config{
		Signals: map[string]SignalConfig{
			"severity.test": {MinSeverity: SeverityWarn},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Debug should be filtered
	c.Debug(context.Background(), sig, key.Field("debug"))
	// Info should be filtered
	c.Info(context.Background(), sig, key.Field("info"))
	// Warn should pass
	c.Warn(context.Background(), sig, key.Field("warn"))
	// Error should pass
	c.Error(context.Background(), sig, key.Field("error"))

	if receivedCount != 2 {
		t.Errorf("expected 2 events (Warn + Error), got %d", receivedCount)
	}
}

// TestApplyConfigMaxListeners verifies max listeners limit.
func TestApplyConfigMaxListeners(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("maxlisteners.test", "Max listeners test")

	cfg := Config{
		Signals: map[string]SignalConfig{
			"maxlisteners.test": {MaxListeners: 2},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	l1 := c.Hook(sig, func(_ context.Context, _ *Event) {})
	l2 := c.Hook(sig, func(_ context.Context, _ *Event) {})
	l3 := c.Hook(sig, func(_ context.Context, _ *Event) {})

	if l1 == nil {
		t.Error("first listener should not be nil")
	}
	if l2 == nil {
		t.Error("second listener should not be nil")
	}
	if l3 != nil {
		t.Error("third listener should be nil (max exceeded)")
	}

	// Verify listener count
	stats := c.Stats()
	if stats.ListenerCounts[sig] != 2 {
		t.Errorf("expected 2 listeners, got %d", stats.ListenerCounts[sig])
	}
}

// TestApplyConfigDropPolicyDropNewest verifies drop_newest policy.
func TestApplyConfigDropPolicyDropNewest(t *testing.T) {
	c := New(WithBufferSize(2)) // Small buffer
	defer c.Shutdown()

	sig := NewSignal("dropnewest.test", "Drop newest test")
	key := NewStringKey("value")

	// Create slow handler to fill buffer
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		close(handlerStarted)
		<-handlerDone
	})

	cfg := Config{
		Signals: map[string]SignalConfig{
			"dropnewest.test": {DropPolicy: DropPolicyDropNewest, BufferSize: 2},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Emit first event (will be processed, blocking handler)
	c.Emit(context.Background(), sig, key.Field("1"))
	<-handlerStarted

	// Emit events to fill buffer + overflow
	for i := 2; i <= 5; i++ {
		c.Emit(context.Background(), sig, key.Field("overflow"))
	}

	// Let handler finish
	close(handlerDone)
	c.Shutdown()

	// Some events should have been dropped
	stats := c.Stats()
	if stats.DroppedEvents == 0 {
		t.Error("expected some dropped events with drop_newest policy")
	}
}

// TestApplyConfigRateLimit verifies rate limiting.
func TestApplyConfigRateLimit(t *testing.T) {
	// Rate limiting requires worker state, so we use async mode
	c := New()
	defer c.Shutdown()

	sig := NewSignal("ratelimit.test", "Rate limit test")
	key := NewStringKey("value")
	var mu sync.Mutex
	var receivedCount int

	// Apply config first so worker is created with rate limiter
	cfg := Config{
		Signals: map[string]SignalConfig{
			"ratelimit.test": {RateLimit: 2, BurstSize: 3}, // 2 events/sec, burst of 3
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
	})

	// Rapid fire 10 events - only burst should get through
	for i := 0; i < 10; i++ {
		c.Emit(context.Background(), sig, key.Field("test"))
	}

	// Drain to ensure all events are processed
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("Drain failed: %v", err)
	}

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	// Should receive at most BurstSize events
	if count > 3 {
		t.Errorf("expected at most 3 events (burst), got %d", count)
	}
	if count == 0 {
		t.Error("expected at least 1 event to get through")
	}
}

// TestApplyConfigWorkerRebuild verifies worker is rebuilt on config change.
func TestApplyConfigWorkerRebuild(t *testing.T) {
	c := New(WithBufferSize(8))
	defer c.Shutdown()

	sig := NewSignal("rebuild.test", "Rebuild test")
	key := NewStringKey("value")
	received := make(chan struct{}, 10)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received <- struct{}{}
	})

	// Emit to create worker with default buffer
	c.Emit(context.Background(), sig, key.Field("1"))
	<-received

	// Apply config with different buffer size
	cfg := Config{
		Signals: map[string]SignalConfig{
			"rebuild.test": {BufferSize: 64},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Emit more events - should work with rebuilt worker
	for i := 0; i < 3; i++ {
		c.Emit(context.Background(), sig, key.Field("after"))
	}

	// Collect events
	timeout := time.After(time.Second)
	count := 0
	for count < 3 {
		select {
		case <-received:
			count++
		case <-timeout:
			t.Fatalf("timeout: only received %d events", count)
		}
	}
}

// TestApplyConfigGlobOverridesExact verifies glob clears matching exact configs.
func TestApplyConfigGlobOverridesExact(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("order.created", "Order created")
	c.Hook(sig, func(_ context.Context, _ *Event) {})

	// First apply exact config
	cfg1 := Config{
		Signals: map[string]SignalConfig{
			"order.created": {BufferSize: 32},
		},
	}
	if err := c.ApplyConfig(cfg1); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	c.mu.RLock()
	buf1 := c.resolveConfig(sig).BufferSize
	c.mu.RUnlock()
	if buf1 != 32 {
		t.Errorf("expected BufferSize=32, got %d", buf1)
	}

	// Apply glob - should replace (no exact match anymore, so glob applies)
	cfg2 := Config{
		Signals: map[string]SignalConfig{
			"order.*": {BufferSize: 128},
		},
	}
	if err := c.ApplyConfig(cfg2); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	c.mu.RLock()
	buf2 := c.resolveConfig(sig).BufferSize
	c.mu.RUnlock()
	if buf2 != 128 {
		t.Errorf("expected BufferSize=128 after glob, got %d", buf2)
	}
}

// TestSeverityAtLeast verifies severity ordering.
func TestSeverityAtLeast(t *testing.T) {
	tests := []struct {
		s, min Severity
		want   bool
	}{
		{SeverityDebug, SeverityDebug, true},
		{SeverityDebug, SeverityInfo, false},
		{SeverityInfo, SeverityDebug, true},
		{SeverityInfo, SeverityInfo, true},
		{SeverityWarn, SeverityInfo, true},
		{SeverityError, SeverityWarn, true},
		{SeverityWarn, SeverityError, false},
	}

	for _, tt := range tests {
		got := severityAtLeast(tt.s, tt.min)
		if got != tt.want {
			t.Errorf("severityAtLeast(%s, %s) = %v, want %v", tt.s, tt.min, got, tt.want)
		}
	}
}

// TestIsGlobPattern verifies glob pattern detection.
func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"order.created", false},
		{"order.*", true},
		{"order.?", true},
		{"order.[abc]", true},
		{"order.created.123", false},
	}

	for _, tt := range tests {
		got := isGlobPattern(tt.pattern)
		if got != tt.want {
			t.Errorf("isGlobPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

// TestConfigErrorMessage verifies ConfigError formatting.
func TestConfigErrorMessage(t *testing.T) {
	err := &ConfigError{
		Pattern: "order.*",
		Field:   "bufferSize",
		Reason:  "must be non-negative",
	}

	want := "config error for order.*: bufferSize must be non-negative"
	if got := err.Error(); got != want {
		t.Errorf("ConfigError.Error() = %q, want %q", got, want)
	}
}

// TestModuleLevelApplyConfig verifies the module-level ApplyConfig wrapper.
func TestModuleLevelApplyConfig(t *testing.T) {
	// Reset default instance for this test
	defaultOnce = sync.Once{}
	defaultCapitan = nil

	defer func() {
		if defaultCapitan != nil {
			defaultCapitan.Shutdown()
		}
		defaultOnce = sync.Once{}
		defaultCapitan = nil
	}()

	cfg := Config{
		Signals: map[string]SignalConfig{
			"test.signal": {BufferSize: 64},
		},
	}

	if err := ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Verify config was applied
	sig := NewSignal("test.signal", "Test")
	Default().mu.RLock()
	resolved := Default().resolveConfig(sig)
	Default().mu.RUnlock()

	if resolved.BufferSize != 64 {
		t.Errorf("expected BufferSize=64, got %d", resolved.BufferSize)
	}
}

// TestRebuildWorkerDisabled verifies worker is destroyed when config disables signal.
func TestRebuildWorkerDisabled(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("disable.test", "Disable test")
	received := make(chan struct{}, 10)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received <- struct{}{}
	})

	// Emit to create worker
	c.Emit(context.Background(), sig)

	// Wait for event
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Verify worker exists
	c.mu.RLock()
	_, hasWorker := c.workers[sig]
	c.mu.RUnlock()
	if !hasWorker {
		t.Fatal("expected worker to exist")
	}

	// Apply config that disables the signal
	cfg := Config{
		Signals: map[string]SignalConfig{
			"disable.test": {Disabled: true},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Give worker time to shut down
	time.Sleep(50 * time.Millisecond)

	// Verify worker was destroyed
	c.mu.RLock()
	_, hasWorker = c.workers[sig]
	c.mu.RUnlock()
	if hasWorker {
		t.Error("expected worker to be destroyed after disabling")
	}

	// Emit should now be dropped (no worker recreated for disabled signal)
	c.Emit(context.Background(), sig)

	select {
	case <-received:
		t.Error("expected event to be dropped for disabled signal")
	case <-time.After(100 * time.Millisecond):
		// Expected - event dropped
	}
}

// TestGlobSpecificityRules verifies glob resolution specificity.
func TestGlobSpecificityRules(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	// Config with overlapping globs
	cfg := Config{
		Signals: map[string]SignalConfig{
			"order.*":         {BufferSize: 32},  // shorter glob
			"order.payment.*": {BufferSize: 64},  // longer glob (more specific)
			"order.created":   {BufferSize: 128}, // exact match
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	tests := []struct {
		name       string
		signal     Signal
		wantBuffer int
	}{
		{"exact match wins", NewSignal("order.created", ""), 128},
		{"longer glob wins", NewSignal("order.payment.processed", ""), 64},
		{"shorter glob fallback", NewSignal("order.shipped", ""), 32},
		{"no match", NewSignal("user.created", ""), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.mu.RLock()
			resolved := c.resolveConfig(tt.signal)
			c.mu.RUnlock()

			if resolved.BufferSize != tt.wantBuffer {
				t.Errorf("resolveConfig(%s).BufferSize = %d, want %d",
					tt.signal.Name(), resolved.BufferSize, tt.wantBuffer)
			}
		})
	}
}

// TestConfigEqual verifies config comparison.
func TestConfigEqual(t *testing.T) {
	a := SignalConfig{BufferSize: 32, MinSeverity: SeverityInfo}
	b := SignalConfig{BufferSize: 32, MinSeverity: SeverityInfo}
	c := SignalConfig{BufferSize: 64, MinSeverity: SeverityInfo}

	if !configEqual(a, b) {
		t.Error("expected equal configs to be equal")
	}
	if configEqual(a, c) {
		t.Error("expected different configs to not be equal")
	}
}

// TestValidateNegativeValues verifies validation catches all negative values.
func TestValidateNegativeValues(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		field string
	}{
		{
			name:  "negative MaxListeners",
			cfg:   Config{Signals: map[string]SignalConfig{"test": {MaxListeners: -1}}},
			field: "maxListeners",
		},
		{
			name:  "negative RateLimit",
			cfg:   Config{Signals: map[string]SignalConfig{"test": {RateLimit: -1}}},
			field: "rateLimit",
		},
		{
			name:  "negative BurstSize",
			cfg:   Config{Signals: map[string]SignalConfig{"test": {BurstSize: -1}}},
			field: "burstSize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			var cfgErr *ConfigError
			if !errors.As(err, &cfgErr) {
				t.Fatalf("expected ConfigError, got %T", err)
			}
			if cfgErr.Field != tt.field {
				t.Errorf("expected field %q, got %q", tt.field, cfgErr.Field)
			}
		})
	}
}

// TestRebuildWorkerWithRateLimit verifies worker rebuild includes rate limiter.
func TestRebuildWorkerWithRateLimit(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("ratelimit.rebuild", "Rate limit rebuild test")
	received := make(chan struct{}, 100)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received <- struct{}{}
	})

	// Emit to create worker
	c.Emit(context.Background(), sig)
	<-received

	// Apply config with rate limit
	cfg := Config{
		Signals: map[string]SignalConfig{
			"ratelimit.rebuild": {RateLimit: 1000, BurstSize: 5},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Drain to ensure rebuild completes
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}

	// Verify worker has rate limiter configured
	c.mu.RLock()
	worker := c.workers[sig]
	c.mu.RUnlock()

	if worker == nil {
		t.Fatal("expected worker to exist")
	}
	if worker.config.RateLimit != 1000 {
		t.Errorf("expected RateLimit=1000, got %f", worker.config.RateLimit)
	}
}

// TestRebuildWorkerNoWorkerExists verifies rebuild handles missing worker.
func TestRebuildWorkerNoWorkerExists(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("no.worker", "No worker test")

	// Apply config for signal that has no worker yet
	cfg := Config{
		Signals: map[string]SignalConfig{
			"no.worker": {BufferSize: 64},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// No worker should exist (none was created)
	c.mu.RLock()
	_, exists := c.workers[sig]
	c.mu.RUnlock()

	if exists {
		t.Error("expected no worker to exist")
	}
}

// TestMinSeverityFilteringInWorker verifies MinSeverity filtering in async mode.
func TestMinSeverityFilteringInWorker(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("severity.filter", "Severity filter test")
	var mu sync.Mutex
	var received []Severity

	c.Hook(sig, func(_ context.Context, e *Event) {
		mu.Lock()
		received = append(received, e.Severity())
		mu.Unlock()
	})

	// Apply config with MinSeverity
	cfg := Config{
		Signals: map[string]SignalConfig{
			"severity.filter": {MinSeverity: SeverityWarn},
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Emit at various severities
	c.Debug(context.Background(), sig) // Should be filtered
	c.Info(context.Background(), sig)  // Should be filtered
	c.Warn(context.Background(), sig)  // Should pass
	c.Error(context.Background(), sig) // Should pass

	// Drain
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Errorf("expected 2 events (WARN, ERROR), got %d", len(received))
	}
}

// TestPanicHandlerInWorker verifies panic handler is called in async mode.
func TestPanicHandlerInWorker(t *testing.T) {
	var panicSignal Signal
	var panicValue any
	var mu sync.Mutex

	c := New(WithPanicHandler(func(sig Signal, recovered any) {
		mu.Lock()
		panicSignal = sig
		panicValue = recovered
		mu.Unlock()
	}))
	defer c.Shutdown()

	sig := NewSignal("panic.worker", "Panic test")

	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("test panic")
	})

	c.Emit(context.Background(), sig)

	// Drain to ensure event is processed
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if panicSignal.Name() != "panic.worker" {
		t.Errorf("expected panic signal 'panic.worker', got %q", panicSignal.Name())
	}
	if panicValue != "test panic" {
		t.Errorf("expected panic value 'test panic', got %v", panicValue)
	}
}

// TestApplyConfigValidationError verifies ApplyConfig returns validation errors.
func TestApplyConfigValidationError(t *testing.T) {
	c := New()
	defer c.Shutdown()

	cfg := Config{
		Signals: map[string]SignalConfig{
			"test": {BufferSize: -1}, // Invalid
		},
	}

	err := c.ApplyConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// TestRateLimitWithDefaultBurst verifies rate limiting with default burst (0 -> 1).
func TestRateLimitWithDefaultBurst(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("burst.default", "Default burst test")
	var mu sync.Mutex
	var count int

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	// Apply config with rate limit but no burst size (defaults to 1)
	cfg := Config{
		Signals: map[string]SignalConfig{
			"burst.default": {RateLimit: 1000}, // No BurstSize specified
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Emit several events
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should receive at least 1 event (the burst)
	if count == 0 {
		t.Error("expected at least 1 event")
	}
}

// TestRebuildWorkerWithDefaultBurst verifies rebuild with default burst.
func TestRebuildWorkerWithDefaultBurst(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("rebuild.burst", "Rebuild burst test")
	received := make(chan struct{}, 10)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received <- struct{}{}
	})

	// Create worker first
	c.Emit(context.Background(), sig)
	<-received

	// Apply config with rate limit but no burst (defaults to 1)
	cfg := Config{
		Signals: map[string]SignalConfig{
			"rebuild.burst": {RateLimit: 100}, // No BurstSize
		},
	}
	if err := c.ApplyConfig(cfg); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}

	// Drain
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Drain(ctx); err != nil {
		t.Fatalf("drain failed: %v", err)
	}

	// Worker should have been rebuilt with rate limiter
	c.mu.RLock()
	worker := c.workers[sig]
	c.mu.RUnlock()

	if worker == nil {
		t.Fatal("expected worker")
	}
	if worker.config.RateLimit != 100 {
		t.Errorf("expected RateLimit=100, got %f", worker.config.RateLimit)
	}
}

// TestRebuildWorkerLockedNoWorker directly tests the !exists guard.
func TestRebuildWorkerLockedNoWorker(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("no.worker.rebuild", "No worker rebuild")

	// Directly call rebuildWorkerLocked for a signal with no worker
	c.mu.Lock()
	c.rebuildWorkerLocked(sig) // Should return early, not panic
	c.mu.Unlock()

	// Verify no worker was created
	c.mu.RLock()
	_, exists := c.workers[sig]
	c.mu.RUnlock()

	if exists {
		t.Error("expected no worker")
	}
}

// TestReplayEventWithCanceledContext verifies replay events with canceled context.
func TestReplayEventWithCanceledContext(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("replay.canceled", "Replay canceled test")
	var received bool

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received = true
	})

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create and replay an event with canceled context
	e := NewEvent(sig, SeverityInfo, time.Now())
	c.Replay(ctx, e)

	// Event should be dropped due to canceled context
	if received {
		t.Error("expected event to be dropped due to canceled context")
	}
}

// TestSyncModeEmitWithCanceledContext tests emit with canceled context in sync mode.
func TestSyncModeEmitWithCanceledContext(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("sync.canceled", "Sync canceled test")
	var received bool

	c.Hook(sig, func(_ context.Context, _ *Event) {
		received = true
	})

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Emit with canceled context - event should be dropped
	c.Emit(ctx, sig)

	if received {
		t.Error("expected event to be dropped due to canceled context")
	}
}
