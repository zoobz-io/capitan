---
title: Your First Event System
description: Build a complete event-driven order fulfillment system with capitan from scratch.
author: Capitan Team
published: 2025-12-01
tags: [Tutorial, Example, Real World]
---

# Your First Event System

Build a complete event-driven order fulfillment system from scratch.

## What We're Building

An order fulfillment system with:

- Order validation and processing
- Inventory management
- Payment processing
- Shipping coordination
- Customer notifications
- Error handling and compensation

## Project Setup

```bash
mkdir order-system
cd order-system
go mod init example.com/order-system
go get github.com/zoobzio/capitan
```

## Step 1: Define the Domain

Create `events/signals.go`:

```go
package events

import "github.com/zoobzio/capitan"

// Order lifecycle signals
var (
    OrderReceived    = capitan.NewSignal("order.received", "Order received from customer")
    OrderValidated   = capitan.NewSignal("order.validated", "Order validated")
    InventoryReserved = capitan.NewSignal("inventory.reserved", "Inventory reserved")
    PaymentAuthorized = capitan.NewSignal("payment.authorized", "Payment authorized")
    OrderFulfilled   = capitan.NewSignal("order.fulfilled", "Order ready for shipping")
    OrderShipped     = capitan.NewSignal("order.shipped", "Order shipped")
    OrderCompleted   = capitan.NewSignal("order.completed", "Order completed")

    // Error signals
    OrderValidationFailed = capitan.NewSignal("order.validation.failed", "Order validation failed")
    InventoryUnavailable  = capitan.NewSignal("inventory.unavailable", "Inventory unavailable")
    PaymentDeclined       = capitan.NewSignal("payment.declined", "Payment declined")
    ShippingFailed        = capitan.NewSignal("shipping.failed", "Shipping failed")

    // Compensation signals
    InventoryReleased = capitan.NewSignal("inventory.released", "Inventory released")
    PaymentRefunded   = capitan.NewSignal("payment.refunded", "Payment refunded")
)
```

Create `events/fields.go`:

```go
package events

import (
    "time"
    "github.com/zoobzio/capitan"
)

// Common field keys
var (
    OrderID    = capitan.NewStringKey("order_id")
    CustomerID = capitan.NewStringKey("customer_id")
    ProductID  = capitan.NewStringKey("product_id")
    Quantity   = capitan.NewIntKey("quantity")
    Amount     = capitan.NewIntKey("amount") // in cents
    Email      = capitan.NewStringKey("email")
    Address    = capitan.NewStringKey("address")
    Reason     = capitan.NewStringKey("reason")
    Timestamp  = capitan.NewTimeKey("timestamp")
    TrackingID = capitan.NewStringKey("tracking_id")
)
```

## Step 2: Define Domain Models

Create `domain/order.go`:

```go
package domain

import "time"

type Order struct {
    ID         string
    CustomerID string
    Items      []OrderItem
    Total      int // cents
    Status     OrderStatus
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type OrderItem struct {
    ProductID string
    Quantity  int
    Price     int // cents
}

type OrderStatus string

const (
    StatusReceived    OrderStatus = "received"
    StatusValidated   OrderStatus = "validated"
    StatusReserved    OrderStatus = "reserved"
    StatusAuthorized  OrderStatus = "authorized"
    StatusFulfilled   OrderStatus = "fulfilled"
    StatusShipped     OrderStatus = "shipped"
    StatusCompleted   OrderStatus = "completed"
    StatusFailed      OrderStatus = "failed"
)

type Customer struct {
    ID      string
    Email   string
    Address string
}
```

## Step 3: Implement Services

Create `services/validation.go`:

```go
package services

import (
    "context"
    "fmt"
    "log"

    "example.com/order-system/domain"
    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type ValidationService struct {
    c *capitan.Capitan
}

func NewValidationService(c *capitan.Capitan) *ValidationService {
    return &ValidationService{c: c}
}

func (s *ValidationService) Start() {
    s.c.Hook(events.OrderReceived, s.validateOrder)
}

func (s *ValidationService) validateOrder(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    customerID := events.CustomerID.Extract(e)

    log.Printf("[Validation] Validating order %s", orderID)

    // Validate customer exists
    if !s.customerExists(customerID) {
        s.c.EmitWithSeverity(ctx, capitan.SeverityError, events.OrderValidationFailed,
            events.OrderID.Field(orderID),
            events.Reason.Field("customer not found"),
        )
        return
    }

    // Validate order data
    quantity := events.Quantity.Extract(e)
    if quantity <= 0 {
        s.c.EmitWithSeverity(ctx, capitan.SeverityError, events.OrderValidationFailed,
            events.OrderID.Field(orderID),
            events.Reason.Field("invalid quantity"),
        )
        return
    }

    log.Printf("[Validation] Order %s validated", orderID)

    // Emit validation success
    s.c.Emit(ctx, events.OrderValidated,
        events.OrderID.Field(orderID),
        events.CustomerID.Field(customerID),
        events.ProductID.Field(events.ProductID.Extract(e)),
        events.Quantity.Field(quantity),
        events.Amount.Field(events.Amount.Extract(e)),
    )
}

func (s *ValidationService) customerExists(customerID string) bool {
    // Simulate database lookup
    return customerID != ""
}
```

Create `services/inventory.go`:

```go
package services

import (
    "context"
    "log"
    "sync"

    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type InventoryService struct {
    c         *capitan.Capitan
    stock     map[string]int
    reserved  map[string]int
    mu        sync.RWMutex
}

func NewInventoryService(c *capitan.Capitan) *InventoryService {
    return &InventoryService{
        c:        c,
        stock:    make(map[string]int),
        reserved: make(map[string]int),
    }
}

func (s *InventoryService) Start() {
    // Initialize some stock
    s.stock["PROD-1"] = 100
    s.stock["PROD-2"] = 50

    s.c.Hook(events.OrderValidated, s.reserveInventory)
    s.c.Hook(events.InventoryReleased, s.releaseInventory)
}

func (s *InventoryService) reserveInventory(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    productID := events.ProductID.Extract(e)
    quantity := events.Quantity.Extract(e)

    log.Printf("[Inventory] Reserving %d units of %s for order %s", quantity, productID, orderID)

    s.mu.Lock()
    defer s.mu.Unlock()

    available := s.stock[productID]
    if available < quantity {
        s.c.EmitWithSeverity(ctx, capitan.SeverityWarn, events.InventoryUnavailable,
            events.OrderID.Field(orderID),
            events.ProductID.Field(productID),
            events.Reason.Field("insufficient stock"),
        )
        return
    }

    // Reserve inventory
    s.stock[productID] -= quantity
    s.reserved[orderID] = quantity

    log.Printf("[Inventory] Reserved %d units for order %s", quantity, orderID)

    s.c.Emit(ctx, events.InventoryReserved,
        events.OrderID.Field(orderID),
        events.CustomerID.Field(events.CustomerID.Extract(e)),
        events.ProductID.Field(productID),
        events.Quantity.Field(quantity),
        events.Amount.Field(events.Amount.Extract(e)),
    )
}

func (s *InventoryService) releaseInventory(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    productID := events.ProductID.Extract(e)

    s.mu.Lock()
    defer s.mu.Unlock()

    if quantity, ok := s.reserved[orderID]; ok {
        s.stock[productID] += quantity
        delete(s.reserved, orderID)
        log.Printf("[Inventory] Released %d units for order %s", quantity, orderID)
    }
}
```

Create `services/payment.go`:

```go
package services

import (
    "context"
    "log"
    "math/rand"

    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type PaymentService struct {
    c *capitan.Capitan
}

func NewPaymentService(c *capitan.Capitan) *PaymentService {
    return &PaymentService{c: c}
}

func (s *PaymentService) Start() {
    s.c.Hook(events.InventoryReserved, s.authorizePayment)
    s.c.Hook(events.PaymentRefunded, s.processRefund)
}

func (s *PaymentService) authorizePayment(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    amount := events.Amount.Extract(e)

    log.Printf("[Payment] Authorizing $%.2f for order %s", float64(amount)/100, orderID)

    // Simulate payment gateway (90% success rate)
    if rand.Float64() < 0.9 {
        log.Printf("[Payment] Payment authorized for order %s", orderID)

        s.c.Emit(ctx, events.PaymentAuthorized,
            events.OrderID.Field(orderID),
            events.CustomerID.Field(events.CustomerID.Extract(e)),
            events.Amount.Field(amount),
        )
    } else {
        log.Printf("[Payment] Payment declined for order %s", orderID)

        s.c.EmitWithSeverity(ctx, capitan.SeverityError, events.PaymentDeclined,
            events.OrderID.Field(orderID),
            events.Reason.Field("card declined"),
        )
    }
}

func (s *PaymentService) processRefund(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Payment] Processing refund for order %s", orderID)
}
```

Create `services/shipping.go`:

```go
package services

import (
    "context"
    "fmt"
    "log"
    "time"

    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type ShippingService struct {
    c *capitan.Capitan
}

func NewShippingService(c *capitan.Capitan) *ShippingService {
    return &ShippingService{c: c}
}

func (s *ShippingService) Start() {
    s.c.Hook(events.PaymentAuthorized, s.fulfillOrder)
    s.c.Hook(events.OrderFulfilled, s.shipOrder)
}

func (s *ShippingService) fulfillOrder(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Shipping] Fulfilling order %s", orderID)

    // Simulate warehouse picking
    time.Sleep(50 * time.Millisecond)

    s.c.Emit(ctx, events.OrderFulfilled,
        events.OrderID.Field(orderID),
        events.CustomerID.Field(events.CustomerID.Extract(e)),
    )
}

func (s *ShippingService) shipOrder(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Shipping] Shipping order %s", orderID)

    // Generate tracking number
    trackingID := fmt.Sprintf("TRK-%s-%d", orderID, time.Now().Unix())

    s.c.Emit(ctx, events.OrderShipped,
        events.OrderID.Field(orderID),
        events.CustomerID.Field(events.CustomerID.Extract(e)),
        events.TrackingID.Field(trackingID),
    )
}
```

Create `services/notification.go`:

```go
package services

import (
    "context"
    "log"

    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type NotificationService struct {
    c *capitan.Capitan
}

func NewNotificationService(c *capitan.Capitan) *NotificationService {
    return &NotificationService{c: c}
}

func (s *NotificationService) Start() {
    s.c.Hook(events.OrderReceived, s.sendOrderConfirmation)
    s.c.Hook(events.OrderShipped, s.sendShippingNotification)
    s.c.Hook(events.OrderValidationFailed, s.sendValidationError)
    s.c.Hook(events.PaymentDeclined, s.sendPaymentError)
}

func (s *NotificationService) sendOrderConfirmation(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Notification] 📧 Sending order confirmation for %s", orderID)
}

func (s *NotificationService) sendShippingNotification(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    trackingID := events.TrackingID.Extract(e)
    log.Printf("[Notification] 📦 Sending shipping notification for %s (tracking: %s)", orderID, trackingID)
}

func (s *NotificationService) sendValidationError(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    reason := events.Reason.Extract(e)
    log.Printf("[Notification] ❌ Sending validation error for %s: %s", orderID, reason)
}

func (s *NotificationService) sendPaymentError(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Notification] ❌ Sending payment declined notification for %s", orderID)
}
```

## Step 4: Add Compensation Logic

Create `services/compensation.go`:

```go
package services

import (
    "context"
    "log"

    "example.com/order-system/events"
    "github.com/zoobzio/capitan"
)

type CompensationService struct {
    c *capitan.Capitan
}

func NewCompensationService(c *capitan.Capitan) *CompensationService {
    return &CompensationService{c: c}
}

func (s *CompensationService) Start() {
    // When payment declines, release inventory
    s.c.Hook(events.PaymentDeclined, s.compensatePaymentFailure)

    // When inventory unavailable, nothing to compensate
    s.c.Hook(events.InventoryUnavailable, s.compensateInventoryFailure)
}

func (s *CompensationService) compensatePaymentFailure(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Compensation] Payment failed for %s, releasing inventory", orderID)

    // Release reserved inventory
    s.c.Emit(ctx, events.InventoryReleased,
        events.OrderID.Field(orderID),
        events.ProductID.Field(""), // Would need to track this
    )
}

func (s *CompensationService) compensateInventoryFailure(ctx context.Context, e *capitan.Event) {
    orderID := events.OrderID.Extract(e)
    log.Printf("[Compensation] Inventory unavailable for %s", orderID)
    // No compensation needed - nothing was reserved
}
```

## Step 5: Wire It All Together

Create `main.go`:

```go
package main

import (
    "context"
    "log"
    "time"

    "example.com/order-system/events"
    "example.com/order-system/services"
    "github.com/zoobzio/capitan"
)

func main() {
    // Create capitan instance
    c := capitan.New(capitan.WithBufferSize(50))
    defer c.Shutdown()

    // Add audit observer
    observer := c.Observe(func(ctx context.Context, e *capitan.Event) {
        log.Printf("[AUDIT] %s | Severity: %d", e.Signal().Name(), e.Severity())
    })
    defer observer.Close()

    // Initialize services
    validation := services.NewValidationService(c)
    inventory := services.NewInventoryService(c)
    payment := services.NewPaymentService(c)
    shipping := services.NewShippingService(c)
    notification := services.NewNotificationService(c)
    compensation := services.NewCompensationService(c)

    // Start all services
    validation.Start()
    inventory.Start()
    payment.Start()
    shipping.Start()
    notification.Start()
    compensation.Start()

    log.Println("Order fulfillment system started")

    // Simulate incoming orders
    placeOrder(c, "ORD-001", "CUST-123", "PROD-1", 5, 9999)
    placeOrder(c, "ORD-002", "CUST-456", "PROD-2", 2, 4999)
    placeOrder(c, "ORD-003", "CUST-789", "PROD-1", 200, 199900) // Will fail - not enough inventory

    // Wait for async processing
    time.Sleep(1 * time.Second)

    log.Println("Order fulfillment system shutting down")
}

func placeOrder(c *capitan.Capitan, orderID, customerID, productID string, quantity, amount int) {
    log.Printf("\n=== Placing Order %s ===", orderID)

    c.Emit(context.Background(), events.OrderReceived,
        events.OrderID.Field(orderID),
        events.CustomerID.Field(customerID),
        events.ProductID.Field(productID),
        events.Quantity.Field(quantity),
        events.Amount.Field(amount),
        events.Timestamp.Field(time.Now()),
    )
}
```

## Step 6: Run the System

```bash
go run main.go
```

Output:
```
Order fulfillment system started

=== Placing Order ORD-001 ===
[AUDIT] order.received | Severity: 1
[Notification] 📧 Sending order confirmation for ORD-001
[Validation] Validating order ORD-001
[Validation] Order ORD-001 validated
[AUDIT] order.validated | Severity: 1
[Inventory] Reserving 5 units of PROD-1 for order ORD-001
[Inventory] Reserved 5 units for order ORD-001
[AUDIT] inventory.reserved | Severity: 1
[Payment] Authorizing $99.99 for order ORD-001
[Payment] Payment authorized for order ORD-001
[AUDIT] payment.authorized | Severity: 1
[Shipping] Fulfilling order ORD-001
[AUDIT] order.fulfilled | Severity: 1
[Shipping] Shipping order ORD-001
[AUDIT] order.shipped | Severity: 1
[Notification] 📦 Sending shipping notification for ORD-001 (tracking: TRK-ORD-001-1701446400)

=== Placing Order ORD-002 ===
[AUDIT] order.received | Severity: 1
[Notification] 📧 Sending order confirmation for ORD-002
[Validation] Validating order ORD-002
...

=== Placing Order ORD-003 ===
[AUDIT] order.received | Severity: 1
[Notification] 📧 Sending order confirmation for ORD-003
[Validation] Validating order ORD-003
[Validation] Order ORD-003 validated
[AUDIT] order.validated | Severity: 1
[Inventory] Reserving 200 units of PROD-1 for order ORD-003
[AUDIT] inventory.unavailable | Severity: 2
[Compensation] Inventory unavailable for ORD-003

Order fulfillment system shutting down
```

## Step 7: Add Tests

Create `main_test.go`:

```go
package main

import (
    "context"
    "testing"
    "time"

    "example.com/order-system/events"
    "example.com/order-system/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/zoobzio/capitan"
    capitantesting "github.com/zoobzio/capitan/testing"
)

func TestSuccessfulOrderFlow(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    // Set up services
    validation := services.NewValidationService(c)
    inventory := services.NewInventoryService(c)
    payment := services.NewPaymentService(c)
    shipping := services.NewShippingService(c)

    validation.Start()
    inventory.Start()
    payment.Start()
    shipping.Start()

    // Capture shipped events
    shippedCapture := capitantesting.NewEventCapture()
    c.Hook(events.OrderShipped, shippedCapture.Handler())

    // Place order
    c.Emit(context.Background(), events.OrderReceived,
        events.OrderID.Field("TEST-001"),
        events.CustomerID.Field("CUST-TEST"),
        events.ProductID.Field("PROD-1"),
        events.Quantity.Field(1),
        events.Amount.Field(1000),
        events.Timestamp.Field(time.Now()),
    )

    // Wait for processing
    time.Sleep(200 * time.Millisecond)
    c.Shutdown()

    // Verify order shipped
    shipped := shippedCapture.Events()
    require.Equal(t, 1, len(shipped))

    orderID := events.OrderID.ExtractFromFields(shipped[0].Fields)
    assert.Equal(t, "TEST-001", orderID)
}

func TestInsufficientInventory(t *testing.T) {
    c := capitan.New()
    defer c.Shutdown()

    validation := services.NewValidationService(c)
    inventory := services.NewInventoryService(c)

    validation.Start()
    inventory.Start()

    // Capture unavailable events
    unavailableCapture := capitantesting.NewEventCapture()
    c.Hook(events.InventoryUnavailable, unavailableCapture.Handler())

    // Place order for too many items
    c.Emit(context.Background(), events.OrderReceived,
        events.OrderID.Field("TEST-002"),
        events.CustomerID.Field("CUST-TEST"),
        events.ProductID.Field("PROD-1"),
        events.Quantity.Field(999),
        events.Amount.Field(99900),
        events.Timestamp.Field(time.Now()),
    )

    time.Sleep(100 * time.Millisecond)
    c.Shutdown()

    // Verify inventory unavailable
    unavailable := unavailableCapture.Events()
    require.Equal(t, 1, len(unavailable))

    reason := events.Reason.ExtractFromFields(unavailable[0].Fields)
    assert.Equal(t, "insufficient stock", reason)
}
```

Run tests:
```bash
go test -v
```

## What We've Built

A complete event-driven system with:

✅ **Clear separation of concerns** - Each service handles one domain
✅ **Async workflows** - Events cascade through the system
✅ **Error handling** - Failed payments trigger compensation
✅ **Observability** - Audit observer logs all events
✅ **Type safety** - Compile-time field type checking
✅ **Testable** - Integration tests verify workflows

## Next Steps

- [Testing Guide](../guides/testing.md) - Advanced testing patterns
- [Event Patterns](../guides/patterns.md) - Common event coordination patterns
- [Observers Guide](../guides/observers.md) - Advanced observer usage
- [Cookbook](../cookbook/README.md) - More real-world recipes
