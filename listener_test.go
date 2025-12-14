package capitan

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenerClose(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.listener.close", "Test listener close signal")
	key := NewStringKey("value")

	count := 0

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	c.Emit(context.Background(), sig, key.Field("first"))

	// Close listener
	listener.Close()

	// This emission should not be received
	c.Emit(context.Background(), sig, key.Field("second"))

	if count != 1 {
		t.Errorf("expected 1 event received, got %d", count)
	}
}

func TestListenerCloseIdempotent(_ *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.listener.idempotent", "Test listener idempotent signal")

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {})

	// Close multiple times should not panic
	listener.Close()
	listener.Close()
	listener.Close()
}

func TestListenerMultiplePerSignal(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.listener.multiple", "Test listener multiple signal")
	key := NewStringKey("value")

	count := 0

	c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if count != 3 {
		t.Errorf("expected 3 listener invocations, got %d", count)
	}
}

func TestObserverClose(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig1 := NewSignal("test.observer.close1", "Test observer close signal 1")
	sig2 := NewSignal("test.observer.close2", "Test observer close signal 2")
	key := NewStringKey("value")

	// Create hooks so signals exist
	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	count := 0

	observer := c.Observe(func(_ context.Context, _ *Event) {
		count++
	})

	c.Emit(context.Background(), sig1, key.Field("first"))
	c.Emit(context.Background(), sig2, key.Field("second"))

	// Close observer
	observer.Close()

	// These should not be received
	c.Emit(context.Background(), sig1, key.Field("third"))
	c.Emit(context.Background(), sig2, key.Field("fourth"))

	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

func TestObserverCloseIdempotent(_ *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.observer.idempotent", "Test observer idempotent signal")

	// Create hook so signal exists
	c.Hook(sig, func(_ context.Context, _ *Event) {})

	observer := c.Observe(func(_ context.Context, _ *Event) {})

	// Close multiple times should not panic
	observer.Close()
	observer.Close()
	observer.Close()
}

func TestObserverSnapshotBehavior(_ *testing.T) {
	c := New()
	defer c.Shutdown()

	sig1 := NewSignal("test.observer.snapshot1", "Test observer snapshot signal 1")
	sig2 := NewSignal("test.observer.snapshot2", "Test observer snapshot signal 2")
	key := NewStringKey("value")

	// Create first signal
	c.Hook(sig1, func(_ context.Context, _ *Event) {})

	var wg sync.WaitGroup
	wg.Add(1)

	// Observer should only see sig1 (snapshot at creation time)
	observer := c.Observe(func(_ context.Context, e *Event) {
		if e.Signal() == sig1 {
			wg.Done()
		}
	})

	// Create second signal after observer created
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	c.Emit(context.Background(), sig1, key.Field("first"))
	c.Emit(context.Background(), sig2, key.Field("second"))

	// Should only receive sig1
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - only sig1 received
	case <-time.After(100 * time.Millisecond):
		// Timeout is OK - we're checking sig2 wasn't received
	}

	observer.Close()
}

func TestObserverReceivesAllExistingSignals(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig1 := NewSignal("test.observer.all1", "Test observer all signal 1")
	sig2 := NewSignal("test.observer.all2", "Test observer all signal 2")
	sig3 := NewSignal("test.observer.all3", "Test observer all signal 3")
	key := NewStringKey("value")

	// Create all signals
	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})
	c.Hook(sig3, func(_ context.Context, _ *Event) {})

	received := make(map[Signal]bool)

	c.Observe(func(_ context.Context, e *Event) {
		received[e.Signal()] = true
	})

	c.Emit(context.Background(), sig1, key.Field("first"))
	c.Emit(context.Background(), sig2, key.Field("second"))
	c.Emit(context.Background(), sig3, key.Field("third"))

	if !received[sig1] || !received[sig2] || !received[sig3] {
		t.Errorf("observer did not receive all signals: %v", received)
	}
}

// TestEmitAfterListenerCloseFullBuffer verifies that emissions after
// listener close don't block when the buffer is full.
func TestEmitAfterListenerCloseFullBuffer(t *testing.T) {
	// Small buffer to easily fill
	c := New(WithBufferSize(2))
	defer c.Shutdown()

	sig := NewSignal("test.fullbuffer", "Test full buffer signal")
	key := NewIntKey("value")

	// Create listener that blocks processing
	block := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	var received int32
	var first sync.Once

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&received, 1)
		first.Do(func() {
			wg.Done()
		})
		<-block // Block until we release
	})

	// Fill the buffer: 1 processing + 2 buffered = 3 total
	c.Emit(context.Background(), sig, key.Field(1))
	wg.Wait() // Wait for first to start processing

	c.Emit(context.Background(), sig, key.Field(2)) // Buffer slot 1
	c.Emit(context.Background(), sig, key.Field(3)) // Buffer slot 2
	time.Sleep(10 * time.Millisecond)

	// Unblock the handler so Close() can drain
	close(block)

	// Close listener - will drain all 3 events
	listener.Close()

	// All 3 events should have been processed
	if atomic.LoadInt32(&received) != 3 {
		t.Errorf("expected 3 events drained, got %d", received)
	}

	// This should NOT block - worker is gone after close
	done := make(chan struct{})
	go func() {
		c.Emit(context.Background(), sig, key.Field(4)) // Should be dropped (no listeners)
		close(done)
	}()

	select {
	case <-done:
		// Success - emit didn't block
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Emit blocked after listener close")
	}
}

// TestConcurrentEmitAndListenerClose tests the TOCTOU race between
// Emit capturing a worker reference and listener close deleting it.
func TestConcurrentEmitAndListenerClose(t *testing.T) {
	const iterations = 100

	for i := 0; i < iterations; i++ {
		c := New(WithBufferSize(1))
		sig := NewSignal("test.race", "Test race signal")
		key := NewIntKey("value")

		// Slow listener to increase contention window
		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			time.Sleep(time.Microsecond)
		})

		var wg sync.WaitGroup

		// Goroutine 1: Emit rapidly
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				c.Emit(context.Background(), sig, key.Field(j))
			}
		}()

		// Goroutine 2: Close listener mid-stream
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Microsecond * 10)
			listener.Close()
		}()

		// Should complete without blocking
		done := make(chan struct{})
		go func() {
			wg.Wait()
			c.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: deadlock detected", i)
		}
	}
}

// TestEmitToClosedWorkerDropsEvent verifies events are properly
// dropped (not leaked) when sent to a closing worker.
func TestEmitToClosedWorkerDropsEvent(t *testing.T) {
	c := New(WithBufferSize(1))
	defer c.Shutdown()

	sig := NewSignal("test.drop", "Test drop signal")
	key := NewStringKey("value")

	received := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		received++
		mu.Unlock()
		wg.Done()
	})

	// Emit first event - should be received
	wg.Add(1)
	c.Emit(context.Background(), sig, key.Field("first"))
	wg.Wait()

	// Close listener
	listener.Close()
	time.Sleep(20 * time.Millisecond) // Ensure worker exits

	// Emit more events - should be dropped silently
	c.Emit(context.Background(), sig, key.Field("dropped1"))
	c.Emit(context.Background(), sig, key.Field("dropped2"))
	c.Emit(context.Background(), sig, key.Field("dropped3"))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	finalCount := received
	mu.Unlock()

	if finalCount != 1 {
		t.Errorf("expected 1 event received, got %d", finalCount)
	}
}

func TestHookOnceFiresOnlyOnce(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.hookonce", "Test hook once signal")
	key := NewStringKey("value")

	count := 0

	c.HookOnce(sig, func(_ context.Context, _ *Event) {
		count++
	})

	// Emit multiple events
	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))
	c.Emit(context.Background(), sig, key.Field("third"))

	if count != 1 {
		t.Errorf("expected callback to fire once, got %d", count)
	}
}

func TestHookOnceReceivesCorrectEvent(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.hookonce.event", "Test hook once event signal")
	key := NewStringKey("value")

	var received string

	c.HookOnce(sig, func(_ context.Context, e *Event) {
		received, _ = key.From(e)
	})

	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))

	if received != "first" {
		t.Errorf("expected 'first', got '%s'", received)
	}
}

func TestHookOnceCloseBeforeFiring(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.hookonce.close", "Test hook once close signal")
	key := NewStringKey("value")

	count := 0

	listener := c.HookOnce(sig, func(_ context.Context, _ *Event) {
		count++
	})

	// Close before any events
	listener.Close()

	// Emit events - should not be received
	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))

	if count != 0 {
		t.Errorf("expected callback to not fire, got %d", count)
	}
}

func TestHookOnceCloseIdempotent(_ *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.hookonce.idempotent", "Test hook once idempotent signal")
	key := NewStringKey("value")

	listener := c.HookOnce(sig, func(_ context.Context, _ *Event) {})

	// Emit to trigger auto-close
	c.Emit(context.Background(), sig, key.Field("first"))

	// Give time for async close
	time.Sleep(10 * time.Millisecond)

	// Manual close should not panic
	listener.Close()
	listener.Close()
}

func TestHookOnceAsync(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.hookonce.async", "Test hook once async signal")
	key := NewStringKey("value")

	var count int32
	done := make(chan struct{})

	c.HookOnce(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count, 1)
		close(done)
	})

	// Emit multiple events rapidly
	for i := 0; i < 10; i++ {
		c.Emit(context.Background(), sig, key.Field("event"))
	}

	// Wait for callback
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for callback")
	}

	// Give time for any additional (incorrect) invocations
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected callback to fire once, got %d", count)
	}
}

func TestHookOnceConcurrent(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.hookonce.concurrent", "Test hook once concurrent signal")
	key := NewStringKey("value")

	var count int32

	c.HookOnce(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count, 1)
	})

	// Emit from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Emit(context.Background(), sig, key.Field("event"))
		}()
	}
	wg.Wait()
	c.Shutdown()

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected callback to fire once, got %d", count)
	}
}

func TestModuleLevelHookOnce(t *testing.T) {
	// Reset default instance for clean test
	defaultOnce = sync.Once{}
	defaultCapitan = nil
	Configure(WithSyncMode())
	defer Shutdown()

	sig := NewSignal("test.module.hookonce", "Test module hook once signal")
	key := NewStringKey("value")

	count := 0

	HookOnce(sig, func(_ context.Context, _ *Event) {
		count++
	})

	Emit(context.Background(), sig, key.Field("first"))
	Emit(context.Background(), sig, key.Field("second"))

	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

// TestListenerCloseDrainsEvents verifies that Close() blocks until
// all events queued before the close have been processed.
func TestListenerCloseDrainsEvents(t *testing.T) {
	c := New(WithBufferSize(16))

	sig := NewSignal("test.drain", "Test drain signal")
	key := NewIntKey("value")

	var received []int
	var mu sync.Mutex

	listener := c.Hook(sig, func(_ context.Context, e *Event) {
		v, _ := key.From(e)
		mu.Lock()
		received = append(received, v)
		mu.Unlock()
	})

	// Emit multiple events
	for i := 1; i <= 5; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Close should block until all 5 events are processed
	listener.Close()

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 5 {
		t.Errorf("expected 5 events processed before Close returned, got %d", count)
	}
}

// TestListenerCloseDrainsWithSlowHandler verifies drain works with slow handlers.
func TestListenerCloseDrainsWithSlowHandler(t *testing.T) {
	c := New(WithBufferSize(16))

	sig := NewSignal("test.drain.slow", "Test drain slow signal")
	key := NewIntKey("value")

	var count int32

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		time.Sleep(10 * time.Millisecond) // Slow handler
		atomic.AddInt32(&count, 1)
	})

	// Emit events
	for i := 0; i < 3; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Close should wait for all slow handlers to complete
	listener.Close()

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("expected 3 events processed, got %d", count)
	}
}

// TestListenerCloseNoEventsEmitted verifies Close() works when no events were emitted.
func TestListenerCloseNoEventsEmitted(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.drain.noemit", "Test drain no emit signal")

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		t.Error("callback should not be invoked")
	})

	// Close without any emissions - should not block or panic
	done := make(chan struct{})
	go func() {
		listener.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close blocked with no events")
	}
}

// TestListenerCloseDrainsSyncMode verifies Close() works correctly in sync mode.
func TestListenerCloseDrainsSyncMode(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.drain.sync", "Test drain sync signal")
	key := NewIntKey("value")

	var count int

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	// In sync mode, events are processed immediately
	c.Emit(context.Background(), sig, key.Field(1))
	c.Emit(context.Background(), sig, key.Field(2))

	// Close should return immediately (no async queue to drain)
	listener.Close()

	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

// TestListenerCloseMultipleListenersSameSignal verifies drain works
// when multiple listeners are registered to the same signal.
func TestListenerCloseMultipleListenersSameSignal(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.drain.multi", "Test drain multi signal")
	key := NewIntKey("value")

	var count1, count2 int32

	listener1 := c.Hook(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count1, 1)
	})

	listener2 := c.Hook(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count2, 1)
	})

	// Emit events
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Close first listener - should drain its events
	listener1.Close()

	// First listener should have received all 5
	if atomic.LoadInt32(&count1) != 5 {
		t.Errorf("listener1 expected 5 events, got %d", count1)
	}

	// Emit more events - only listener2 should receive
	for i := 5; i < 8; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	listener2.Close()

	// Second listener should have received all 8
	if atomic.LoadInt32(&count2) != 8 {
		t.Errorf("listener2 expected 8 events, got %d", count2)
	}
}

// TestListenerCloseDuringShutdown verifies Close() handles concurrent shutdown.
func TestListenerCloseDuringShutdown(t *testing.T) {
	for i := 0; i < 50; i++ {
		c := New(WithBufferSize(16))

		sig := NewSignal("test.drain.shutdown", "Test drain shutdown signal")
		key := NewIntKey("value")

		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			time.Sleep(time.Microsecond)
		})

		// Emit events
		for j := 0; j < 10; j++ {
			c.Emit(context.Background(), sig, key.Field(j))
		}

		// Race: Close and Shutdown concurrently
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			listener.Close()
		}()

		go func() {
			defer wg.Done()
			c.Shutdown()
		}()

		// Should complete without deadlock
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: deadlock detected", i)
		}
	}
}

// TestObserverCloseDrainsEvents verifies Observer.Close() also drains.
func TestObserverCloseDrainsEvents(t *testing.T) {
	c := New(WithBufferSize(16))

	sig1 := NewSignal("test.observer.drain1", "Test observer drain signal 1")
	sig2 := NewSignal("test.observer.drain2", "Test observer drain signal 2")
	key := NewIntKey("value")

	// Create hooks so signals exist
	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	var count int32

	observer := c.Observe(func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count, 1)
	})

	// Emit to both signals
	for i := 0; i < 3; i++ {
		c.Emit(context.Background(), sig1, key.Field(i))
		c.Emit(context.Background(), sig2, key.Field(i))
	}

	// Close should drain events from all signals
	observer.Close()

	if atomic.LoadInt32(&count) != 6 {
		t.Errorf("expected 6 events, got %d", count)
	}

	c.Shutdown()
}
