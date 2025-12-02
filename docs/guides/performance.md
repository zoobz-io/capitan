---
title: Performance Guide
description: Optimization techniques and performance characteristics of capitan.
author: Capitan Team
published: 2025-12-01
tags: [Performance, Optimization, Guide]
---

# Performance Guide

Optimization techniques and performance characteristics.

## Performance Characteristics

### Emit() Latency

**Async Mode** (default):
```
Event pool retrieval:    ~10ns
Observer invocation:     ~100ns per observer
Queue send:              ~50ns
────────────────────────────────
Total:                   ~1-2µs (1000-2000 nanoseconds)
```

**Sync Mode** (testing):
```
Event pool retrieval:    ~10ns
Observer invocation:     ~100ns per observer
Direct listener calls:   ~50ns per listener
────────────────────────────────
Total:                   ~500ns-1µs
```

### Memory Usage

**Per Signal**:
- Worker goroutine: ~2KB stack
- Queue buffer: `bufferSize * 200 bytes`
- Listener slice: `listeners * 8 bytes`

**Per Observer**:
- Entry struct: ~40 bytes
- Whitelist map: `signals * 16 bytes` (if used)

**Event Pool**:
- Pool grows to max concurrent events
- Each event: ~200 bytes
- Reclaimed during GC pauses

## Benchmark Results

From `testing/benchmarks/core_performance_test.go`:

```
BenchmarkEmit_Async-8              850000    1200 ns/op     152 B/op    2 allocs/op
BenchmarkEmit_Sync-8              3000000     380 ns/op      88 B/op    1 alloc/op
BenchmarkEmit_MultipleFields-8     800000    1450 ns/op     649 B/op    2 allocs/op
BenchmarkHook-8                   5000000     240 ns/op      64 B/op    1 alloc/op
BenchmarkObserve-8                 650000    1870 ns/op     128 B/op    3 allocs/op
BenchmarkFieldExtraction-8        1500000     688 ns/op       0 B/op    0 allocs/op
```

Key observations:
- Event pooling keeps allocations low (1-2 allocs/op)
- Field extraction is zero-allocation
- Observer overhead is minimal (~1-2µs)

## Optimization Techniques

### 1. Minimize Observer Count

Observers add O(n) overhead to every `Emit()`:

```go
// ❌ Bad: 10 observers = 10x overhead
for i := 0; i < 10; i++ {
    c.Observe(handler)
}

// ✅ Good: 1 observer, dispatch internally
type ObserverHub struct {
    handlers []capitan.EventCallback
}

func (oh *ObserverHub) Start(c *capitan.Capitan) {
    c.Observe(oh.dispatch)
}

func (oh *ObserverHub) dispatch(ctx context.Context, e *capitan.Event) {
    for _, handler := range oh.handlers {
        handler(ctx, e)
    }
}
```

**Impact**: 10x reduction in observer overhead per event.

### 2. Use Observer Whitelists

Limit observer invocations with whitelists:

```go
// ❌ Bad: Called for ALL events
observer := c.Observe(func(ctx context.Context, e *capitan.Event) {
    if e.Signal() == signal1 || e.Signal() == signal2 {
        // Handle
    }
    // Still called for every event!
})

// ✅ Good: Only called for whitelist
observer := c.Observe(handler, signal1, signal2)
```

**Impact**: O(signals) → O(whitelist) invocations.

### 3. Optimize Buffer Sizes

Match buffer sizes to workload:

```go
// Low-throughput: Small buffers (10-50)
c := capitan.New(capitan.WithBufferSize(10))

// Medium-throughput: Balanced (100-500)
c := capitan.New(capitan.WithBufferSize(100))

// High-throughput: Large buffers (1000+)
c := capitan.New(capitan.WithBufferSize(1000))
```

**Tradeoff**: Memory vs backpressure latency.

### 4. Reduce Field Count

Fewer fields = less work per event:

```go
// ❌ Many fields
c.Emit(ctx, signal,
    field1.Field(v1),
    field2.Field(v2),
    field3.Field(v3),
    field4.Field(v4),
    field5.Field(v5),
    field6.Field(v6),
    field7.Field(v7),
    field8.Field(v8),
) // ~649B/op with 8 fields

// ✅ Essential fields only
c.Emit(ctx, signal,
    field1.Field(v1),
    field2.Field(v2),
) // ~88B/op with 2 fields
```

**Impact**: 7x reduction in allocation size.

### 5. Avoid Heavy Work in Observers

Observers block `Emit()` - offload heavy work:

```go
// ❌ Bad: Slow observer
c.Observe(func(ctx context.Context, e *capitan.Event) {
    db.Save(e) // Blocks every Emit()!
})

// ✅ Good: Queue for async processing
queue := make(chan CapturedEvent, 100)

c.Observe(func(ctx context.Context, e *capitan.Event) {
    select {
    case queue <- CapturedEvent{
        Signal:   e.Signal(),
        Fields:   e.Fields(),
    }:
    default:
        // Drop if queue full
    }
})

go func() {
    for e := range queue {
        db.Save(e)
    }
}()
```

**Impact**: Emit() latency: 2ms → 2µs (1000x faster).

### 6. Reuse Field Keys

Define keys once, reuse everywhere:

```go
// ✅ Good: Reuse keys
var (
    UserIDKey = capitan.NewStringKey("user_id")
    OrderIDKey = capitan.NewStringKey("order_id")
)

c.Emit(ctx, signal1, UserIDKey.Field("u1"))
c.Emit(ctx, signal2, UserIDKey.Field("u2"))

// ❌ Bad: Recreate keys
c.Emit(ctx, signal1, capitan.NewStringKey("user_id").Field("u1"))
c.Emit(ctx, signal2, capitan.NewStringKey("user_id").Field("u2"))
```

**Impact**: Slight reduction in allocations.

### 7. Batch Events When Possible

Emit in batches to amortize overhead:

```go
// ❌ Inefficient: 1000 separate Emit() calls
for _, item := range items {
    c.Emit(ctx, signal, idKey.Field(item.ID))
}

// ✅ Better: Batch emit
batchKey := capitan.NewAnyKey("batch")
c.Emit(ctx, batchSignal, batchKey.Field(items))

// Handler processes batch
c.Hook(batchSignal, func(ctx context.Context, e *capitan.Event) {
    items := batchKey.Extract(e).([]Item)
    for _, item := range items {
        processItem(item)
    }
})
```

**Impact**: 1000 Emit() calls → 1 Emit() call.

## Profiling

### CPU Profiling

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()

    // ... run capitan workload
}
```

Visit `http://localhost:6060/debug/pprof/profile?seconds=30` for CPU profile.

### Memory Profiling

```bash
go test -bench=. -memprofile=mem.prof ./testing/benchmarks/...
go tool pprof mem.prof
```

### Trace Analysis

```bash
go test -bench=. -trace=trace.out ./testing/benchmarks/...
go tool trace trace.out
```

## Monitoring

### Track Queue Depths

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stats := c.Stats()

        for signal, count := range stats.PendingEvents {
            if count > 100 {
                log.Printf("Warning: Signal %s has %d pending events", signal, count)
            }

            metrics.Gauge("capitan.queue.depth", float64(count),
                "signal", signal,
            )
        }
    }
}()
```

### Track Event Throughput

```go
var eventCount atomic.Int64

c.Observe(func(ctx context.Context, e *capitan.Event) {
    eventCount.Add(1)
})

go func() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    lastCount := int64(0)

    for range ticker.C {
        currentCount := eventCount.Load()
        throughput := currentCount - lastCount
        lastCount = currentCount

        metrics.Gauge("capitan.events.per_second", float64(throughput))
    }
}()
```

### Track Handler Latency

```go
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        metrics.RecordDuration("capitan.handler.duration", duration,
            "signal", e.Signal().Name(),
        )
    }()

    processEvent(e)
})
```

## Scalability

### Horizontal Scaling

Capitan is in-process only. For distributed scaling:

```go
// Option 1: Shard by signal
instance1.Hook(orderSignals...)
instance2.Hook(userSignals...)
instance3.Hook(paymentSignals...)

// Option 2: Use message queue for cross-process
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Forward to message queue
    queue.Publish(e.Signal().Name(), e.Fields())
})
```

### Vertical Scaling

Per-signal workers scale with goroutines:

```go
// 100 signals = 100 workers (goroutines)
// Each goroutine: ~2KB

// 100 * 2KB = 200KB total
```

Goroutines are cheap - thousands are fine.

## Performance Comparisons

### vs. Direct Function Calls

```go
// Direct call: ~5ns
processOrder(order)

// Capitan async: ~1-2µs
c.Emit(ctx, orderPlaced, fields...)

// Overhead: ~400x slower
// Trade-off: Decoupling, observability, async processing
```

**When to use direct calls**: Tight loops, hot paths, critical latency.

**When to use capitan**: Coordination, decoupling, observability.

### vs. Channels

```go
// Channel send/receive: ~50-100ns
ch <- event
event := <-ch

// Capitan emit: ~1-2µs

// Overhead: ~20x slower
// Trade-off: Type safety, observers, worker isolation
```

**When to use channels**: Low-level goroutine coordination.

**When to use capitan**: Application-level event coordination.

### vs. Traditional Event Bus

```go
// Traditional event bus (shared lock): ~500ns-1µs
bus.Emit("order.placed", data)

// Capitan (per-signal workers): ~1-2µs

// Similar performance, but:
// - Capitan: Type-safe fields, no cross-signal contention
// - Traditional: Lower latency, higher contention
```

## Optimization Checklist

- [ ] Minimize observer count (use hub pattern)
- [ ] Use observer whitelists when possible
- [ ] Tune buffer sizes for workload
- [ ] Reduce field count to essentials
- [ ] Avoid heavy work in observers
- [ ] Reuse field keys (define as constants)
- [ ] Batch events when appropriate
- [ ] Monitor queue depths
- [ ] Profile hot paths
- [ ] Test with race detector

## Performance Anti-Patterns

### ❌ Creating Capitan Per Request

```go
// Bad: Creates new instance per request
func handleRequest(w http.ResponseWriter, r *http.Request) {
    c := capitan.New() // Expensive!
    defer c.Shutdown()

    c.Emit(ctx, signal, fields...)
}
```

**Fix**: Use single global instance.

### ❌ Many Observers

```go
// Bad: 100 observers
for i := 0; i < 100; i++ {
    c.Observe(handler)
}
```

**Fix**: Use hub pattern (1 observer, dispatch internally).

### ❌ Heavy Work in Observers

```go
// Bad: Blocks Emit()
c.Observe(func(ctx context.Context, e *capitan.Event) {
    time.Sleep(100 * time.Millisecond) // Blocks!
})
```

**Fix**: Queue for async processing.

### ❌ Recreating Field Keys

```go
// Bad: Allocates new key each time
c.Emit(ctx, signal, capitan.NewStringKey("user_id").Field("u1"))
```

**Fix**: Define keys as package constants.

### ❌ Tiny Buffer Sizes

```go
// Bad: Queue fills immediately
c := capitan.New(capitan.WithBufferSize(1))
```

**Fix**: Size buffers for expected load (100+ for production).

## Next Steps

- [Concurrency Guide](./concurrency.md) - Thread safety patterns
- [Best Practices](./best-practices.md) - Production patterns
- [Benchmarks](../../testing/benchmarks/README.md) - Run benchmarks
- [Testing Guide](./testing.md) - Performance testing
