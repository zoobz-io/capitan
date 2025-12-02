---
title: Getting Started with Capitan
description: Build your first event coordination system in 10 minutes with complete examples and best practices.
author: Capitan Team
published: 2025-12-01
tags: [Tutorial, Getting Started, Guide]
---

# Getting Started with Capitan

Build your first event coordination system in 10 minutes.

## What is Capitan?

Capitan is a type-safe event coordination library for Go. It provides structured event handling with per-signal worker isolation, ensuring events flow predictably and efficiently through your application.

Think of capitan as a coordination layer between your application components - when something happens, capitan ensures all interested parties are notified reliably and in order.

## Installation

```bash
go get github.com/zoobzio/capitan
```

Requires Go 1.23+ for generics support.

## Basic Concepts

Before diving in, understand these core concepts:

- **Signal**: A named event type (e.g., "order.placed")
- **Listener**: A handler function that responds to a signal
- **Observer**: A handler that watches multiple signals
- **Event**: An instance of a signal with typed data fields
- **Worker**: A per-signal goroutine that processes events

## Your First Event System

Let's build a user registration system with email verification:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/zoobzio/capitan"
)

// Define signals
var (
    UserRegistered = capitan.NewSignal("user.registered", "User registered")
    EmailSent      = capitan.NewSignal("email.sent", "Email sent")
    UserVerified   = capitan.NewSignal("user.verified", "User verified")
)

// Define field keys
var (
    UserIDKey    = capitan.NewStringKey("user_id")
    EmailKey     = capitan.NewStringKey("email")
    UsernameKey  = capitan.NewStringKey("username")
    VerifyToken  = capitan.NewStringKey("verify_token")
)

type User struct {
    ID       string
    Email    string
    Username string
    Verified bool
}

func main() {
    // Create capitan instance
    c := capitan.New()
    defer c.Shutdown()

    // Set up event handlers
    setupHandlers(c)

    // Register a user (triggers the workflow)
    registerUser(c, User{
        ID:       "user-123",
        Email:    "alice@example.com",
        Username: "alice",
    })

    // Wait for async processing
    time.Sleep(100 * time.Millisecond)
}

func setupHandlers(c *capitan.Capitan) {
    // When user registers, send verification email
    c.Hook(UserRegistered, func(ctx context.Context, e *capitan.Event) {
        userID := UserIDKey.Extract(e)
        email := EmailKey.Extract(e)

        log.Printf("Sending verification email to %s", email)

        // Generate token and send email
        token := generateToken(userID)
        sendVerificationEmail(email, token)

        // Emit email sent event
        c.Emit(ctx, EmailSent,
            UserIDKey.Field(userID),
            EmailKey.Field(email),
            VerifyToken.Field(token),
        )
    })

    // When user registers, update analytics
    c.Hook(UserRegistered, func(ctx context.Context, e *capitan.Event) {
        username := UsernameKey.Extract(e)
        log.Printf("Tracking registration: %s", username)
        // analytics.Track("user_registered", ...)
    })

    // When user registers, send welcome notification
    c.Hook(UserRegistered, func(ctx context.Context, e *capitan.Event) {
        email := EmailKey.Extract(e)
        log.Printf("Sending welcome notification to %s", email)
        // notification.Send(...)
    })

    // When email sent, log it
    c.Hook(EmailSent, func(ctx context.Context, e *capitan.Event) {
        email := EmailKey.Extract(e)
        log.Printf("Email sent successfully to %s", email)
    })

    // When user verified, celebrate
    c.Hook(UserVerified, func(ctx context.Context, e *capitan.Event) {
        username := UsernameKey.Extract(e)
        log.Printf("🎉 User %s is verified!", username)
    })
}

func registerUser(c *capitan.Capitan, user User) {
    log.Printf("Registering user: %s (%s)", user.Username, user.Email)

    // Save to database
    // db.SaveUser(user)

    // Emit user registered event
    c.Emit(context.Background(), UserRegistered,
        UserIDKey.Field(user.ID),
        EmailKey.Field(user.Email),
        UsernameKey.Field(user.Username),
    )
}

func generateToken(userID string) string {
    return fmt.Sprintf("tok_%s_%d", userID, time.Now().Unix())
}

func sendVerificationEmail(email, token string) {
    // Send email via SMTP
    log.Printf("📧 Verification link: https://example.com/verify?token=%s", token)
}
```

Output:
```
Registering user: alice (alice@example.com)
Sending verification email to alice@example.com
Tracking registration: alice
Sending welcome notification to alice@example.com
📧 Verification link: https://example.com/verify?token=tok_user-123_1701446400
Email sent successfully to alice@example.com
```

## Understanding the Flow

Let's break down what happens:

```
registerUser()
    │
    ▼
c.Emit(UserRegistered, ...)
    │
    ├─────────────────┬─────────────────┬─────────────────┐
    ▼                 ▼                 ▼                 ▼
Send Verification  Update Analytics  Send Welcome   (All run in parallel)
    │
    ▼
c.Emit(EmailSent, ...)
    │
    ▼
Log Email Sent
```

Key observations:

1. **Multiple listeners** - Three handlers respond to `UserRegistered`
2. **Sequential per signal** - Listeners run in order within each signal's worker
3. **Cascading events** - `UserRegistered` triggers `EmailSent`
4. **Type safety** - Field keys ensure compile-time correctness

## Adding Observers

Observers watch all signals for cross-cutting concerns:

```go
// Audit logging observer
observer := c.Observe(func(ctx context.Context, e *capitan.Event) {
    log.Printf("[AUDIT] Signal: %s, Time: %s, Fields: %v",
        e.Signal().Name(),
        e.Timestamp().Format(time.RFC3339),
        e.Fields(),
    )
})
defer observer.Close()

// Now all events are logged
registerUser(c, user)
```

Output with observer:
```
[AUDIT] Signal: user.registered, Time: 2025-12-01T10:30:00Z, Fields: [...]
[AUDIT] Signal: email.sent, Time: 2025-12-01T10:30:00Z, Fields: [...]
```

## Building a Workflow

Let's expand to a complete order processing workflow:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/zoobzio/capitan"
)

// Order lifecycle signals
var (
    OrderPlaced      = capitan.NewSignal("order.placed", "Order placed")
    InventoryChecked = capitan.NewSignal("inventory.checked", "Inventory checked")
    PaymentProcessed = capitan.NewSignal("payment.processed", "Payment processed")
    OrderShipped     = capitan.NewSignal("order.shipped", "Order shipped")
    OrderFailed      = capitan.NewSignal("order.failed", "Order failed")
)

// Field keys
var (
    OrderIDKey  = capitan.NewStringKey("order_id")
    CustomerKey = capitan.NewStringKey("customer_id")
    AmountKey   = capitan.NewIntKey("amount")
    ReasonKey   = capitan.NewStringKey("reason")
)

func main() {
    c := capitan.New()
    defer c.Shutdown()

    setupOrderWorkflow(c)

    // Place an order
    c.Emit(context.Background(), OrderPlaced,
        OrderIDKey.Field("ORD-123"),
        CustomerKey.Field("CUST-456"),
        AmountKey.Field(9999), // $99.99
    )

    // Wait for workflow
    time.Sleep(500 * time.Millisecond)
}

func setupOrderWorkflow(c *capitan.Capitan) {
    // Step 1: Check inventory
    c.Hook(OrderPlaced, func(ctx context.Context, e *capitan.Event) {
        orderID := OrderIDKey.Extract(e)
        log.Printf("[1/4] Checking inventory for order %s", orderID)

        if checkInventory(orderID) {
            c.Emit(ctx, InventoryChecked,
                OrderIDKey.Field(orderID),
                CustomerKey.Field(CustomerKey.Extract(e)),
                AmountKey.Field(AmountKey.Extract(e)),
            )
        } else {
            c.EmitWithSeverity(ctx, capitan.SeverityError, OrderFailed,
                OrderIDKey.Field(orderID),
                ReasonKey.Field("insufficient inventory"),
            )
        }
    })

    // Step 2: Process payment
    c.Hook(InventoryChecked, func(ctx context.Context, e *capitan.Event) {
        orderID := OrderIDKey.Extract(e)
        amount := AmountKey.Extract(e)

        log.Printf("[2/4] Processing payment for order %s: $%.2f", orderID, float64(amount)/100)

        if processPayment(orderID, amount) {
            c.Emit(ctx, PaymentProcessed,
                OrderIDKey.Field(orderID),
                CustomerKey.Field(CustomerKey.Extract(e)),
            )
        } else {
            c.EmitWithSeverity(ctx, capitan.SeverityError, OrderFailed,
                OrderIDKey.Field(orderID),
                ReasonKey.Field("payment declined"),
            )
        }
    })

    // Step 3: Ship order
    c.Hook(PaymentProcessed, func(ctx context.Context, e *capitan.Event) {
        orderID := OrderIDKey.Extract(e)
        log.Printf("[3/4] Shipping order %s", orderID)

        shipOrder(orderID)

        c.Emit(ctx, OrderShipped,
            OrderIDKey.Field(orderID),
            CustomerKey.Field(CustomerKey.Extract(e)),
        )
    })

    // Step 4: Notify customer
    c.Hook(OrderShipped, func(ctx context.Context, e *capitan.Event) {
        orderID := OrderIDKey.Extract(e)
        customerID := CustomerKey.Extract(e)

        log.Printf("[4/4] Order %s shipped! Notifying customer %s", orderID, customerID)
        notifyCustomer(customerID, orderID)
    })

    // Error handler
    c.Hook(OrderFailed, func(ctx context.Context, e *capitan.Event) {
        orderID := OrderIDKey.Extract(e)
        reason := ReasonKey.Extract(e)

        log.Printf("❌ Order %s failed: %s", orderID, reason)
        // Refund, notify customer, etc.
    })
}

func checkInventory(orderID string) bool {
    time.Sleep(50 * time.Millisecond) // Simulate API call
    return true
}

func processPayment(orderID string, amount int) bool {
    time.Sleep(100 * time.Millisecond) // Simulate payment gateway
    return true
}

func shipOrder(orderID string) {
    time.Sleep(50 * time.Millisecond) // Simulate shipping API
}

func notifyCustomer(customerID, orderID string) {
    fmt.Printf("📧 Sent shipping notification to %s for order %s\n", customerID, orderID)
}
```

Output:
```
[1/4] Checking inventory for order ORD-123
[2/4] Processing payment for order ORD-123: $99.99
[3/4] Shipping order ORD-123
[4/4] Order ORD-123 shipped! Notifying customer CUST-456
📧 Sent shipping notification to CUST-456 for order ORD-123
```

## Testing Your Events

Capitan includes testing helpers:

```go
package main

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zoobzio/capitan"
    capitantesting "github.com/zoobzio/capitan/testing"
)

func TestOrderWorkflow(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    // Capture events
    shippedCapture := capitantesting.NewEventCapture()
    c.Hook(OrderShipped, shippedCapture.Handler())

    failedCapture := capitantesting.NewEventCapture()
    c.Hook(OrderFailed, failedCapture.Handler())

    // Set up workflow
    setupOrderWorkflow(c)

    // Place order
    c.Emit(context.Background(), OrderPlaced,
        OrderIDKey.Field("TEST-123"),
        CustomerKey.Field("CUST-TEST"),
        AmountKey.Field(5000),
    )

    // Wait for processing
    c.Shutdown()

    // Verify success
    shipped := shippedCapture.Events()
    require.Equal(t, 1, len(shipped))

    // Verify no failures
    failed := failedCapture.Events()
    assert.Equal(t, 0, len(failed))

    // Verify order ID
    orderID := OrderIDKey.ExtractFromFields(shipped[0].Fields)
    assert.Equal(t, "TEST-123", orderID)
}
```

## Best Practices

### 1. Define Signals as Constants

```go
package events

var (
    UserRegistered = capitan.NewSignal("user.registered", "User registered")
    UserVerified   = capitan.NewSignal("user.verified", "User verified")
    UserDeleted    = capitan.NewSignal("user.deleted", "User deleted")
)
```

### 2. Define Field Keys as Constants

```go
package fields

var (
    UserID   = capitan.NewStringKey("user_id")
    Email    = capitan.NewStringKey("email")
    Username = capitan.NewStringKey("username")
)
```

### 3. Use Hierarchical Signal Names

```go
// Good: Clear hierarchy
"order.placed"
"order.payment.processed"
"order.shipped"

// Avoid: Flat naming
"orderplaced"
"payment"
"shipped"
```

### 4. Handle Errors in Listeners

```go
c.Hook(PaymentProcessed, func(ctx context.Context, e *capitan.Event) {
    if err := shipOrder(orderID); err != nil {
        log.Printf("Shipping failed: %v", err)

        c.EmitWithSeverity(ctx, capitan.SeverityError, OrderFailed,
            OrderIDKey.Field(orderID),
            ReasonKey.Field(err.Error()),
        )
    }
})
```

### 5. Close Listeners on Shutdown

```go
type Service struct {
    listeners []*capitan.Listener
}

func (s *Service) Start(c *capitan.Capitan) {
    s.listeners = append(s.listeners,
        c.Hook(signal1, s.handler1),
        c.Hook(signal2, s.handler2),
    )
}

func (s *Service) Stop() {
    for _, l := range s.listeners {
        l.Close()
    }
}
```

## Common Patterns

### Multi-Tenant Event Routing

```go
tenantKey := capitan.NewStringKey("tenant_id")

c.Hook(DataUpdated, func(ctx context.Context, e *capitan.Event) {
    tenant := tenantKey.Extract(e)

    // Route to tenant-specific handler
    tenantCache[tenant].Invalidate()
})
```

### Retry Pattern

```go
c.Hook(APICallFailed, func(ctx context.Context, e *capitan.Event) {
    retries := retriesKey.Extract(e)

    if retries < 3 {
        // Retry
        c.Emit(ctx, APICallRequested,
            requestKey.Field(RequestKey.Extract(e)),
            retriesKey.Field(retries+1),
        )
    } else {
        // Give up
        c.Emit(ctx, APICallAbandoned,
            requestKey.Field(RequestKey.Extract(e)),
        )
    }
})
```

### Saga Pattern

```go
// Compensation events
var (
    OrderCancelled       = capitan.NewSignal("order.cancelled", "Order cancelled")
    InventoryRestored    = capitan.NewSignal("inventory.restored", "Inventory restored")
    PaymentRefunded      = capitan.NewSignal("payment.refunded", "Payment refunded")
)

c.Hook(OrderCancelled, func(ctx context.Context, e *capitan.Event) {
    // Restore inventory
    c.Emit(ctx, InventoryRestored, OrderIDKey.Field(OrderIDKey.Extract(e)))
})

c.Hook(InventoryRestored, func(ctx context.Context, e *capitan.Event) {
    // Refund payment
    c.Emit(ctx, PaymentRefunded, OrderIDKey.Field(OrderIDKey.Extract(e)))
})
```

## Next Steps

- [First Event System](./first-event-system.md) - Build a complete real-world system
- [Core Concepts](../learn/core-concepts.md) - Deep dive into signals and observers
- [Testing Guide](../guides/testing.md) - Comprehensive testing patterns
- [Event Patterns](../guides/patterns.md) - Common event coordination patterns
- [Cookbook](../cookbook/README.md) - Copy-paste recipes for common scenarios
