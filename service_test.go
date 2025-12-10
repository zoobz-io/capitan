package capitan

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New(WithSyncMode())
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.registry == nil {
		t.Error("registry not initialized")
	}
	if c.workers == nil {
		t.Error("workers not initialized")
	}
	if c.shutdown == nil {
		t.Error("shutdown channel not initialized")
	}
}

func TestEmitCreatesWorkerLazily(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.lazy", "Test lazy signal")
	key := NewStringKey("value")

	// Before emit, no worker exists
	c.mu.RLock()
	_, exists := c.workers[sig]
	c.mu.RUnlock()

	if exists {
		t.Error("worker should not exist before first emit")
	}

	var wg sync.WaitGroup
	wg.Add(1)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		wg.Done()
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	// After emit, worker should exist
	c.mu.RLock()
	_, exists = c.workers[sig]
	c.mu.RUnlock()

	if !exists {
		t.Error("worker should exist after first emit")
	}

	wg.Wait()
}

// Note: TestEmitCreatesWorkerLazily keeps async mode because it specifically tests worker creation

func TestEmitAsyncProcessing(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.async", "Test async signal")
	key := NewIntKey("value")

	const numEmissions = 100
	received := make([]int, 0, numEmissions)

	c.Hook(sig, func(_ context.Context, e *Event) {
		field := e.Get(key).(GenericField[int])
		received = append(received, field.Get())
	})

	// Emit many events
	for i := 0; i < numEmissions; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	if len(received) != numEmissions {
		t.Errorf("expected %d events, got %d", numEmissions, len(received))
	}
}

func TestEmitWithMultipleListeners(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.multi", "Test multi signal")
	key := NewStringKey("value")

	var received1, received2 string

	c.Hook(sig, func(_ context.Context, e *Event) {
		field := e.Get(key).(GenericField[string])
		received1 = field.Get()
	})

	c.Hook(sig, func(_ context.Context, e *Event) {
		field := e.Get(key).(GenericField[string])
		received2 = field.Get()
	})

	c.Emit(context.Background(), sig, key.Field("hello"))

	if received1 != "hello" {
		t.Errorf("listener 1: expected %q, got %q", "hello", received1)
	}
	if received2 != "hello" {
		t.Errorf("listener 2: expected %q, got %q", "hello", received2)
	}
}

func TestEmitWithNoListeners(_ *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.nolisteners", "Test no listeners signal")
	key := NewStringKey("value")

	// Should not panic or block
	c.Emit(context.Background(), sig, key.Field("test"))
}

func TestEmitWithPanicRecovery(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.panic", "Test panic signal")
	key := NewStringKey("value")

	var executed bool

	// First listener panics
	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("intentional panic")
	})

	// Second listener should still execute
	c.Hook(sig, func(_ context.Context, _ *Event) {
		executed = true
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if !executed {
		t.Fatal("second listener never executed - panic not recovered")
	}
}

func TestHook(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.hook", "Test hook signal")
	key := NewStringKey("value")

	var received *Event

	listener := c.Hook(sig, func(_ context.Context, e *Event) {
		received = e
	})

	if listener == nil {
		t.Fatal("Hook returned nil")
	}

	c.Emit(context.Background(), sig, key.Field("test"))

	if received == nil {
		t.Error("event not received")
	}
}

func TestObserve(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig1 := NewSignal("test.sig1", "Test signal 1")
	sig2 := NewSignal("test.sig2", "Test signal 2")
	key := NewStringKey("msg")

	// Create hooks first so signals exist in registry
	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	var received []Signal

	observer := c.Observe(func(_ context.Context, e *Event) {
		received = append(received, e.Signal())
	})

	if observer == nil {
		t.Fatal("Observe returned nil")
	}

	c.Emit(context.Background(), sig1, key.Field("first"))
	c.Emit(context.Background(), sig2, key.Field("second"))

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	// Order not guaranteed, check both present
	found1, found2 := false, false
	for _, sig := range received {
		if sig == sig1 {
			found1 = true
		}
		if sig == sig2 {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("expected both signals, got %v", received)
	}
}

func TestShutdown(_ *testing.T) {
	c := New()

	sig := NewSignal("test.shutdown", "Test shutdown signal")
	key := NewStringKey("value")

	var wg sync.WaitGroup
	wg.Add(1)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		wg.Done()
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	// Shutdown should wait for in-flight events
	c.Shutdown()

	// If shutdown didn't wait, this would fail
	wg.Wait()
}

func TestModuleLevelAPI(t *testing.T) {
	sig := NewSignal("test.module", "Test module signal")
	key := NewStringKey("value")

	var received *Event
	var wg sync.WaitGroup
	wg.Add(1)

	listener := Hook(sig, func(_ context.Context, e *Event) {
		received = e
		wg.Done()
	})

	if listener == nil {
		t.Fatal("Hook returned nil")
	}

	Emit(context.Background(), sig, key.Field("test"))

	wg.Wait()

	if received == nil {
		t.Error("event not received")
	}
	if received.Signal() != sig {
		t.Errorf("expected signal %q, got %q", sig, received.Signal())
	}
}

func TestListenerReceivesContext(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.ctx.value", "Test context value signal")
	key := NewStringKey("value")

	type ctxKey string
	const testKey ctxKey = "test_key"
	expectedValue := "test_value"

	var received string

	c.Hook(sig, func(ctx context.Context, _ *Event) {
		received = ctx.Value(testKey).(string)
	})

	ctx := context.WithValue(context.Background(), testKey, expectedValue)
	c.Emit(ctx, sig, key.Field("test"))

	if received != expectedValue {
		t.Errorf("expected %q, got %q", expectedValue, received)
	}
}

func TestContextIsolationPerSignal(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig1 := NewSignal("test.iso.one", "Test isolation signal 1")
	sig2 := NewSignal("test.iso.two", "Test isolation signal 2")
	key := NewStringKey("value")

	type ctxKey string
	const valKey ctxKey = "val"

	var val1, val2 string

	c.Hook(sig1, func(ctx context.Context, _ *Event) {
		val1 = ctx.Value(valKey).(string)
	})

	c.Hook(sig2, func(ctx context.Context, _ *Event) {
		val2 = ctx.Value(valKey).(string)
	})

	ctx1 := context.WithValue(context.Background(), valKey, "A")
	ctx2 := context.WithValue(context.Background(), valKey, "B")

	c.Emit(ctx1, sig1, key.Field("first"))
	c.Emit(ctx2, sig2, key.Field("second"))

	if val1 != "A" {
		t.Errorf("sig1: expected %q, got %q", "A", val1)
	}
	if val2 != "B" {
		t.Errorf("sig2: expected %q, got %q", "B", val2)
	}
}

func TestModuleLevelConfigure(t *testing.T) {
	// Test that Configure can be called without panicking
	// Note: Configure only affects the default instance if called before first use
	// Since other tests may have already used the default instance, we just verify
	// that Configure is callable. Actual option behavior is tested in config_test.go

	Configure(
		WithBufferSize(32),
		WithPanicHandler(func(_ Signal, _ any) {
			// Handler for coverage
		}),
	)

	// Verify the default instance still works after Configure
	sig := NewSignal("test.configure", "Test configure signal")
	key := NewStringKey("value")

	var received bool
	var wg sync.WaitGroup
	wg.Add(1)

	Hook(sig, func(_ context.Context, _ *Event) {
		received = true
		wg.Done()
	})

	Emit(context.Background(), sig, key.Field("test"))

	wg.Wait()

	if !received {
		t.Error("event not received after Configure")
	}
}

func TestModuleLevelObserve(t *testing.T) {
	sig1 := NewSignal("test.observe.1", "Test observe signal 1")
	sig2 := NewSignal("test.observe.2", "Test observe signal 2")
	key := NewStringKey("msg")

	Hook(sig1, func(_ context.Context, _ *Event) {})
	Hook(sig2, func(_ context.Context, _ *Event) {})

	var received []Signal
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	observer := Observe(func(_ context.Context, e *Event) {
		mu.Lock()
		received = append(received, e.Signal())
		mu.Unlock()
		wg.Done()
	})

	if observer == nil {
		t.Fatal("Observe returned nil")
	}

	Emit(context.Background(), sig1, key.Field("first"))
	Emit(context.Background(), sig2, key.Field("second"))

	wg.Wait()

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
}

func TestModuleLevelSeverityMethods(t *testing.T) {
	sig := NewSignal("test.module.severity", "Test module severity signal")
	key := NewStringKey("value")

	tests := []struct {
		name     string
		emitFunc func(context.Context, Signal, ...Field)
		expected Severity
	}{
		{"Debug", Debug, SeverityDebug},
		{"Info", Info, SeverityInfo},
		{"Warn", Warn, SeverityWarn},
		{"Error", Error, SeverityError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedSeverity Severity
			var wg sync.WaitGroup
			wg.Add(1)

			listener := Hook(sig, func(_ context.Context, e *Event) {
				receivedSeverity = e.Severity()
				wg.Done()
			})
			defer listener.Close()

			tt.emitFunc(context.Background(), sig, key.Field("test"))
			wg.Wait()

			if receivedSeverity != tt.expected {
				t.Errorf("expected severity %q, got %q", tt.expected, receivedSeverity)
			}
		})
	}
}

func TestModuleLevelShutdown(t *testing.T) {
	sig := NewSignal("test.shutdown.module", "Test shutdown module signal")
	key := NewStringKey("value")

	var processed bool
	var wg sync.WaitGroup
	wg.Add(1)

	Hook(sig, func(_ context.Context, _ *Event) {
		time.Sleep(10 * time.Millisecond)
		processed = true
		wg.Done()
	})

	Emit(context.Background(), sig, key.Field("test"))

	// Shutdown should wait for event processing
	Shutdown()

	if !processed {
		t.Error("event not processed before shutdown")
	}

	wg.Wait()
}

func TestDefault(t *testing.T) {
	c := Default()
	if c == nil {
		t.Fatal("Default() returned nil")
	}

	// Verify it returns the same instance
	c2 := Default()
	if c != c2 {
		t.Error("Default() should return the same instance")
	}
}

func TestModuleLevelReplay(t *testing.T) {
	sig := NewSignal("test.module.replay", "Test module replay signal")
	orderID := NewStringKey("order_id")

	var receivedEvent *Event
	var wg sync.WaitGroup
	wg.Add(1)

	listener := Hook(sig, func(_ context.Context, e *Event) {
		receivedEvent = e
		wg.Done()
	})
	defer listener.Close()

	// Create a historical event
	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	e := NewEvent(sig, SeverityError, timestamp,
		orderID.Field("ORD-789"),
	)

	// Replay via module-level function
	Replay(context.Background(), e)

	wg.Wait()

	if receivedEvent == nil {
		t.Fatal("replayed event not received via module-level Replay")
	}

	if !receivedEvent.IsReplay() {
		t.Error("event should be marked as replay")
	}

	if receivedEvent.Severity() != SeverityError {
		t.Errorf("expected severity %v, got %v", SeverityError, receivedEvent.Severity())
	}

	id, ok := orderID.From(receivedEvent)
	if !ok || id != "ORD-789" {
		t.Errorf("expected order_id %q, got %q", "ORD-789", id)
	}
}
