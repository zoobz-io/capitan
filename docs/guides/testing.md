---
title: Testing Guide
description: Comprehensive testing patterns for capitan event systems using the testing infrastructure.
author: Capitan Team
published: 2025-12-01
tags: [Testing, Guide, Best Practices]
---

# Testing Guide

Complete guide to testing event coordination systems built with capitan.

## Overview

Capitan provides a comprehensive testing infrastructure in the `github.com/zoobzio/capitan/testing` package. These helpers solve common testing challenges, particularly event pooling issues.

## Quick Start

```go
import (
    "testing"
    "github.com/zoobzio/capitan"
    capitantesting "github.com/zoobzio/capitan/testing"
)

func TestUserRegistration(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    signal := capitan.NewSignal("user.registered", "User registered")
    emailKey := capitan.NewStringKey("email")

    // Capture events with defensive copying
    capture := capitantesting.NewEventCapture()
    c.Hook(signal, capture.Handler())

    // Emit event
    c.Emit(context.Background(), signal, emailKey.Field("test@example.com"))

    // Shutdown to drain queues
    c.Shutdown()

    // Verify
    events := capture.Events()
    require.Equal(t, 1, len(events))

    email := emailKey.ExtractFromFields(events[0].Fields)
    assert.Equal(t, "test@example.com", email)
}
```

## Testing Helpers

### EventCapture

Captures events with defensive copying to avoid event pooling issues:

```go
func TestEventCapture(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    signal := capitan.NewSignal("test", "Test signal")
    key := capitan.NewStringKey("message")

    // Create capture
    capture := capitantesting.NewEventCapture()
    c.Hook(signal, capture.Handler())

    // Emit multiple events
    c.Emit(context.Background(), signal, key.Field("first"))
    c.Emit(context.Background(), signal, key.Field("second"))
    c.Emit(context.Background(), signal, key.Field("third"))

    c.Shutdown()

    // Get captured events
    events := capture.Events()
    assert.Equal(t, 3, len(events))

    // Events are defensive copies - safe to access
    assert.Equal(t, "first", key.ExtractFromFields(events[0].Fields))
    assert.Equal(t, "second", key.ExtractFromFields(events[1].Fields))
    assert.Equal(t, "third", key.ExtractFromFields(events[2].Fields))
}
```

### EventCounter

Count events efficiently:

```go
func TestEventCounter(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    signal := capitan.NewSignal("counter", "Counter signal")

    // Create counter
    counter := capitantesting.NewEventCounter()
    c.Hook(signal, counter.Handler())

    // Emit events
    for i := 0; i < 100; i++ {
        c.Emit(context.Background(), signal)
    }

    c.Shutdown()

    // Verify count
    assert.Equal(t, int64(100), counter.Count())
}
```

### TestCapitan

Pre-configured capitan for tests (async mode):

```go
func TestWithTestCapitan(t *testing.T) {
    c := capitantesting.TestCapitan()
    defer c.Shutdown()

    signal := capitan.NewSignal("test", "Test")

    counter := capitantesting.NewEventCounter()
    c.Hook(signal, counter.Handler())

    c.Emit(context.Background(), signal)
    c.Shutdown()

    assert.Equal(t, int64(1), counter.Count())
}
```

## Event Pooling Awareness

Events are pooled via `sync.Pool` - never store event pointers:

### ❌ Wrong - Stores Pointer

```go
// BUG: Event pointer gets reused by pool
var events []*capitan.Event

c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    events = append(events, e) // WRONG!
})
```

### ✅ Right - Defensive Copy

```go
// Use EventCapture helper (does defensive copying)
capture := capitantesting.NewEventCapture()
c.Hook(signal, capture.Handler())

// Or manually copy data you need
var data []string
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    data = append(data, key.Extract(e)) // Extract value, not event
})
```

## Async Testing Pattern

Capitan processes events asynchronously. Use `Shutdown()` to drain queues:

```go
func TestAsyncProcessing(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    signal := capitan.NewSignal("async", "Async test")
    capture := capitantesting.NewEventCapture()
    c.Hook(signal, capture.Handler())

    // Emit events
    c.Emit(context.Background(), signal)
    c.Emit(context.Background(), signal)

    // Shutdown waits for all workers to drain queues
    c.Shutdown()

    // Now safe to check results
    events := capture.Events()
    assert.Equal(t, 2, len(events))
}
```

## Cascading Events

For cascading events, add a small delay before shutdown:

```go
func TestCascadingEvents(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    sig1 := capitan.NewSignal("step1", "Step 1")
    sig2 := capitan.NewSignal("step2", "Step 2")
    sig3 := capitan.NewSignal("step3", "Step 3")

    // Step 1 → Step 2 → Step 3
    c.Hook(sig1, func(ctx context.Context, e *capitan.Event) {
        c.Emit(ctx, sig2)
    })

    c.Hook(sig2, func(ctx context.Context, e *capitan.Event) {
        c.Emit(ctx, sig3)
    })

    capture := capitantesting.NewEventCapture()
    c.Hook(sig3, capture.Handler())

    // Trigger cascade
    c.Emit(context.Background(), sig1)

    // Give time for cascade to propagate
    time.Sleep(50 * time.Millisecond)
    c.Shutdown()

    // Verify final event received
    events := capture.Events()
    assert.Equal(t, 1, len(events))
}
```

## Testing Workflows

Test multi-step workflows with event capture:

```go
func TestOrderWorkflow(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    // Define signals
    orderPlaced := capitan.NewSignal("order.placed", "Order placed")
    orderValidated := capitan.NewSignal("order.validated", "Order validated")
    orderShipped := capitan.NewSignal("order.shipped", "Order shipped")

    orderIDKey := capitan.NewStringKey("order_id")

    // Capture final event
    shippedCapture := capitantesting.NewEventCapture()
    c.Hook(orderShipped, shippedCapture.Handler())

    // Wire workflow
    c.Hook(orderPlaced, func(ctx context.Context, e *capitan.Event) {
        orderID := orderIDKey.Extract(e)
        // Validate...
        c.Emit(ctx, orderValidated, orderIDKey.Field(orderID))
    })

    c.Hook(orderValidated, func(ctx context.Context, e *capitan.Event) {
        orderID := orderIDKey.Extract(e)
        // Ship...
        c.Emit(ctx, orderShipped, orderIDKey.Field(orderID))
    })

    // Trigger workflow
    c.Emit(context.Background(), orderPlaced, orderIDKey.Field("ORD-123"))

    time.Sleep(50 * time.Millisecond)
    c.Shutdown()

    // Verify
    events := shippedCapture.Events()
    require.Equal(t, 1, len(events))

    orderID := orderIDKey.ExtractFromFields(events[0].Fields)
    assert.Equal(t, "ORD-123", orderID)
}
```

## Testing Error Handling

Verify error paths with severity filtering:

```go
func TestErrorHandling(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    orderPlaced := capitan.NewSignal("order.placed", "Order placed")
    orderFailed := capitan.NewSignal("order.failed", "Order failed")

    orderIDKey := capitan.NewStringKey("order_id")
    reasonKey := capitan.NewStringKey("reason")

    // Capture failures
    failedCapture := capitantesting.NewEventCapture()
    c.Hook(orderFailed, failedCapture.Handler())

    // Simulate failure
    c.Hook(orderPlaced, func(ctx context.Context, e *capitan.Event) {
        orderID := orderIDKey.Extract(e)

        // Validation fails
        c.EmitWithSeverity(ctx, capitan.SeverityError, orderFailed,
            orderIDKey.Field(orderID),
            reasonKey.Field("invalid order"),
        )
    })

    // Trigger
    c.Emit(context.Background(), orderPlaced, orderIDKey.Field("BAD-123"))
    c.Shutdown()

    // Verify error
    events := failedCapture.Events()
    require.Equal(t, 1, len(events))

    assert.Equal(t, capitan.SeverityError, events[0].Severity)

    reason := reasonKey.ExtractFromFields(events[0].Fields)
    assert.Equal(t, "invalid order", reason)
}
```

## Testing Observers

Test observer behavior with whitelists:

```go
func TestObserver(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    sig1 := capitan.NewSignal("event1", "Event 1")
    sig2 := capitan.NewSignal("event2", "Event 2")
    sig3 := capitan.NewSignal("event3", "Event 3")

    // Observer receives all events
    allCapture := capitantesting.NewEventCapture()
    observer := c.Observe(allCapture.Handler())
    defer observer.Close()

    // Emit to multiple signals
    c.Emit(context.Background(), sig1)
    c.Emit(context.Background(), sig2)
    c.Emit(context.Background(), sig3)

    c.Shutdown()

    // Verify observer saw all
    events := allCapture.Events()
    assert.Equal(t, 3, len(events))
}

func TestObserverWhitelist(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    sig1 := capitan.NewSignal("sensitive1", "Sensitive 1")
    sig2 := capitan.NewSignal("sensitive2", "Sensitive 2")
    sig3 := capitan.NewSignal("public", "Public")

    // Observer only watches sensitive signals
    sensitiveCapture := capitantesting.NewEventCapture()
    observer := c.Observe(sensitiveCapture.Handler(), sig1, sig2)
    defer observer.Close()

    c.Emit(context.Background(), sig1)
    c.Emit(context.Background(), sig2)
    c.Emit(context.Background(), sig3) // Not in whitelist

    c.Shutdown()

    // Verify only sensitive signals received
    events := sensitiveCapture.Events()
    assert.Equal(t, 2, len(events))
}
```

## Testing Concurrency

Test concurrent event emission:

```go
func TestConcurrentEmission(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    signal := capitan.NewSignal("concurrent", "Concurrent test")
    counter := capitantesting.NewEventCounter()
    c.Hook(signal, counter.Handler())

    const numGoroutines = 10
    const eventsPerGoroutine = 100

    var wg sync.WaitGroup
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < eventsPerGoroutine; j++ {
                c.Emit(context.Background(), signal)
            }
        }()
    }

    wg.Wait()
    c.Shutdown()

    // Verify all events received
    expectedCount := numGoroutines * eventsPerGoroutine
    assert.Equal(t, int64(expectedCount), counter.Count())
}
```

## Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name      string
        quantity  int
        wantError bool
        reason    string
    }{
        {
            name:      "valid quantity",
            quantity:  5,
            wantError: false,
        },
        {
            name:      "zero quantity",
            quantity:  0,
            wantError: true,
            reason:    "quantity must be positive",
        },
        {
            name:      "negative quantity",
            quantity:  -5,
            wantError: true,
            reason:    "quantity must be positive",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c := capitan.New()
            defer c.Shutdown()

            validated := capitan.NewSignal("validated", "Validated")
            failed := capitan.NewSignal("failed", "Failed")

            quantityKey := capitan.NewIntKey("quantity")
            reasonKey := capitan.NewStringKey("reason")

            validCapture := capitantesting.NewEventCapture()
            failCapture := capitantesting.NewEventCapture()

            c.Hook(validated, validCapture.Handler())
            c.Hook(failed, failCapture.Handler())

            // Validation logic
            c.Hook(capitan.NewSignal("check", "Check"), func(ctx context.Context, e *capitan.Event) {
                qty := quantityKey.Extract(e)
                if qty <= 0 {
                    c.Emit(ctx, failed, reasonKey.Field("quantity must be positive"))
                } else {
                    c.Emit(ctx, validated)
                }
            })

            // Trigger
            c.Emit(context.Background(), capitan.NewSignal("check", "Check"),
                quantityKey.Field(tt.quantity))

            c.Shutdown()

            if tt.wantError {
                assert.Equal(t, 0, len(validCapture.Events()))
                require.Equal(t, 1, len(failCapture.Events()))

                reason := reasonKey.ExtractFromFields(failCapture.Events()[0].Fields)
                assert.Equal(t, tt.reason, reason)
            } else {
                assert.Equal(t, 1, len(validCapture.Events()))
                assert.Equal(t, 0, len(failCapture.Events()))
            }
        })
    }
}
```

## Integration Tests

Write integration tests in `testing/integration/`:

```go
// testing/integration/my_feature_test.go
package integration

import (
    "context"
    "testing"
    "time"

    "github.com/zoobzio/capitan"
    capitantesting "github.com/zoobzio/capitan/testing"
)

func TestCompleteWorkflow(t *testing.T) {
    c := capitantesting.TestCapitan()
    defer c.Shutdown()

    // Set up complete system
    // Wire all handlers
    // Trigger workflow
    // Verify results
}
```

## Best Practices

### 1. Always Call Shutdown()

```go
func TestExample(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown() // Ensures cleanup

    // ... test code

    c.Shutdown() // Drain queues before assertions
}
```

### 2. Use Helpers for Event Capture

```go
// ✅ Good: Use helper (handles pooling)
capture := capitantesting.NewEventCapture()
c.Hook(signal, capture.Handler())

// ❌ Bad: Manual capture (pooling bug)
var events []*capitan.Event
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    events = append(events, e) // BUG!
})
```

### 3. Test Error Paths

```go
func TestBothPaths(t *testing.T) {
    // Test success
    t.Run("success", func(t *testing.T) {
        // ...
    })

    // Test failure
    t.Run("failure", func(t *testing.T) {
        // ...
    })
}
```

### 4. Add Small Delays for Cascades

```go
// Cascading events need time to propagate
c.Emit(context.Background(), trigger)
time.Sleep(50 * time.Millisecond)
c.Shutdown()
```

### 5. Use Race Detector

```bash
go test -race ./...
```

## Common Pitfalls

### ❌ Not Calling Shutdown()

```go
// Events may not be processed yet
c.Emit(context.Background(), signal)
events := capture.Events() // May be empty!
```

### ✅ Call Shutdown()

```go
c.Emit(context.Background(), signal)
c.Shutdown() // Wait for processing
events := capture.Events() // All events processed
```

### ❌ Storing Event Pointers

```go
var e *capitan.Event
c.Hook(signal, func(ctx context.Context, event *capitan.Event) {
    e = event // BUG: Pooled event
})
```

### ✅ Extract Values

```go
var value string
c.Hook(signal, func(ctx context.Context, event *capitan.Event) {
    value = key.Extract(event) // Copy value
})
```

## Next Steps

- [Event Patterns](./patterns.md) - Common event coordination patterns
- [Concurrency Guide](./concurrency.md) - Thread-safe event handling
- [Best Practices](./best-practices.md) - Production patterns
- [Integration Tests](../../testing/integration/README.md) - Integration test examples
