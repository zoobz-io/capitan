package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobz-io/capitan"
	capitantesting "github.com/zoobz-io/capitan/testing"
)

// TestConcurrency_MultipleGoroutinesEmitting tests concurrent event emission.
func TestConcurrency_MultipleGoroutinesEmitting(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig := capitan.NewSignal("test.concurrent.emit", "Concurrent emission test")
	key := capitan.NewStringKey("goroutine")

	capture := capitantesting.NewEventCapture()
	c.Hook(sig, capture.Handler())

	const numGoroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				c.Emit(context.Background(), sig, key.Field(string(rune('A'+id))))
			}
		}(i)
	}

	wg.Wait()
	c.Shutdown()

	// Verify all events received
	events := capture.Events()
	expectedCount := numGoroutines * eventsPerGoroutine
	if len(events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(events))
	}
}

// TestConcurrency_HookAndEmitSimultaneous tests concurrent hook and emit operations.
func TestConcurrency_HookAndEmitSimultaneous(t *testing.T) {
	c := capitantesting.TestCapitan()

	const numSignals = 20
	const duration = 100 * time.Millisecond

	var emitCount atomic.Int64
	var hookCount atomic.Int64

	var wg sync.WaitGroup

	// Goroutine 1: Hook listeners continuously
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(duration)
		for i := 0; time.Now().Before(deadline); i++ {
			sig := capitan.NewSignal("test.hook."+string(rune('a'+i%numSignals)), "Test signal")
			c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})
			hookCount.Add(1)
			time.Sleep(time.Microsecond * 100)
		}
	}()

	// Goroutine 2: Emit events continuously
	wg.Add(1)
	go func() {
		defer wg.Done()
		key := capitan.NewIntKey("value")
		deadline := time.Now().Add(duration)
		for i := 0; time.Now().Before(deadline); i++ {
			sig := capitan.NewSignal("test.hook."+string(rune('a'+i%numSignals)), "Test signal")
			c.Emit(context.Background(), sig, key.Field(i))
			emitCount.Add(1)
			time.Sleep(time.Microsecond * 100)
		}
	}()

	wg.Wait()
	c.Shutdown()

	// Just verify operations completed without deadlock
	if hookCount.Load() == 0 {
		t.Error("no hooks were created")
	}
	if emitCount.Load() == 0 {
		t.Error("no events were emitted")
	}

	t.Logf("Completed %d hooks and %d emits concurrently", hookCount.Load(), emitCount.Load())
}

// TestConcurrency_ObserverCreationDuringEmission tests observer creation while emitting.
func TestConcurrency_ObserverCreationDuringEmission(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig1 := capitan.NewSignal("test.observer.1", "Test signal 1")
	sig2 := capitan.NewSignal("test.observer.2", "Test signal 2")

	// Pre-create signals
	c.Hook(sig1, func(_ context.Context, _ *capitan.Event) {})
	c.Hook(sig2, func(_ context.Context, _ *capitan.Event) {})

	const duration = 100 * time.Millisecond

	var wg sync.WaitGroup

	// Goroutine 1: Create and close observers
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(duration)
		for time.Now().Before(deadline) {
			obs := c.Observe(func(_ context.Context, _ *capitan.Event) {})
			time.Sleep(time.Microsecond * 500)
			obs.Close()
		}
	}()

	// Goroutine 2: Emit events
	wg.Add(1)
	go func() {
		defer wg.Done()
		key := capitan.NewIntKey("value")
		deadline := time.Now().Add(duration)
		for i := 0; time.Now().Before(deadline); i++ {
			c.Emit(context.Background(), sig1, key.Field(i))
			c.Emit(context.Background(), sig2, key.Field(i))
			time.Sleep(time.Microsecond * 200)
		}
	}()

	wg.Wait()
	c.Shutdown()

	// Just verify no deadlock/panic
	t.Log("Observer creation and emission completed without deadlock")
}

// TestConcurrency_ListenerCloseDuringEmission tests closing listeners while events are being emitted.
func TestConcurrency_ListenerCloseDuringEmission(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig := capitan.NewSignal("test.close.concurrent", "Test concurrent close")
	key := capitan.NewIntKey("value")

	const numListeners = 10
	listeners := make([]*capitan.Listener, numListeners)

	counter := capitantesting.NewEventCounter()
	for i := 0; i < numListeners; i++ {
		listeners[i] = c.Hook(sig, counter.Handler())
	}

	var wg sync.WaitGroup

	// Goroutine 1: Emit events
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			c.Emit(context.Background(), sig, key.Field(i))
			time.Sleep(time.Microsecond * 100)
		}
	}()

	// Goroutine 2: Close listeners one by one
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Let some events process
		for _, listener := range listeners {
			listener.Close()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()
	c.Shutdown()

	// We should have received some events (not all, since we closed listeners)
	count := counter.Count()
	if count == 0 {
		t.Error("expected some events to be received before listeners closed")
	}
	t.Logf("Received %d events with concurrent listener closure", count)
}

// TestConcurrency_WorkerSaturation tests behavior when worker queues fill up.
func TestConcurrency_WorkerSaturation(t *testing.T) {
	// Use small buffer to force saturation
	c := capitan.New(capitan.WithBufferSize(5))

	sig := capitan.NewSignal("test.saturation", "Test saturation")
	key := capitan.NewIntKey("value")

	// Slow listener to cause backpressure
	var processed atomic.Int64
	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		processed.Add(1)
		time.Sleep(5 * time.Millisecond) // Slow processing
	})

	// Emit many events quickly
	const numEvents = 50
	for i := 0; i < numEvents; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	c.Shutdown()

	// All events should eventually be processed
	if processed.Load() != numEvents {
		t.Errorf("expected %d processed events, got %d", numEvents, processed.Load())
	}
}

// TestConcurrency_MultipleSignalsParallel tests independent signal workers processing in parallel.
func TestConcurrency_MultipleSignalsParallel(t *testing.T) {
	c := capitantesting.TestCapitan()

	const numSignals = 10
	const eventsPerSignal = 100

	signals := make([]capitan.Signal, numSignals)
	counters := make([]*capitantesting.EventCounter, numSignals)

	// Create signals with separate counters
	for i := 0; i < numSignals; i++ {
		signals[i] = capitan.NewSignal("test.parallel."+string(rune('a'+i)), "Test parallel signal")
		counters[i] = capitantesting.NewEventCounter()
		c.Hook(signals[i], counters[i].Handler())
	}

	key := capitan.NewIntKey("value")

	// Emit to all signals in parallel
	var wg sync.WaitGroup
	for i := 0; i < numSignals; i++ {
		wg.Add(1)
		go func(signalIdx int) {
			defer wg.Done()
			for j := 0; j < eventsPerSignal; j++ {
				c.Emit(context.Background(), signals[signalIdx], key.Field(j))
			}
		}(i)
	}

	wg.Wait()
	c.Shutdown()

	// Verify each signal processed all its events
	for i := 0; i < numSignals; i++ {
		count := counters[i].Count()
		if count != eventsPerSignal {
			t.Errorf("signal %d: expected %d events, got %d", i, eventsPerSignal, count)
		}
	}
}

// TestConcurrency_ShutdownUnderLoad tests shutdown behavior with many in-flight events.
func TestConcurrency_ShutdownUnderLoad(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig := capitan.NewSignal("test.shutdown.load", "Test shutdown under load")
	key := capitan.NewIntKey("value")

	counter := capitantesting.NewEventCounter()
	c.Hook(sig, counter.Handler())

	// Emit many events in parallel
	const numGoroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				c.Emit(context.Background(), sig, key.Field(j))
			}
		}()
	}

	wg.Wait()

	// Shutdown should drain all queued events
	c.Shutdown()

	expectedCount := numGoroutines * eventsPerGoroutine
	actualCount := counter.Count()
	if actualCount != int64(expectedCount) {
		t.Errorf("expected %d events processed, got %d (some events lost during shutdown)",
			expectedCount, actualCount)
	}
}

// TestConcurrency_ContextCancellationConcurrent tests context cancellation with concurrent emissions.
func TestConcurrency_ContextCancellationConcurrent(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig := capitan.NewSignal("test.ctx.cancel", "Test context cancellation")
	key := capitan.NewIntKey("value")

	var processed atomic.Int64
	var canceled atomic.Int64

	c.Hook(sig, func(ctx context.Context, _ *capitan.Event) {
		if ctx.Err() != nil {
			canceled.Add(1)
		} else {
			processed.Add(1)
		}
	})

	const numGoroutines = 5

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Half with canceled context, half with valid context
			ctx, cancel := context.WithCancel(context.Background())
			if id%2 == 0 {
				cancel() // Cancel immediately
			} else {
				defer cancel()
			}

			for j := 0; j < 50; j++ {
				c.Emit(ctx, sig, key.Field(j))
			}
		}(i)
	}

	wg.Wait()
	c.Shutdown()

	// Should have processed some events (canceled contexts may still process in race)
	total := processed.Load() + canceled.Load()
	if total == 0 {
		t.Error("expected some events to be received")
	}

	t.Logf("Processed: %d, Canceled: %d, Total: %d", processed.Load(), canceled.Load(), total)
}
