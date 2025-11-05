# capitan

[![CI Status](https://github.com/zoobzio/capitan/workflows/CI/badge.svg)](https://github.com/zoobzio/capitan/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/capitan/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/capitan)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/capitan)](https://goreportcard.com/report/github.com/zoobzio/capitan)
[![CodeQL](https://github.com/zoobzio/capitan/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/capitan/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/capitan.svg)](https://pkg.go.dev/github.com/zoobzio/capitan)
[![License](https://img.shields.io/github/license/zoobzio/capitan)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/capitan)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/capitan)](https://github.com/zoobzio/capitan/releases)

Type-safe event coordination for Go with zero dependencies.

Emit events with typed fields, hook listeners, and let capitan handle the rest with async processing and backpressure.

## The Power of Simplicity

At its core, capitan provides just three operations:

```go
// Emit an event
capitan.Emit(ctx, signal, fields...)

// Hook a listener
listener := capitan.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Handle event
})

// Observe all signals
observer := capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Handle any event
})
```

**That's it.** No schemas to declare, no complex configuration, just events and listeners.

## Quick Start

```go
package main

import (
    "context"
    "github.com/zoobzio/capitan"
)

func main() {
    // Define signal and keys
    orderCreated := capitan.NewSignal("order.created", "New order has been created")
    orderID := capitan.NewStringKey("order_id")
    total := capitan.NewFloat64Key("total")

    // Hook a listener
    capitan.Hook(orderCreated, func(ctx context.Context, e *capitan.Event) {
        id, _ := orderID.From(e)
        amount, _ := total.From(e)
        // Process order...
    })

    // Emit an event (async with backpressure)
    capitan.Emit(context.Background(), orderCreated,
        orderID.Field("ORDER-123"),
        total.Field(99.99),
    )

    // Gracefully drain pending events
    capitan.Shutdown()
}
```

## Why capitan?

- **Type-safe**: Typed fields with compile-time safety
- **Zero dependencies**: Just standard library
- **Async by default**: Non-blocking emission with per-signal worker goroutines and backpressure
- **Lazy**: Workers created only when needed
- **Isolated**: Slow listeners don't affect other signals
- **Panic-safe**: Listener panics recovered, system stays running
- **Configurable**: Optional buffer sizes, panic handlers, and runtime metrics
- **Clean**: No schemas, no boilerplate, no registration ceremony
- **Testable**: Every component independently testable

## Installation

```bash
go get github.com/zoobzio/capitan
```

Requirements: Go 1.23+ (released August 2024)

## Core Concepts

**Signals** identify event types:
```go
userLogin := capitan.NewSignal("user.login", "User successfully logged in")
orderShipped := capitan.NewSignal("order.shipped", "Order has been shipped to customer")
```

**Keys** define typed field names:
```go
userID := capitan.NewStringKey("user_id")
count := capitan.NewIntKey("count")
ratio := capitan.NewFloat64Key("ratio")
active := capitan.NewBoolKey("active")
```

**Fields** carry typed values:
```go
capitan.Emit(context.Background(), userLogin,
    userID.Field("user_123"),
    count.Field(5),
)
```

**Listeners** handle events:
```go
listener := capitan.Hook(userLogin, func(ctx context.Context, e *capitan.Event) {
    uid, _ := userID.From(e)
    // Handle login...
})
```

**Observers** watch all signals (dynamic):
```go
// Observe all signals - receives every event
observer := capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    log.Printf("[%s] %s: %d fields at %s",
        e.Severity(),
        e.Signal().Name(),
        len(e.Fields()),
        e.Timestamp().Format(time.RFC3339))
})

// Observe specific signals only (whitelist)
authObserver := capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Only receives UserLogin, UserLogout, PasswordReset events
    log.Printf("Auth event: %s by user %s",
        e.Signal().Name(),
        mustGetUserID(e))
}, UserLogin, UserLogout, PasswordReset)
```

Observers receive events from both existing signals and any signals created after the observer is registered. This is compatible with lazy signal initialization - observers automatically attach to workers as they're created. When signals are provided to `Observe()`, only those signals are observed (whitelist mode).

### Best Practice: Define Signals and Keys as Constants

**Always define signals and keys as package-level constants:**

```go
// Define signals at package level
var (
    UserLogin    = capitan.NewSignal("user.login", "User successfully logged in")
    UserLogout   = capitan.NewSignal("user.logout", "User logged out of session")
    OrderCreated = capitan.NewSignal("order.created", "New order has been created")
)

// Define keys at package level
var (
    userID   = capitan.NewStringKey("user_id")
    orderID  = capitan.NewStringKey("order_id")
    total    = capitan.NewFloat64Key("total")
)
```

**Why?**

1. **Bounded memory** - Capitan maintains internal registries keyed by signal. A fixed set of signals means predictable memory usage. Dynamic signal creation (e.g., per-user, per-request) will cause unbounded registry growth.

2. **Worker lifecycle** - Each signal gets its own worker goroutine. Dynamic signals create goroutines that persist until explicit cleanup, leading to goroutine leaks in long-running systems.

3. **Predictable behavior** - Const signals make event flow analyzable at compile time. Observers and hooks can be reasoned about statically.

4. **Performance** - Internal lookups are optimized for a stable signal set. Dynamic signals bypass lazy initialization benefits.

**Avoid dynamic signals:**
```go
// BAD: Creates new signal per user
signal := capitan.NewSignal(fmt.Sprintf("user.%s.login", userID), "User login")

// GOOD: Use fields to carry dynamic data
capitan.Emit(context.Background(), UserLogin, userID.Field(id))
```

Use fields to carry variable data, not signal names. Think of signals as event _types_, not event _instances_.

## Real-World Example

Here's a realistic example showing how capitan handles application events:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    "github.com/zoobzio/capitan"
)

// Define signals as constants
var (
    orderCreated = capitan.NewSignal("order.created", "New order has been created")
    orderShipped = capitan.NewSignal("order.shipped", "Order has been shipped to customer")
)

// Define keys
var (
    orderID   = capitan.NewStringKey("order_id")
    userID    = capitan.NewStringKey("user_id")
    total     = capitan.NewFloat64Key("total")
    timestamp = capitan.NewStringKey("timestamp")
)

func main() {
    // Setup logging observer for all events
    capitan.Observe(func(ctx context.Context, e *capitan.Event) {
        log.Printf("[EVENT] %s at %s",
            e.Signal(),
            e.Timestamp().Format(time.RFC3339))
    })

    // Hook order created handler
    capitan.Hook(orderCreated, func(ctx context.Context, e *capitan.Event) {
        id, _ := orderID.From(e)
        amount, _ := total.From(e)

        // Send confirmation email
        fmt.Printf("📧 Sending confirmation for order %s (%.2f)\n", id, amount)

        // Update analytics
        fmt.Printf("📊 Recording order metrics\n")
    })

    // Hook order shipped handler
    capitan.Hook(orderShipped, func(ctx context.Context, e *capitan.Event) {
        id, _ := orderID.From(e)
        user, _ := userID.From(e)

        // Send shipping notification
        fmt.Printf("📦 Notifying user %s: order %s shipped\n", user, id)
    })

    // Emit events (async with backpressure)
    capitan.Emit(context.Background(), orderCreated,
        orderID.Field("ORDER-123"),
        userID.Field("user_456"),
        total.Field(99.99),
    )

    capitan.Emit(context.Background(), orderShipped,
        orderID.Field("ORDER-123"),
        userID.Field("user_456"),
    )

    // Gracefully drain queue before exit
    capitan.Shutdown()
}
```

## Architecture

**Async Execution**: Each signal gets its own worker goroutine, created lazily on first emission. Events are queued (16 buffer default) and processed asynchronously.

**Isolation**: Slow or misbehaving listeners on one signal don't affect other signals. Each signal's queue operates independently.

**Panic Recovery**: Listener panics are caught and recovered silently. One bad listener won't crash your system or prevent other listeners from running.

**Event Pooling**: Events are pooled internally to reduce allocations. Events are returned to the pool after all listeners finish.

**Shutdown**: `Shutdown()` closes all worker goroutines gracefully, processing remaining queued events before exit.

## Concurrency & Ordering

**Per-Signal Ordering**: Events emitted to the same signal are processed in emission order. Each signal's worker processes its queue sequentially.

**Cross-Signal Independence**: No ordering guarantees between different signals. Workers operate concurrently and independently.

```go
Emit(ctx, "order.created", orderID.Field("123"))
Emit(ctx, "email.sent", orderID.Field("123"))
Shutdown()

// order.created's listeners might complete before OR after email.sent's listeners
// Each signal processes independently
```

**Shutdown Behavior**: `Shutdown()` waits for all workers to drain their queues, but workers complete independently. Events queued at shutdown time will be processed before exit.

**Backpressure**: Each signal has a buffered queue (16 events by default). If the queue fills, `Emit()` blocks until space is available. This provides natural backpressure - slow listeners will slow down emitters for that signal only, preventing unbounded memory growth. Other signals are unaffected.

## Configuration

Capitan supports optional configuration for buffer sizes and panic handling.

**Configure the default instance** (must be called before first use):

```go
func main() {
    capitan.Configure(
        capitan.WithBufferSize(128),
        capitan.WithPanicHandler(func(sig capitan.Signal, recovered any) {
            log.Printf("Listener panic on %s: %v", sig, recovered)
        }),
    )

    // Now use module-level API with custom config
    capitan.Hook(signal, handler)
    capitan.Emit(ctx, signal, fields...)
}
```

**Available options:**

- `WithBufferSize(n int)` - Sets event queue buffer size per signal (default: 16). Larger buffers reduce backpressure but increase memory usage.
- `WithPanicHandler(func(Signal, any))` - Called when a listener panics. By default, panics are recovered silently to prevent system crashes.

**Runtime metrics:**

```go
stats := capitan.Stats()
fmt.Printf("Active workers: %d\n", stats.ActiveWorkers)
fmt.Printf("Queue depths: %v\n", stats.QueueDepths)
fmt.Printf("Listener counts: %v\n", stats.ListenerCounts)
fmt.Printf("Emit counts: %v\n", stats.EmitCounts)
fmt.Printf("Field schemas: %v\n", stats.FieldSchemas)
```

For custom instances, use `c.Stats()`.

**Integration with monitoring systems:**

```go
// Expose metrics to Prometheus
func exposeMetrics() {
    stats := capitan.Stats()

    // Active worker goroutines
    activeWorkersGauge.Set(float64(stats.ActiveWorkers))

    // Queue depth per signal
    for signal, depth := range stats.QueueDepths {
        queueDepthGauge.WithLabelValues(signal.Name()).Set(float64(depth))
    }

    // Total emits per signal
    for signal, count := range stats.EmitCounts {
        emitCountCounter.WithLabelValues(signal.Name()).Set(float64(count))
    }
}
```

## Multiple Instances

While the module-level API uses a default singleton, you can create isolated instances with custom configuration:

```go
// Default configuration
c := capitan.New()

// Custom configuration
c := capitan.New(
    capitan.WithBufferSize(256),
    capitan.WithPanicHandler(logPanic),
)

c.Hook(signal, handler)
c.Emit(ctx, signal, fields...)
c.Stats() // Get metrics for this instance
c.Shutdown()
```

## Listener Management

**Close individual listeners**:
```go
listener := capitan.Hook(signal, handler)
// ...later
listener.Close() // Stop receiving events
```

**Close observers**:
```go
observer := capitan.Observe(handler)
// ...later
observer.Close() // Stop all observer listeners
```

## Field Types

Capitan provides built-in field types for common Go types:

- `StringKey` - string values
- `IntKey` - int values
- `Int32Key` - int32 values
- `Int64Key` - int64 values
- `UintKey` - uint values
- `Uint32Key` - uint32 values
- `Uint64Key` - uint64 values
- `Float32Key` - float32 values
- `Float64Key` - float64 values
- `BoolKey` - bool values
- `TimeKey` - time.Time values
- `DurationKey` - time.Duration values
- `BytesKey` - []byte values
- `ErrorKey` - error values

Access typed values using the From() method:
```go
value, ok := key.From(e)  // Returns (value, ok) tuple
if !ok {
    // Field not present or wrong type
}
```

Or via type assertion on fields:
```go
field := e.Get(key)
if gf, ok := field.(capitan.GenericField[string]); ok {
    value := gf.Get()
}
```

### Common Field Patterns

**Error propagation:**

Use `ErrorKey` to propagate errors through events and handle them consistently:

```go
var (
    paymentFailed = capitan.NewSignal("payment.failed", "Payment processing failed")
    errKey        = capitan.NewErrorKey("error")
    orderID       = capitan.NewStringKey("order_id")
    userID        = capitan.NewStringKey("user_id")
)

// Emit error events
if err := processPayment(order); err != nil {
    capitan.Error(ctx, paymentFailed,
        errKey.Field(err),
        orderID.Field(order.ID),
        userID.Field(order.UserID),
    )
}

// Handle error events
capitan.Hook(paymentFailed, func(ctx context.Context, e *capitan.Event) {
    if err, ok := errKey.From(e); ok {
        log.Printf("Payment failed: %v", err)
        // Send alert, trigger retry logic, etc.
    }
})
```

**Severity levels:**

Use severity helper methods to indicate event importance:

```go
// Debug - development and troubleshooting
capitan.Debug(ctx, signal,
    capitan.NewStringKey("sql").Field("SELECT * FROM users WHERE id = ?"))

// Info - normal operations (default for Emit)
capitan.Info(ctx, signal, fields...)

// Warn - warning conditions that need attention
capitan.Warn(ctx, signal,
    capitan.NewIntKey("retry_count").Field(3))

// Error - error conditions requiring immediate action
capitan.Error(ctx, signal,
    errKey.Field(err))
```

Filter by severity in observers:

```go
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Only log warnings and errors
    if e.Severity() == capitan.SeverityWarn || e.Severity() == capitan.SeverityError {
        log.Printf("[%s] %s", e.Severity(), e.Signal().Name())
    }
})
```

### Extending with Custom Types

You can extend capitan with your own field types for structs or custom types using `NewKey[T]`:

```go
// Define your custom type
type OrderInfo struct {
    ID     string
    Total  float64
    Items  int
}

// Create a typed key (one line!)
var orderKey = capitan.NewKey[OrderInfo]("order", "myapp.OrderInfo")

// Use it exactly like built-in types
func main() {
    sig := capitan.NewSignal("order.processed", "Order has been processed successfully")

    capitan.Hook(sig, func(ctx context.Context, e *capitan.Event) {
        order, ok := orderKey.From(e)  // Type-safe extraction
        if ok {
            fmt.Printf("Order %s: $%.2f (%d items)\n",
                order.ID, order.Total, order.Items)
        }
    })

    capitan.Emit(context.Background(), sig, orderKey.Field(OrderInfo{
        ID:    "ORDER-123",
        Total: 99.99,
        Items: 3,
    }))

    capitan.Shutdown()
}
```

**Variant naming**: Use namespaced strings to avoid collisions (e.g., `"myapp.OrderInfo"` or `"github.com/yourorg/yourpkg.TypeName"`).

**Note**: The built-in types (`StringKey`, `IntKey`, `Float64Key`, `BoolKey`) are just aliases of `GenericKey[T]` with predefined variants. You can use `NewKey[T]` for any type.

## Event Access

```go
// Get specific field
field := e.Get(key)

// Get all fields
fields := e.Fields() // Returns []Field

// Access metadata
signal := e.Signal()       // Signal identifier
timestamp := e.Timestamp() // When event was created
```

## Performance

Capitan is designed for performance:

- Lazy worker creation (no upfront cost)
- Event pooling reduces allocations
- Per-signal goroutines prevent contention
- Minimal locking on hot paths
- Zero reflection or runtime type assertions

Run benchmarks:
```bash
go test -bench=.
```

## Testing

Run tests:
```bash
go test -v ./...
```

Run with coverage:
```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `go test ./...`
- Code is formatted: `go fmt ./...`
- No lint errors: `golangci-lint run`

## License

MIT License - see [LICENSE](LICENSE) file for details.
