---
title: Best Practices
description: Production-ready patterns and best practices for capitan applications.
author: Capitan Team
published: 2025-12-01
tags: [Best Practices, Production, Guide]
---

# Best Practices

Production-ready patterns for capitan applications.

## Signal Organization

### Define Signals as Constants

```go
package events

import "github.com/zoobzio/capitan"

// Order lifecycle
var (
    OrderPlaced    = capitan.NewSignal("order.placed", "Order placed by customer")
    OrderValidated = capitan.NewSignal("order.validated", "Order passed validation")
    OrderShipped   = capitan.NewSignal("order.shipped", "Order shipped to customer")
    OrderCompleted = capitan.NewSignal("order.completed", "Order completed successfully")
)

// User lifecycle
var (
    UserRegistered = capitan.NewSignal("user.registered", "New user registered")
    UserVerified   = capitan.NewSignal("user.verified", "User email verified")
    UserDeleted    = capitan.NewSignal("user.deleted", "User account deleted")
)
```

### Use Hierarchical Naming

```go
// ✅ Good: Clear hierarchy
"order.placed"
"order.payment.processed"
"order.payment.failed"
"order.shipped"
"order.delivered"

"user.registered"
"user.verified"
"user.profile.updated"
"user.password.changed"

// ❌ Avoid: Flat naming
"orderplaced"
"paymentprocessed"
"shipped"
```

### Group Related Signals

```go
package events

// Order domain
var (
    OrderPlaced      = capitan.NewSignal("order.placed", "...")
    OrderValidated   = capitan.NewSignal("order.validated", "...")
    OrderShipped     = capitan.NewSignal("order.shipped", "...")
)

// Payment domain
var (
    PaymentRequested = capitan.NewSignal("payment.requested", "...")
    PaymentProcessed = capitan.NewSignal("payment.processed", "...")
    PaymentFailed    = capitan.NewSignal("payment.failed", "...")
)

// Inventory domain
var (
    InventoryReserved = capitan.NewSignal("inventory.reserved", "...")
    InventoryReleased = capitan.NewSignal("inventory.released", "...")
)
```

## Field Key Organization

### Define Field Keys as Constants

```go
package fields

import "github.com/zoobzio/capitan"

// Common identifiers
var (
    UserID    = capitan.NewStringKey("user_id")
    OrderID   = capitan.NewStringKey("order_id")
    ProductID = capitan.NewStringKey("product_id")
    SessionID = capitan.NewStringKey("session_id")
)

// Order fields
var (
    OrderAmount   = capitan.NewIntKey("order_amount")
    OrderStatus   = capitan.NewStringKey("order_status")
    OrderItems    = capitan.NewIntKey("order_items")
)

// User fields
var (
    UserEmail    = capitan.NewStringKey("user_email")
    UserRole     = capitan.NewStringKey("user_role")
    UserVerified = capitan.NewBoolKey("user_verified")
)

// Timestamps
var (
    CreatedAt = capitan.NewTimeKey("created_at")
    UpdatedAt = capitan.NewTimeKey("updated_at")
)
```

### Use Specific Types

```go
// ✅ Good: Type-safe
amountKey := capitan.NewIntKey("amount")
timestampKey := capitan.NewTimeKey("timestamp")
activeKey := capitan.NewBoolKey("active")

// ❌ Avoid: Loses type safety
amountKey := capitan.NewAnyKey("amount")
timestampKey := capitan.NewAnyKey("timestamp")
activeKey := capitan.NewAnyKey("active")
```

### Consistent Naming

```go
// ✅ Good: Consistent snake_case
"user_id"
"order_id"
"created_at"
"is_active"

// ❌ Avoid: Mixed styles
"userId"
"order_id"
"CreatedAt"
"isActive"
```

## Handler Organization

### Keep Handlers Focused

```go
// ✅ Good: Single responsibility
c.Hook(orderPlaced, validateOrder)
c.Hook(orderPlaced, reserveInventory)
c.Hook(orderPlaced, sendConfirmationEmail)

// ❌ Bad: Doing too much
c.Hook(orderPlaced, func(ctx context.Context, e *capitan.Event) {
    validateOrder(e)
    reserveInventory(e)
    sendConfirmationEmail(e)
    updateAnalytics(e)
    logToAudit(e)
    // ... too many responsibilities
})
```

### Extract Handler Functions

```go
// ✅ Good: Named functions, testable
func validateOrder(ctx context.Context, e *capitan.Event) {
    orderID := fields.OrderID.Extract(e)
    // Validation logic
}

func reserveInventory(ctx context.Context, e *capitan.Event) {
    orderID := fields.OrderID.Extract(e)
    // Inventory logic
}

c.Hook(events.OrderPlaced, validateOrder)
c.Hook(events.OrderPlaced, reserveInventory)

// ❌ Avoid: Inline anonymous functions (harder to test)
c.Hook(events.OrderPlaced, func(ctx context.Context, e *capitan.Event) {
    // Lots of inline logic...
})
```

### Service-Based Handlers

```go
type OrderService struct {
    c          *capitan.Capitan
    db         *sql.DB
    inventory  *InventoryService
    listeners  []*capitan.Listener
}

func (s *OrderService) Start() {
    s.listeners = append(s.listeners,
        s.c.Hook(events.OrderPlaced, s.handleOrderPlaced),
        s.c.Hook(events.OrderValidated, s.handleOrderValidated),
        s.c.Hook(events.OrderShipped, s.handleOrderShipped),
    )
}

func (s *OrderService) Stop() {
    for _, l := range s.listeners {
        l.Close()
    }
}

func (s *OrderService) handleOrderPlaced(ctx context.Context, e *capitan.Event) {
    // Access service dependencies
    orderID := fields.OrderID.Extract(e)

    if err := s.db.Save(orderID); err != nil {
        log.Printf("Error saving order: %v", err)
        return
    }

    s.c.Emit(ctx, events.OrderValidated, fields.OrderID.Field(orderID))
}
```

## Error Handling

### Handle Errors Gracefully

```go
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    if err := processEvent(e); err != nil {
        log.Printf("Error processing event: %v", err)

        // Emit error event
        c.EmitWithSeverity(ctx, capitan.SeverityError, events.ProcessingFailed,
            fields.ErrorMessage.Field(err.Error()),
            fields.OriginalSignal.Field(e.Signal().Name()),
        )

        // Don't panic - would affect all emitters
        return
    }
})
```

### Use Severity Levels

```go
// Debug: Development info
c.EmitWithSeverity(ctx, capitan.SeverityDebug, events.CacheHit,
    fields.CacheKey.Field(key),
)

// Info: Normal operations
c.EmitWithSeverity(ctx, capitan.SeverityInfo, events.UserLogin,
    fields.UserID.Field(userID),
)

// Warn: Potential issues
c.EmitWithSeverity(ctx, capitan.SeverityWarn, events.RateLimitApproaching,
    fields.UserID.Field(userID),
    fields.CurrentRate.Field(rate),
)

// Error: Failed operations
c.EmitWithSeverity(ctx, capitan.SeverityError, events.PaymentFailed,
    fields.OrderID.Field(orderID),
    fields.ErrorMessage.Field(err.Error()),
)

// Critical: System-level failures
c.EmitWithSeverity(ctx, capitan.SeverityCritical, events.DatabaseDown,
    fields.ErrorMessage.Field(err.Error()),
)
```

### Create Error Events

```go
// Define error signals
var (
    OrderValidationFailed = capitan.NewSignal("order.validation.failed", "Order validation failed")
    PaymentDeclined       = capitan.NewSignal("payment.declined", "Payment declined")
    InventoryUnavailable  = capitan.NewSignal("inventory.unavailable", "Inventory unavailable")
)

// Error field keys
var (
    ErrorMessage = capitan.NewStringKey("error_message")
    ErrorCode    = capitan.NewStringKey("error_code")
    FailureReason = capitan.NewStringKey("failure_reason")
)

// Emit errors with context
c.EmitWithSeverity(ctx, capitan.SeverityError, events.PaymentDeclined,
    fields.OrderID.Field(orderID),
    fields.ErrorCode.Field("CARD_DECLINED"),
    fields.ErrorMessage.Field("Insufficient funds"),
)
```

## Context Usage

### Propagate Context

```go
// ✅ Good: Use provided context
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Use ctx for downstream calls
    result, err := api.CallWithContext(ctx, data)
    if err != nil {
        return
    }

    // Propagate ctx to cascading events
    c.Emit(ctx, nextSignal, fields...)
})

// ❌ Bad: Creating new context
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Loses trace IDs, cancellation, etc.
    result, err := api.Call(context.Background(), data)
})
```

### Respect Cancellation

```go
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Check before expensive work
    select {
    case <-ctx.Done():
        log.Println("Context cancelled, aborting")
        return
    default:
    }

    // Do work
    result := expensiveOperation()

    // Check again before continuing
    if ctx.Err() != nil {
        return
    }

    saveResult(result)
})
```

### Add Request Metadata

```go
type requestKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
    return context.WithValue(ctx, requestKey{}, requestID)
}

func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(requestKey{}).(string); ok {
        return id
    }
    return ""
}

// Use in handlers
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    requestID := GetRequestID(ctx)
    log.Printf("[%s] Processing event", requestID)
})
```

## Configuration

### Single Capitan Instance

```go
// ✅ Good: One instance per application
var globalCapitan *capitan.Capitan

func init() {
    globalCapitan = capitan.New(
        capitan.WithBufferSize(100),
    )
}

// ❌ Bad: Multiple instances (confusing)
c1 := capitan.New()
c2 := capitan.New()
```

### Configure Buffer Sizes

```go
// Development: Small buffers for fast feedback
c := capitan.New(capitan.WithBufferSize(10))

// Production: Larger buffers for traffic spikes
c := capitan.New(capitan.WithBufferSize(500))

// High-throughput: Very large buffers
c := capitan.New(capitan.WithBufferSize(5000))
```

### Environment-Based Configuration

```go
func NewCapitan() *capitan.Capitan {
    bufferSize := 100

    if size := os.Getenv("CAPITAN_BUFFER_SIZE"); size != "" {
        if parsed, err := strconv.Atoi(size); err == nil {
            bufferSize = parsed
        }
    }

    return capitan.New(capitan.WithBufferSize(bufferSize))
}
```

## Lifecycle Management

### Graceful Shutdown

```go
func main() {
    c := capitan.New()
    defer c.Shutdown()

    // Handle signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Start application
    startServices(c)

    // Wait for shutdown signal
    <-sigChan
    log.Println("Shutting down...")

    // Cleanup
    stopServices()
    c.Shutdown() // Waits for all events to process

    log.Println("Shutdown complete")
}
```

### Service Lifecycle

```go
type Service struct {
    c         *capitan.Capitan
    listeners []*capitan.Listener
    observers []*capitan.Observer
}

func (s *Service) Start() error {
    // Register listeners
    s.listeners = append(s.listeners,
        s.c.Hook(signal1, s.handler1),
        s.c.Hook(signal2, s.handler2),
    )

    // Register observers
    s.observers = append(s.observers,
        s.c.Observe(s.auditHandler),
    )

    return nil
}

func (s *Service) Stop() error {
    // Close all listeners
    for _, l := range s.listeners {
        l.Close()
    }
    s.listeners = nil

    // Close all observers
    for _, o := range s.observers {
        o.Close()
    }
    s.observers = nil

    return nil
}
```

## Testing

### Use TestCapitan Helper

```go
func TestFeature(t *testing.T) {
    c := capitantesting.TestCapitan()
    defer c.Shutdown()

    // Test code
}
```

### Use Event Capture

```go
func TestWorkflow(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    // Capture events
    capture := capitantesting.NewEventCapture()
    c.Hook(signal, capture.Handler())

    // Trigger workflow
    c.Emit(context.Background(), signal, fields...)
    c.Shutdown()

    // Verify
    events := capture.Events()
    require.Equal(t, 1, len(events))
}
```

### Test Error Paths

```go
func TestErrorHandling(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    failedCapture := capitantesting.NewEventCapture()
    c.Hook(events.OrderFailed, failedCapture.Handler())

    // Trigger error condition
    c.Emit(context.Background(), events.OrderPlaced,
        fields.OrderID.Field("INVALID"),
    )

    c.Shutdown()

    // Verify error event
    events := failedCapture.Events()
    require.Equal(t, 1, len(events))
}
```

## Monitoring

### Emit Metrics

```go
c.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        metrics.RecordDuration("handler.duration", duration,
            "signal", e.Signal().Name(),
        )
    }()

    processEvent(e)
})
```

### Track Queue Depths

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stats := c.Stats()

        for signal, count := range stats.PendingEvents {
            metrics.Gauge("capitan.queue.depth", float64(count),
                "signal", signal,
            )
        }
    }
}()
```

### Log Important Events

```go
c.Hook(events.OrderPlaced, func(ctx context.Context, e *capitan.Event) {
    orderID := fields.OrderID.Extract(e)
    amount := fields.OrderAmount.Extract(e)

    log.Printf("Order placed: id=%s amount=$%.2f", orderID, float64(amount)/100)
})
```

## Security

### Validate Input

```go
c.Hook(events.UserInput, func(ctx context.Context, e *capitan.Event) {
    input := fields.UserInput.Extract(e)

    // Validate
    if !isValid(input) {
        c.EmitWithSeverity(ctx, capitan.SeverityWarn, events.InvalidInput,
            fields.InputValue.Field(input),
        )
        return
    }

    // Process
    processInput(input)
})
```

### Sanitize Sensitive Data

```go
c.Observe(func(ctx context.Context, e *capitan.Event) {
    // Don't log sensitive fields
    for _, field := range e.Fields() {
        if field.Key == "password" || field.Key == "credit_card" {
            continue
        }
        log.Printf("%s: %v", field.Key, field.Value)
    }
})
```

### Audit Security Events

```go
securitySignals := []capitan.Signal{
    events.UserLogin,
    events.UserLoginFailed,
    events.PasswordChanged,
    events.PermissionGranted,
    events.DataExported,
}

securityObserver := c.Observe(auditSecurityEvent, securitySignals...)
```

## Performance

### Minimize Observer Count

```go
// ❌ Bad: Multiple observers
c.Observe(auditHandler)
c.Observe(metricsHandler)
c.Observe(debugHandler)
c.Observe(tracingHandler)

// ✅ Good: Single observer hub
type ObserverHub struct {
    handlers []capitan.EventCallback
}

hub := &ObserverHub{
    handlers: []capitan.EventCallback{
        auditHandler,
        metricsHandler,
        debugHandler,
        tracingHandler,
    },
}

c.Observe(hub.Dispatch)
```

### Use Whitelists

```go
// Observe only relevant signals
observer := c.Observe(handler, signal1, signal2, signal3)
```

### Avoid Heavy Work in Observers

```go
// ❌ Bad: Slow observer blocks Emit()
c.Observe(func(ctx context.Context, e *capitan.Event) {
    db.Save(e) // Blocks every Emit()!
})

// ✅ Good: Queue for async processing
queue := make(chan *capitan.Event, 100)

c.Observe(func(ctx context.Context, e *capitan.Event) {
    queue <- copyEvent(e) // Quick enqueue
})

// Process async
go func() {
    for e := range queue {
        db.Save(e)
    }
}()
```

## Documentation

### Document Signals

```go
// OrderPlaced is emitted when a customer successfully places an order.
// Fields: order_id (string), customer_id (string), amount (int)
var OrderPlaced = capitan.NewSignal("order.placed", "Order placed by customer")
```

### Document Field Keys

```go
// OrderID uniquely identifies an order across the system.
// Used in: order.placed, order.validated, order.shipped
var OrderID = capitan.NewStringKey("order_id")
```

### Document Workflows

```go
// Order Processing Workflow:
// 1. order.placed → validateOrder() → order.validated
// 2. order.validated → reserveInventory() → inventory.reserved
// 3. inventory.reserved → processPayment() → payment.processed
// 4. payment.processed → shipOrder() → order.shipped
```

## Next Steps

- [Testing Guide](./testing.md) - Test best practices
- [Performance Guide](./performance.md) - Optimization techniques
- [Concurrency Guide](./concurrency.md) - Thread safety patterns
- [Event Patterns](./patterns.md) - Common patterns
