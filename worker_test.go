package capitan

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWorkerCreatedLazily(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.lazy", "Test worker lazy signal")
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

func TestWorkerProcessesEventsAsync(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.async", "Test worker async signal")
	key := NewIntKey("value")

	const numEvents = 100
	received := make([]int, 0, numEvents)
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(numEvents)

	c.Hook(sig, func(_ context.Context, e *Event) {
		field := e.Get(key).(GenericField[int])
		mu.Lock()
		received = append(received, field.Get())
		mu.Unlock()
		wg.Done()
	})

	// Emit many events rapidly - should not block caller
	start := time.Now()
	for i := 0; i < numEvents; i++ {
		c.Emit(context.Background(), sig, key.Field(i))
	}
	emitDuration := time.Since(start)

	// Emissions should be very fast (not blocking on listener execution)
	if emitDuration > 100*time.Millisecond {
		t.Errorf("emissions took too long: %v", emitDuration)
	}

	wg.Wait()

	if len(received) != numEvents {
		t.Errorf("expected %d events, got %d", numEvents, len(received))
	}
}

func TestWorkerInvokesAllListeners(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.all", "Test worker all signal")
	key := NewStringKey("value")

	count1, count2, count3 := 0, 0, 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		count1++
		mu.Unlock()
		wg.Done()
	})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		count2++
		mu.Unlock()
		wg.Done()
	})

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		count3++
		mu.Unlock()
		wg.Done()
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if count1 != 1 || count2 != 1 || count3 != 1 {
		t.Errorf("expected all listeners invoked once, got %d, %d, %d", count1, count2, count3)
	}
}

func TestWorkerPanicRecovery(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.panic", "Test worker panic signal")
	key := NewStringKey("value")

	var wg sync.WaitGroup
	wg.Add(1)

	// First listener panics
	c.Hook(sig, func(_ context.Context, _ *Event) {
		panic("intentional panic")
	})

	// Second listener should still execute
	c.Hook(sig, func(_ context.Context, _ *Event) {
		wg.Done()
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - second listener executed despite first panic
	case <-time.After(time.Second):
		t.Fatal("second listener never executed - panic not recovered")
	}
}

func TestWorkerShutdownDrainsQueue(t *testing.T) {
	c := New()

	sig := NewSignal("test.worker.drain", "Test worker drain signal")
	key := NewStringKey("value")

	processed := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(5)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		processed++
		mu.Unlock()
		wg.Done()
	})

	// Emit multiple events
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig, key.Field("test"))
	}

	// Shutdown should wait for all events to be processed
	c.Shutdown()

	mu.Lock()
	finalCount := processed
	mu.Unlock()

	if finalCount != 5 {
		t.Errorf("expected 5 events processed, got %d", finalCount)
	}
}

func TestWorkerNoListeners(_ *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.none", "Test worker none signal")
	key := NewStringKey("value")

	// Should not panic or hang
	c.Emit(context.Background(), sig, key.Field("test"))

	time.Sleep(10 * time.Millisecond)
}

func TestWorkerOnlyCreatedWithListeners(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.creation", "Test worker creation signal")
	key := NewStringKey("value")

	// Emit without any listeners - worker should NOT be created
	c.Emit(context.Background(), sig, key.Field("test"))

	c.mu.RLock()
	_, workerExists := c.workers[sig]
	c.mu.RUnlock()

	if workerExists {
		t.Error("worker should not be created when emitting with no listeners")
	}

	// Now add a listener
	c.Hook(sig, func(_ context.Context, _ *Event) {})

	// Emit with listener - worker SHOULD be created
	c.Emit(context.Background(), sig, key.Field("test"))

	c.mu.RLock()
	_, workerExists = c.workers[sig]
	c.mu.RUnlock()

	if !workerExists {
		t.Error("worker should be created when emitting with listeners present")
	}
}

func TestWorkerNotCreatedOnHookOnly(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.hook.only", "Test hook only signal")

	// Just hook a listener without emitting
	c.Hook(sig, func(_ context.Context, _ *Event) {})

	c.mu.RLock()
	_, workerExists := c.workers[sig]
	c.mu.RUnlock()

	if workerExists {
		t.Error("worker should not be created on Hook, only on Emit")
	}
}

func TestWorkerCreatedWithObserver(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.observer.worker", "Test observer worker signal")
	key := NewStringKey("value")

	// Create an observer
	c.Observe(func(_ context.Context, _ *Event) {})

	// Emit - observer should cause listener to be attached, and worker created
	c.Emit(context.Background(), sig, key.Field("test"))

	c.mu.RLock()
	_, workerExists := c.workers[sig]
	c.mu.RUnlock()

	if !workerExists {
		t.Error("worker should be created when emitting with observer present")
	}
}

func TestWorkerMultipleSignalsIsolated(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig1 := NewSignal("test.worker.iso1", "Test worker isolation signal 1")
	sig2 := NewSignal("test.worker.iso2", "Test worker isolation signal 2")
	key := NewStringKey("value")

	count1, count2 := 0, 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	c.Hook(sig1, func(_ context.Context, _ *Event) {
		mu.Lock()
		count1++
		mu.Unlock()
		wg.Done()
	})

	c.Hook(sig2, func(_ context.Context, _ *Event) {
		mu.Lock()
		count2++
		mu.Unlock()
		wg.Done()
	})

	c.Emit(context.Background(), sig1, key.Field("first"))
	c.Emit(context.Background(), sig1, key.Field("second"))
	c.Emit(context.Background(), sig2, key.Field("third"))

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if count1 != 2 {
		t.Errorf("sig1: expected 2 events, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("sig2: expected 1 event, got %d", count2)
	}
}

func TestWorkerContextCancellation(t *testing.T) {
	c := New()
	defer c.Shutdown()

	sig := NewSignal("test.worker.cancel", "Test worker cancel signal")
	key := NewStringKey("value")

	var received int
	var mu sync.Mutex

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		received++
		mu.Unlock()
	})

	// Emit with already-canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.Emit(ctx, sig, key.Field("test"))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	count := received
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 events with canceled context, got %d", count)
	}
}

func TestWorkerContextTimeout(t *testing.T) {
	c := New(WithBufferSize(1))
	defer c.Shutdown()

	sig := NewSignal("test.worker.timeout", "Test worker timeout signal")
	key := NewIntKey("value")

	block := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		wg.Done()
		<-block
	})

	// Fill buffer
	c.Emit(context.Background(), sig, key.Field(1))
	wg.Wait()

	c.Emit(context.Background(), sig, key.Field(2))

	// This should timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	c.Emit(ctx, sig, key.Field(3))
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("Emit blocked too long: %v", elapsed)
	}

	close(block)
}

func TestWorkerSkipsCancelledEvents(t *testing.T) {
	c := New(WithBufferSize(10))
	defer c.Shutdown()

	sig := NewSignal("test.worker.skip", "Test worker skip signal")
	key := NewStringKey("value")

	var received int
	var mu sync.Mutex
	firstEvent := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		mu.Lock()
		received++
		mu.Unlock()
		wg.Done()
		<-firstEvent
	})

	// Emit first event with valid context
	c.Emit(context.Background(), sig, key.Field("first"))
	wg.Wait()

	// Emit second event with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	c.Emit(ctx, sig, key.Field("second"))

	// Cancel context while event is queued
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Release first event (second should be skipped due to canceled context)
	close(firstEvent)

	// Give time for processing
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	count := received
	mu.Unlock()

	// Should only have processed first event
	if count != 1 {
		t.Errorf("expected 1 event processed, got %d", count)
	}
}

func TestEmitBlockedOnShutdown(t *testing.T) {
	// Test that Emit unblocks when Shutdown is called while waiting on full buffer
	c := New(WithBufferSize(1))

	sig := NewSignal("test.shutdown.blocked", "Test shutdown blocked signal")
	key := NewStringKey("value")

	block := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	c.Hook(sig, func(_ context.Context, _ *Event) {
		wg.Done()
		<-block // Block listener to keep worker busy
	})

	// Emit first event - will be received by listener and block
	c.Emit(context.Background(), sig, key.Field("first"))
	wg.Wait() // Wait for listener to receive first event

	// Emit second event - fills the buffer
	c.Emit(context.Background(), sig, key.Field("second"))

	// Start goroutine to emit third event - will block on full buffer
	emitDone := make(chan struct{})
	go func() {
		c.Emit(context.Background(), sig, key.Field("third"))
		close(emitDone)
	}()

	// Give goroutine time to block on the send
	time.Sleep(50 * time.Millisecond)

	// Call Shutdown while Emit is blocked - should unblock via c.shutdown case
	shutdownDone := make(chan struct{})
	go func() {
		c.Shutdown()
		close(shutdownDone)
	}()

	// Emit should return quickly after shutdown fires
	select {
	case <-emitDone:
		// Success - Emit unblocked
	case <-time.After(time.Second):
		t.Fatal("Emit did not unblock on Shutdown")
	}

	// Unblock listener so shutdown can complete
	close(block)

	// Shutdown should complete
	select {
	case <-shutdownDone:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not complete")
	}
}

// TestEmitDuringShutdownNoWorker tests that emitting to a signal without
// a worker during shutdown is handled gracefully (worker creation is skipped).
func TestEmitDuringShutdownNoWorker(t *testing.T) {
	c := New()

	sig := NewSignal("test.shutdown.noworker", "Test shutdown no worker signal")
	key := NewStringKey("value")

	var received bool
	c.Hook(sig, func(_ context.Context, _ *Event) {
		received = true
	})

	// Call shutdown immediately (no workers created yet)
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Start shutdown
	go func() {
		defer wg.Done()
		c.Shutdown()
	}()

	// Goroutine 2: Try to emit (will try to create worker but should abort due to shutdown)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Give shutdown a chance to fire
		c.Emit(context.Background(), sig, key.Field("test"))
	}()

	wg.Wait()

	// Event should not be received since worker creation was aborted
	if received {
		t.Error("expected event to be dropped during shutdown, but it was received")
	}
}
