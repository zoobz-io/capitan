package testing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestEventCapture(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.capture", "Test capture signal")
	key := capitan.NewStringKey("msg")

	capture := NewEventCapture()
	c.Hook(sig, capture.Handler())

	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))

	if capture.Count() != 2 {
		t.Errorf("expected 2 events, got %d", capture.Count())
	}

	events := capture.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Signal != sig {
		t.Errorf("expected signal %v, got %v", sig, events[0].Signal)
	}
}

func TestEventCaptureReset(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.capture.reset", "Test capture reset signal")
	key := capitan.NewStringKey("msg")

	capture := NewEventCapture()
	c.Hook(sig, capture.Handler())

	c.Emit(context.Background(), sig, key.Field("test"))

	if capture.Count() != 1 {
		t.Errorf("expected 1 event, got %d", capture.Count())
	}

	capture.Reset()

	if capture.Count() != 0 {
		t.Errorf("expected 0 events after reset, got %d", capture.Count())
	}
}

func TestEventCaptureWaitForCount(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.capture.wait", "Test capture wait signal")
	key := capitan.NewStringKey("msg")

	capture := NewEventCapture()
	c.Hook(sig, capture.Handler())

	go func() {
		time.Sleep(10 * time.Millisecond)
		c.Emit(context.Background(), sig, key.Field("test"))
	}()

	if !capture.WaitForCount(1, 500*time.Millisecond) {
		t.Error("WaitForCount timed out")
	}

	// Test timeout
	if capture.WaitForCount(100, 10*time.Millisecond) {
		t.Error("WaitForCount should have timed out")
	}
}

func TestEventCaptureConcurrent(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.capture.concurrent", "Test concurrent capture")
	key := capitan.NewStringKey("msg")

	capture := NewEventCapture()
	c.Hook(sig, capture.Handler())

	const numEmissions = 50
	var wg sync.WaitGroup
	wg.Add(numEmissions)

	for i := 0; i < numEmissions; i++ {
		go func() {
			defer wg.Done()
			c.Emit(context.Background(), sig, key.Field("test"))
		}()
	}

	wg.Wait()

	if !capture.WaitForCount(numEmissions, time.Second) {
		t.Errorf("expected %d events, got %d", numEmissions, capture.Count())
	}
}

func TestEventCounter(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.counter", "Test counter signal")
	key := capitan.NewStringKey("msg")

	counter := NewEventCounter()
	c.Hook(sig, counter.Handler())

	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))
	c.Emit(context.Background(), sig, key.Field("third"))

	if counter.Count() != 3 {
		t.Errorf("expected 3, got %d", counter.Count())
	}
}

func TestEventCounterReset(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.counter.reset", "Test counter reset signal")
	key := capitan.NewStringKey("msg")

	counter := NewEventCounter()
	c.Hook(sig, counter.Handler())

	c.Emit(context.Background(), sig, key.Field("test"))

	if counter.Count() != 1 {
		t.Errorf("expected 1, got %d", counter.Count())
	}

	counter.Reset()

	if counter.Count() != 0 {
		t.Errorf("expected 0 after reset, got %d", counter.Count())
	}
}

func TestEventCounterWaitForCount(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.counter.wait", "Test counter wait signal")
	key := capitan.NewStringKey("msg")

	counter := NewEventCounter()
	c.Hook(sig, counter.Handler())

	go func() {
		time.Sleep(10 * time.Millisecond)
		c.Emit(context.Background(), sig, key.Field("test"))
	}()

	if !counter.WaitForCount(1, 500*time.Millisecond) {
		t.Error("WaitForCount timed out")
	}

	// Test timeout
	if counter.WaitForCount(100, 10*time.Millisecond) {
		t.Error("WaitForCount should have timed out")
	}
}

func TestPanicRecorder(t *testing.T) {
	sig := capitan.NewSignal("test.panic", "Panic signal")
	key := capitan.NewStringKey("msg")

	recorder := NewPanicRecorder()

	c := capitan.New(
		capitan.WithSyncMode(),
		capitan.WithPanicHandler(recorder.Handler()),
	)
	defer c.Shutdown()

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		panic("intentional panic")
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if recorder.Count() != 1 {
		t.Errorf("expected 1 panic recorded, got %d", recorder.Count())
	}

	panics := recorder.Panics()
	if len(panics) != 1 {
		t.Fatalf("expected 1 panic, got %d", len(panics))
	}

	if panics[0].Signal != sig {
		t.Errorf("expected signal %v, got %v", sig, panics[0].Signal)
	}

	if panics[0].Recovered != "intentional panic" {
		t.Errorf("expected 'intentional panic', got %v", panics[0].Recovered)
	}
}

func TestPanicRecorderReset(t *testing.T) {
	recorder := NewPanicRecorder()

	// Manually add a panic record via handler
	handler := recorder.Handler()
	handler(capitan.NewSignal("test", "test"), "panic value")

	if recorder.Count() != 1 {
		t.Errorf("expected 1 panic, got %d", recorder.Count())
	}

	recorder.Reset()

	if recorder.Count() != 0 {
		t.Errorf("expected 0 panics after reset, got %d", recorder.Count())
	}
}

func TestFieldExtractor(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.extract", "Extract signal")
	strKey := capitan.NewStringKey("str")
	intKey := capitan.NewIntKey("int")
	boolKey := capitan.NewBoolKey("bool")
	floatKey := capitan.NewFloat64Key("float")

	extractor := NewFieldExtractor()

	var receivedStr string
	var receivedInt int
	var receivedBool bool
	var receivedFloat float64

	c.Hook(sig, func(_ context.Context, e *capitan.Event) {
		receivedStr = extractor.GetString(e, strKey)
		receivedInt = extractor.GetInt(e, intKey)
		receivedBool = extractor.GetBool(e, boolKey)
		receivedFloat = extractor.GetFloat64(e, floatKey)
	})

	c.Emit(context.Background(), sig,
		strKey.Field("hello"),
		intKey.Field(42),
		boolKey.Field(true),
		floatKey.Field(3.14),
	)

	if receivedStr != "hello" {
		t.Errorf("expected 'hello', got %q", receivedStr)
	}
	if receivedInt != 42 {
		t.Errorf("expected 42, got %d", receivedInt)
	}
	if receivedBool != true {
		t.Errorf("expected true, got %v", receivedBool)
	}
	if receivedFloat != 3.14 {
		t.Errorf("expected 3.14, got %f", receivedFloat)
	}
}

func TestFieldExtractorMissing(t *testing.T) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.extract.missing", "Extract missing signal")
	strKey := capitan.NewStringKey("str")
	intKey := capitan.NewIntKey("int")
	boolKey := capitan.NewBoolKey("bool")
	floatKey := capitan.NewFloat64Key("float")
	otherKey := capitan.NewStringKey("other")

	extractor := NewFieldExtractor()

	var receivedStr string
	var receivedInt int
	var receivedBool bool
	var receivedFloat float64

	c.Hook(sig, func(_ context.Context, e *capitan.Event) {
		receivedStr = extractor.GetString(e, strKey)
		receivedInt = extractor.GetInt(e, intKey)
		receivedBool = extractor.GetBool(e, boolKey)
		receivedFloat = extractor.GetFloat64(e, floatKey)
	})

	// Emit without the keys we're looking for
	c.Emit(context.Background(), sig, otherKey.Field("other"))

	if receivedStr != "" {
		t.Errorf("expected empty string for missing key, got %q", receivedStr)
	}
	if receivedInt != 0 {
		t.Errorf("expected 0 for missing key, got %d", receivedInt)
	}
	if receivedBool != false {
		t.Errorf("expected false for missing key, got %v", receivedBool)
	}
	if receivedFloat != 0.0 {
		t.Errorf("expected 0.0 for missing key, got %f", receivedFloat)
	}
}

func TestTestCapitan(t *testing.T) {
	c := TestCapitan(capitan.WithSyncMode())
	if c == nil {
		t.Fatal("TestCapitan returned nil")
	}
	defer c.Shutdown()

	sig := capitan.NewSignal("test.testcapitan", "Test capitan signal")
	key := capitan.NewStringKey("msg")

	var received bool
	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		received = true
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	if !received {
		t.Error("event not received")
	}
}

func TestStatsWaiter(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.stats", "Stats signal")
	key := capitan.NewStringKey("msg")

	waiter := NewStatsWaiter(c)

	// Initial state - no workers
	if !waiter.WaitForWorkers(0, 100*time.Millisecond) {
		t.Error("expected 0 workers initially")
	}

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		time.Sleep(50 * time.Millisecond)
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	// Worker should be created
	if !waiter.WaitForWorkers(1, 500*time.Millisecond) {
		t.Error("expected 1 worker after emit")
	}
}

func TestStatsWaiterEmptyQueues(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.stats.queue", "Stats queue signal")
	key := capitan.NewStringKey("msg")

	waiter := NewStatsWaiter(c)

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		time.Sleep(10 * time.Millisecond)
	})

	c.Emit(context.Background(), sig, key.Field("test"))

	// Wait for queues to drain
	if !waiter.WaitForEmptyQueues(500 * time.Millisecond) {
		t.Error("WaitForEmptyQueues timed out")
	}
}

func TestStatsWaiterEmitCount(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.stats.emit", "Stats emit signal")
	key := capitan.NewStringKey("msg")

	waiter := NewStatsWaiter(c)

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

	c.Emit(context.Background(), sig, key.Field("first"))
	c.Emit(context.Background(), sig, key.Field("second"))
	c.Emit(context.Background(), sig, key.Field("third"))

	if !waiter.WaitForEmitCount(sig, 3, 500*time.Millisecond) {
		t.Error("WaitForEmitCount timed out")
	}

	// Test timeout for unreachable count
	if waiter.WaitForEmitCount(sig, 100, 10*time.Millisecond) {
		t.Error("WaitForEmitCount should have timed out")
	}
}

func TestStatsWaiterWorkersTimeout(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	waiter := NewStatsWaiter(c)

	// Request impossible number of workers - should timeout
	if waiter.WaitForWorkers(100, 10*time.Millisecond) {
		t.Error("WaitForWorkers should have timed out")
	}
}

func TestStatsWaiterEmptyQueuesTimeout(t *testing.T) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.stats.queue.timeout", "Queue timeout signal")
	key := capitan.NewStringKey("msg")

	waiter := NewStatsWaiter(c)

	// Hook that blocks for a while to keep queue non-empty
	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		time.Sleep(200 * time.Millisecond)
	})

	// Emit multiple events to fill the queue
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig, key.Field("test"))
	}

	// Wait a tiny amount - queue should still have items
	if waiter.WaitForEmptyQueues(5 * time.Millisecond) {
		t.Error("WaitForEmptyQueues should have timed out with non-empty queue")
	}
}
