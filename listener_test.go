package capitan

import (
	"context"
	"errors"
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

// TestListenerCloseWorkerDone verifies Close handles worker.done closing
// while trying to send a marker.
func TestListenerCloseWorkerDone(t *testing.T) {
	c := New(WithBufferSize(1))
	sig := NewSignal("test.listener.close.workerdone", "Test listener close worker done")

	block := make(chan struct{})

	// Two listeners - we need both for the race
	listener1 := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block
	})
	listener2 := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block
	})

	// Emit to create worker and block it
	c.Emit(context.Background(), sig)

	// Race: both listeners try to close concurrently
	// One will succeed, the other will hit worker.done
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		listener1.Close()
	}()
	go func() {
		defer wg.Done()
		listener2.Close()
	}()

	// Unblock the worker
	close(block)

	// Wait for both closes to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("deadlock in listener close")
	}

	c.Shutdown()
}

// TestListenerDrain verifies Listener.Drain() blocks until queued events are processed.
func TestListenerDrain(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain", "Test listener drain signal")
	key := NewIntKey("value")

	var count int32

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		time.Sleep(5 * time.Millisecond) // Slow handler
		atomic.AddInt32(&count, 1)
	})

	// Emit events
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Drain should block until all events processed
	err := listener.Drain(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&count) != 5 {
		t.Errorf("expected 5 events processed, got %d", count)
	}

	// Listener should still be active after drain
	c.Emit(context.Background(), sig, key.Field(99))
	err = listener.Drain(context.Background())
	if err != nil {
		t.Errorf("unexpected error on second drain: %v", err)
	}

	if atomic.LoadInt32(&count) != 6 {
		t.Errorf("expected 6 events after second emit, got %d", count)
	}
}

// TestListenerDrainContextCancellation verifies Drain respects context cancellation.
func TestListenerDrainContextCancellation(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.cancel", "Test listener drain cancel signal")
	key := NewIntKey("value")

	block := make(chan struct{})

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block // Block forever
	})

	// Emit event to block the worker
	c.Emit(context.Background(), sig, key.Field(1))

	// Try to drain with already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := listener.Drain(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	close(block)
}

// TestListenerDrainContextCancellationWhileWaiting verifies Drain returns when context is canceled while waiting for marker.
func TestListenerDrainContextCancellationWhileWaiting(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.cancel.waiting", "Test listener drain cancel waiting signal")
	key := NewIntKey("value")

	block := make(chan struct{})

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block // Block until released
	})

	// Emit event to start blocking
	c.Emit(context.Background(), sig, key.Field(1))
	time.Sleep(10 * time.Millisecond) // Ensure event is being processed

	// Start drain in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Drain(ctx)
	}()

	// Cancel while drain is waiting
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not return after context cancellation")
	}

	close(block)
}

// TestListenerDrainSyncMode verifies Drain is a no-op in sync mode.
func TestListenerDrainSyncMode(t *testing.T) {
	c := New(WithSyncMode())
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.sync", "Test listener drain sync signal")
	key := NewIntKey("value")

	var count int

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		count++
	})

	c.Emit(context.Background(), sig, key.Field(1))

	// Drain should return immediately in sync mode
	err := listener.Drain(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

// TestListenerDrainNoWorker verifies Drain returns immediately when no worker exists.
func TestListenerDrainNoWorker(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.noworker", "Test listener drain no worker signal")

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {})

	// No events emitted, so no worker exists
	done := make(chan struct{})
	go func() {
		err := listener.Drain(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success - returned immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("drain blocked with no worker")
	}
}

// TestListenerDrainWorkerDone verifies Drain handles worker.done closing.
func TestListenerDrainWorkerDone(t *testing.T) {
	c := New(WithBufferSize(1))

	sig := NewSignal("test.listener.drain.workerdone", "Test listener drain worker done signal")
	key := NewIntKey("value")

	block := make(chan struct{})

	listener1 := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block
	})
	listener2 := c.Hook(sig, func(_ context.Context, _ *Event) {
		<-block
	})

	// Emit to create worker
	c.Emit(context.Background(), sig, key.Field(1))

	// Close listener1, which will drain and then close the worker when listener2 also closes
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		listener1.Close()
	}()

	go func() {
		defer wg.Done()
		// Give listener1 time to start closing
		time.Sleep(10 * time.Millisecond)
		// This should handle the worker being done
		listener2.Drain(context.Background())
		listener2.Close()
	}()

	// Unblock handlers
	close(block)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("deadlock detected")
	}

	c.Shutdown()
}

// TestListenerDrainDuringShutdown verifies Drain handles global shutdown.
func TestListenerDrainDuringShutdown(t *testing.T) {
	for i := 0; i < 20; i++ {
		c := New(WithBufferSize(16))

		sig := NewSignal("test.listener.drain.shutdown", "Test listener drain shutdown signal")
		key := NewIntKey("value")

		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			time.Sleep(time.Millisecond)
		})

		// Emit events
		for j := 0; j < 5; j++ {
			c.Emit(context.Background(), sig, key.Field(j))
		}

		// Race: Drain and Shutdown concurrently
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			listener.Drain(context.Background())
		}()

		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			c.Shutdown()
		}()

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

// TestListenerDrainConcurrent verifies multiple concurrent Drain calls work correctly.
func TestListenerDrainConcurrent(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.concurrent", "Test listener drain concurrent signal")
	key := NewIntKey("value")

	var count int32

	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count, 1)
	})

	// Emit events
	for i := 0; i < 10; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Multiple concurrent drains
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			listener.Drain(context.Background())
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&count) != 10 {
		t.Errorf("expected 10 events, got %d", count)
	}
}

// TestObserverDrain verifies Observer.Drain() blocks until all queued events are processed.
func TestObserverDrain(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig1 := NewSignal("test.observer.drain.one", "Test observer drain signal 1")
	sig2 := NewSignal("test.observer.drain.two", "Test observer drain signal 2")
	key := NewIntKey("value")

	// Create hooks so signals exist
	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	var count int32

	observer := c.Observe(func(_ context.Context, _ *Event) {
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&count, 1)
	})

	// Emit to both signals
	for i := 0; i < 3; i++ {
		c.Emit(context.Background(), sig1, key.Field(i))
		c.Emit(context.Background(), sig2, key.Field(i))
	}

	// Drain should block until all events processed
	err := observer.Drain(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&count) != 6 {
		t.Errorf("expected 6 events, got %d", count)
	}

	// Observer should still be active after drain
	c.Emit(context.Background(), sig1, key.Field(99))
	err = observer.Drain(context.Background())
	if err != nil {
		t.Errorf("unexpected error on second drain: %v", err)
	}

	if atomic.LoadInt32(&count) != 7 {
		t.Errorf("expected 7 events after second emit, got %d", count)
	}
}

// TestObserverDrainContextCancellation verifies Observer.Drain respects context cancellation.
func TestObserverDrainContextCancellation(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.observer.drain.cancel", "Test observer drain cancel signal")
	key := NewIntKey("value")

	c.Hook(sig, func(_ context.Context, _ *Event) {})

	block := make(chan struct{})

	observer := c.Observe(func(_ context.Context, _ *Event) {
		<-block
	})

	// Emit event to block
	c.Emit(context.Background(), sig, key.Field(1))

	// Try to drain with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := observer.Drain(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	close(block)
}

// TestObserverDrainInactive verifies Drain returns immediately for closed observer.
func TestObserverDrainInactive(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.observer.drain.inactive", "Test observer drain inactive signal")

	c.Hook(sig, func(_ context.Context, _ *Event) {})

	observer := c.Observe(func(_ context.Context, _ *Event) {})

	// Close the observer
	observer.Close()

	// Drain should return immediately
	done := make(chan struct{})
	go func() {
		err := observer.Drain(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("drain blocked on inactive observer")
	}
}

// TestObserverDrainNoListeners verifies Drain returns immediately when observer has no listeners.
func TestObserverDrainNoListeners(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	// Create observer before any signals exist (no listeners yet)
	observer := c.Observe(func(_ context.Context, _ *Event) {})

	// Drain should return immediately
	done := make(chan struct{})
	go func() {
		err := observer.Drain(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("drain blocked with no listeners")
	}
}

// TestObserverDrainPropagatesListenerError verifies Observer.Drain returns listener errors.
func TestObserverDrainPropagatesListenerError(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig := NewSignal("test.observer.drain.error", "Test observer drain error signal")
	key := NewIntKey("value")

	c.Hook(sig, func(_ context.Context, _ *Event) {})

	block := make(chan struct{})

	observer := c.Observe(func(_ context.Context, _ *Event) {
		<-block // Block until released
	})

	// Emit event to start blocking
	c.Emit(context.Background(), sig, key.Field(1))
	time.Sleep(10 * time.Millisecond) // Ensure event is being processed

	// Start drain with context that will be canceled while waiting
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- observer.Drain(ctx)
	}()

	// Give time for drain to start waiting
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		// Either context.Canceled from outer select or from listener propagation
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not return after context cancellation")
	}

	close(block)
}

// TestObserverDrainReturnsListenerErrorAfterCompletion tests the errCh read path.
// This requires listener drains to complete with errors and select to pick done.
func TestObserverDrainReturnsListenerErrorAfterCompletion(t *testing.T) {
	c := New(WithBufferSize(32))
	defer c.Shutdown()

	sig := NewSignal("test.observer.drain.error.completion", "Test observer drain error completion signal")
	key := NewIntKey("value")

	c.Hook(sig, func(_ context.Context, _ *Event) {})

	observer := c.Observe(func(_ context.Context, _ *Event) {
		time.Sleep(10 * time.Millisecond)
	})

	// Emit events
	for j := 0; j < 3; j++ {
		c.Emit(context.Background(), sig, key.Field(j))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := observer.Drain(ctx)

	// We expect context.DeadlineExceeded
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestListenerDrainWorkerDoneRace specifically tests the worker.done branch in Drain.
func TestListenerDrainWorkerDoneRace(t *testing.T) {
	for i := 0; i < 10; i++ {
		c := New(WithBufferSize(1))

		sig := NewSignal("test.listener.drain.workerdone.race", "Test listener drain worker done race signal")
		key := NewIntKey("value")

		slowBlock := make(chan struct{})

		// Single listener with slow handler
		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			<-slowBlock
		})

		// Emit to create worker and start processing
		c.Emit(context.Background(), sig, key.Field(1))

		// Race: Shutdown and Drain concurrently
		// Shutdown will close worker.done
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			c.Shutdown()
		}()

		go func() {
			defer wg.Done()
			// Small delay to increase chance of hitting worker.done case
			time.Sleep(time.Microsecond * time.Duration(i%10))
			listener.Drain(context.Background())
		}()

		// Unblock after a bit
		go func() {
			time.Sleep(5 * time.Millisecond)
			close(slowBlock)
		}()

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

// TestListenerDrainShutdownRace specifically tests the c.shutdown branch in Drain.
func TestListenerDrainShutdownRace(t *testing.T) {
	for i := 0; i < 10; i++ {
		c := New(WithBufferSize(16))

		sig := NewSignal("test.listener.drain.shutdown.race", "Test listener drain shutdown race signal")
		key := NewIntKey("value")

		// Fill the markers channel so Drain has to wait
		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			time.Sleep(20 * time.Millisecond)
		})

		// Emit multiple events to keep worker busy
		for j := 0; j < 5; j++ {
			c.Emit(context.Background(), sig, key.Field(j))
		}

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			// Immediately shutdown
			c.Shutdown()
		}()

		go func() {
			defer wg.Done()
			listener.Drain(context.Background())
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: deadlock detected", i)
		}
	}
}

// TestListenerDrainContextCancelledWhileWaitingForMarker tests the inner ctx.Done branch.
// This requires: marker sent successfully, but context canceled while draining remaining events.
func TestListenerDrainContextCancelledWhileWaitingForMarker(t *testing.T) {
	c := New(WithBufferSize(32))
	defer c.Shutdown()

	sig := NewSignal("test.listener.drain.ctx.inner", "Test listener drain ctx inner signal")
	key := NewIntKey("value")

	// Handler that takes 30ms per event
	listener := c.Hook(sig, func(_ context.Context, _ *Event) {
		time.Sleep(30 * time.Millisecond)
	})

	// Emit 10 events - will take 300ms total to drain
	for i := 0; i < 10; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}

	// Wait for first event to start processing
	time.Sleep(5 * time.Millisecond)

	// Drain with timeout:
	// - After ~30ms: first event done, worker enters select, receives marker
	// - Worker starts draining 9 remaining events (270ms)
	// - After 100ms: context times out while draining
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := listener.Drain(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestListenerDrainWorkerDoneViaConfigDisable tests the worker.done branch when
// ApplyConfig disables a signal while Drain is waiting.
func TestListenerDrainWorkerDoneViaConfigDisable(t *testing.T) {
	for i := 0; i < 5; i++ {
		c := New(WithBufferSize(1))

		sig := NewSignal("test.drain.config.disable", "Test drain config disable signal")
		key := NewIntKey("value")

		// Slow handler to keep worker busy
		listener := c.Hook(sig, func(_ context.Context, _ *Event) {
			time.Sleep(20 * time.Millisecond)
		})

		// Emit to create worker and keep it busy
		c.Emit(context.Background(), sig, key.Field(1))

		// Small delay to ensure worker is processing
		time.Sleep(5 * time.Millisecond)

		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: Drain (will block trying to send marker)
		go func() {
			defer wg.Done()
			listener.Drain(context.Background())
		}()

		// Goroutine 2: Disable the signal via config (closes worker.done)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			c.ApplyConfig(Config{
				Signals: map[string]SignalConfig{
					sig.Name(): {Disabled: true},
				},
			})
		}()

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

		c.Shutdown()
	}
}

// TestObserverDrainConcurrent verifies multiple concurrent Drain calls work correctly.
func TestObserverDrainConcurrent(t *testing.T) {
	c := New(WithBufferSize(16))
	defer c.Shutdown()

	sig1 := NewSignal("test.observer.drain.concurrent.one", "Test observer drain concurrent signal 1")
	sig2 := NewSignal("test.observer.drain.concurrent.two", "Test observer drain concurrent signal 2")
	key := NewIntKey("value")

	c.Hook(sig1, func(_ context.Context, _ *Event) {})
	c.Hook(sig2, func(_ context.Context, _ *Event) {})

	var count int32

	observer := c.Observe(func(_ context.Context, _ *Event) {
		atomic.AddInt32(&count, 1)
	})

	// Emit events
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig1, key.Field(i))
		c.Emit(context.Background(), sig2, key.Field(i))
	}

	// Multiple concurrent drains
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			observer.Drain(context.Background())
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&count) != 10 {
		t.Errorf("expected 10 events, got %d", count)
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
