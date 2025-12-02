---
title: Quick Start
description: Build your first event system with capitan in 5 minutes.
author: Capitan Team
published: 2025-12-01
tags: [Tutorial, Quick Start, Getting Started]
---

# Quick Start

Build your first event system with capitan in 5 minutes.

## Installation

```bash
go get github.com/zoobzio/capitan
```

Requires Go 1.23+ for testing features.

## Your First Event

Create a simple event coordination system:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/zoobzio/capitan"
)

func main() {
    // Create capitan instance
    c := capitan.New()
    defer c.Shutdown()

    // Define a signal
    userRegistered := capitan.NewSignal("user.registered", "New user registered")

    // Define typed field keys
    emailKey := capitan.NewStringKey("email")
    nameKey := capitan.NewStringKey("name")

    // Hook a listener
    c.Hook(userRegistered, func(ctx context.Context, e *capitan.Event) {
        email := emailKey.Extract(e)
        name := nameKey.Extract(e)

        fmt.Printf("Welcome %s (%s)!\n", name, email)
    })

    // Emit an event
    c.Emit(context.Background(), userRegistered,
        emailKey.Field("alice@example.com"),
        nameKey.Field("Alice"),
    )

    // Output: Welcome Alice (alice@example.com)!
}
```

## Multiple Listeners

Add more listeners to the same signal:

```go
// Send welcome email
c.Hook(userRegistered, func(ctx context.Context, e *capitan.Event) {
    email := emailKey.Extract(e)
    sendEmail(email, "Welcome to our platform!")
})

// Track analytics
c.Hook(userRegistered, func(ctx context.Context, e *capitan.Event) {
    analytics.Track("user_registered", map[string]any{
        "email": emailKey.Extract(e),
        "time":  e.Timestamp(),
    })
})

// Update metrics
c.Hook(userRegistered, func(ctx context.Context, e *capitan.Event) {
    metrics.Inc("users_total")
})
```

## Using Observers

Observe all events for cross-cutting concerns:

```go
// Audit logging for all events
observer := c.Observe(func(ctx context.Context, e *capitan.Event) {
    log.Printf("[AUDIT] %s at %s", e.Signal().Name(), e.Timestamp())
})
defer observer.Close()
```

## Event Cascading

Events can trigger other events:

```go
orderPlaced := capitan.NewSignal("order.placed", "Order placed")
inventoryChecked := capitan.NewSignal("inventory.checked", "Inventory checked")
paymentProcessed := capitan.NewSignal("payment.processed", "Payment processed")

orderIDKey := capitan.NewStringKey("order_id")

// Step 1: Order placed → Check inventory
c.Hook(orderPlaced, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if checkInventory(orderID) {
        c.Emit(ctx, inventoryChecked, orderIDKey.Field(orderID))
    }
})

// Step 2: Inventory checked → Process payment
c.Hook(inventoryChecked, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if processPayment(orderID) {
        c.Emit(ctx, paymentProcessed, orderIDKey.Field(orderID))
    }
})

// Step 3: Payment processed → Ship order
c.Hook(paymentProcessed, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)
    shipOrder(orderID)
})

// Trigger the workflow
c.Emit(context.Background(), orderPlaced, orderIDKey.Field("ORD-123"))
```

## Error Handling

Handle errors in listeners gracefully:

```go
c.Hook(paymentProcessed, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if err := shipOrder(orderID); err != nil {
        log.Printf("Failed to ship order %s: %v", orderID, err)

        // Emit error event
        errorKey := capitan.NewStringKey("error")
        c.EmitWithSeverity(ctx, capitan.SeverityError, shipmentFailed,
            orderIDKey.Field(orderID),
            errorKey.Field(err.Error()),
        )
    }
})
```

## Testing Your Events

Use the testing helpers:

```go
func TestUserRegistration(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    userRegistered := capitan.NewSignal("user.registered", "User registered")
    emailKey := capitan.NewStringKey("email")

    // Capture events
    capture := capitantesting.NewEventCapture()
    c.Hook(userRegistered, capture.Handler())

    // Emit event
    c.Emit(context.Background(), userRegistered,
        emailKey.Field("test@example.com"),
    )

    // Wait for processing
    c.Shutdown()

    // Verify
    events := capture.Events()
    require.Equal(t, 1, len(events))

    // Extract field from captured event
    email := emailKey.ExtractFromFields(events[0].Fields)
    assert.Equal(t, "test@example.com", email)
}
```

## Configuration

Customize capitan behavior:

```go
c := capitan.New(
    capitan.WithBufferSize(100),     // 100-event queue per signal
    capitan.WithSyncMode(),           // Synchronous (testing only)
)
```

## What's Next?

- [Getting Started](./getting-started.md) - Complete tutorial with examples
- [First Event System](./first-event-system.md) - Build a real-world system
- [Core Concepts](../learn/core-concepts.md) - Deep dive into signals and observers
- [Testing Guide](../guides/testing.md) - Comprehensive testing patterns
