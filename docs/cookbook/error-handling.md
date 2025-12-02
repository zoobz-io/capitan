---
title: Error Handling
description: Retry, compensation, and error recovery patterns for resilient event systems.
author: Capitan Team
published: 2025-12-01
tags: [Cookbook, Error Handling, Retry, Compensation]
---

# Error Handling

> **Pattern Illustration**: This recipe demonstrates error handling patterns.
> Define the signals, fields, and business logic for your specific domain.

Retry, compensation, and error recovery patterns.

## Quick Start

Get a retry handler running in 30 seconds:

```go
package main

import (
    "context"
    "errors"
    "github.com/zoobzio/capitan"
)

// 1. Define your signals
var (
    TaskRequested = capitan.NewSignal("task.requested", "Task requested")
    TaskFailed    = capitan.NewSignal("task.failed", "Task failed")
    TaskSucceeded = capitan.NewSignal("task.succeeded", "Task succeeded")
)

// 2. Define your fields
var (
    TaskID       = capitan.NewStringKey("task_id")
    ErrorMessage = capitan.NewStringKey("error_message")
)

// 3. Implement your actual task logic
func executeTask(taskID string) error {
    // TODO: Your actual task logic here
    return errors.New("not implemented")
}

// 4. Hook and emit
func main() {
    c := capitan.New()
    defer c.Shutdown()

    c.Hook(TaskRequested, func(ctx context.Context, e *capitan.Event) {
        taskID := TaskID.Extract(e)

        if err := executeTask(taskID); err != nil {
            c.Emit(ctx, TaskFailed,
                TaskID.Field(taskID),
                ErrorMessage.Field(err.Error()),
            )
        } else {
            c.Emit(ctx, TaskSucceeded, TaskID.Field(taskID))
        }
    })

    // Trigger a task
    c.Emit(context.Background(), TaskRequested, TaskID.Field("task-1"))
}
```

Now see the full retry pattern with backoff strategies below...

## Problem

Distributed systems experience transient failures. You need:
- Automatic retry for transient errors
- Compensation for failed transactions
- Circuit breakers to prevent cascading failures
- Dead letter queues for unrecoverable errors

## Solution: Retry Pattern

```go
package retry

import (
    "context"
    "errors"
    "log"
    "sync"
    "time"

    "github.com/zoobzio/capitan"
)

// Signals used in this recipe
var (
    APICallRequested = capitan.NewSignal("api.call.requested", "API call requested")
    APICallFailed    = capitan.NewSignal("api.call.failed", "API call failed")
    APICallSucceeded = capitan.NewSignal("api.call.succeeded", "API call succeeded")
    APICallAbandoned = capitan.NewSignal("api.call.abandoned", "API call abandoned after max retries")
)

// Field keys used in this recipe
var (
    RequestID    = capitan.NewStringKey("request_id")
    ErrorMessage = capitan.NewStringKey("error_message")
    RetryCount   = capitan.NewIntKey("retry_count")
)

// Business logic stub (implement for your domain)
func makeAPICall(requestID string) error {
    // TODO: Implement your actual API call logic
    // Example: return yourAPIClient.Call(requestID)
    return errors.New("not implemented")
}

type RetryHandler struct {
    c          *capitan.Capitan
    maxRetries int
    backoff    BackoffStrategy
    attempts   map[string]*AttemptTracker
    mu         sync.Mutex
}

type AttemptTracker struct {
    Count       int
    FirstAttempt time.Time
    LastAttempt  time.Time
}

type BackoffStrategy func(attempt int) time.Duration

// Exponential backoff: 1s, 2s, 4s, 8s, ...
func ExponentialBackoff(attempt int) time.Duration {
    return time.Duration(1<<uint(attempt)) * time.Second
}

// Linear backoff: 1s, 2s, 3s, 4s, ...
func LinearBackoff(attempt int) time.Duration {
    return time.Duration(attempt+1) * time.Second
}

// Constant backoff: 5s, 5s, 5s, ...
func ConstantBackoff(delay time.Duration) BackoffStrategy {
    return func(attempt int) time.Duration {
        return delay
    }
}

func NewRetryHandler(c *capitan.Capitan, maxRetries int, backoff BackoffStrategy) *RetryHandler {
    return &RetryHandler{
        c:          c,
        maxRetries: maxRetries,
        backoff:    backoff,
        attempts:   make(map[string]*AttemptTracker),
    }
}

func (rh *RetryHandler) Start() {
    // Hook into operation signals
    rh.c.Hook(APICallRequested, rh.handleAPICall)
    rh.c.Hook(APICallFailed, rh.handleAPICallFailed)
    rh.c.Hook(APICallSucceeded, rh.handleAPICallSucceeded)
}

func (rh *RetryHandler) handleAPICall(ctx context.Context, e *capitan.Event) {
    requestID := RequestID.Extract(e)

    log.Printf("[Retry] Attempting API call: %s", requestID)

    // Make API call
    if err := makeAPICall(requestID); err != nil {
        rh.c.EmitWithSeverity(ctx, capitan.SeverityWarn, APICallFailed,
            RequestID.Field(requestID),
            ErrorMessage.Field(err.Error()),
        )
    } else {
        rh.c.Emit(ctx, APICallSucceeded,
            RequestID.Field(requestID),
        )
    }
}

func (rh *RetryHandler) handleAPICallFailed(ctx context.Context, e *capitan.Event) {
    requestID := RequestID.Extract(e)
    errorMsg := ErrorMessage.Extract(e)

    rh.mu.Lock()
    tracker, exists := rh.attempts[requestID]
    if !exists {
        tracker = &AttemptTracker{
            FirstAttempt: time.Now(),
        }
        rh.attempts[requestID] = tracker
    }

    tracker.Count++
    tracker.LastAttempt = time.Now()
    attempt := tracker.Count
    rh.mu.Unlock()

    if attempt < rh.maxRetries {
        backoff := rh.backoff(attempt)

        log.Printf("[Retry] Attempt %d/%d failed for %s: %s. Retrying in %v",
            attempt, rh.maxRetries, requestID, errorMsg, backoff)

        // Wait before retrying
        time.Sleep(backoff)

        // Retry
        rh.c.Emit(ctx, APICallRequested,
            RequestID.Field(requestID),
        )
    } else {
        log.Printf("[Retry] Max retries (%d) reached for %s. Giving up.", rh.maxRetries, requestID)

        // Move to dead letter queue
        rh.c.EmitWithSeverity(ctx, capitan.SeverityError, APICallAbandoned,
            RequestID.Field(requestID),
            ErrorMessage.Field(errorMsg),
            RetryCount.Field(attempt),
        )

        // Cleanup
        rh.mu.Lock()
        delete(rh.attempts, requestID)
        rh.mu.Unlock()
    }
}

func (rh *RetryHandler) handleAPICallSucceeded(ctx context.Context, e *capitan.Event) {
    requestID := RequestID.Extract(e)

    rh.mu.Lock()
    tracker := rh.attempts[requestID]
    delete(rh.attempts, requestID)
    rh.mu.Unlock()

    if tracker != nil {
        duration := time.Since(tracker.FirstAttempt)
        log.Printf("[Retry] API call succeeded for %s after %d attempts in %v",
            requestID, tracker.Count, duration)
    } else {
        log.Printf("[Retry] API call succeeded for %s on first attempt", requestID)
    }
}
```

## Compensation Pattern (Saga)

```go
package compensation

import (
    "context"
    "log"

    "github.com/zoobzio/capitan"
)

// Signals for compensation pattern
var (
    OrderPlaced              = capitan.NewSignal("order.placed", "Order placed")
    InventoryReserved        = capitan.NewSignal("inventory.reserved", "Inventory reserved")
    InventoryUnavailable     = capitan.NewSignal("inventory.unavailable", "Inventory unavailable")
    PaymentProcessed         = capitan.NewSignal("payment.processed", "Payment processed")
    PaymentFailed            = capitan.NewSignal("payment.failed", "Payment failed")
    PaymentRefunded          = capitan.NewSignal("payment.refunded", "Payment refunded")
    OrderCompleted           = capitan.NewSignal("order.completed", "Order completed")
    OrderFailed              = capitan.NewSignal("order.failed", "Order failed")
    OrderCancelled           = capitan.NewSignal("order.cancelled", "Order cancelled")
    OrderCancellationComplete = capitan.NewSignal("order.cancellation.complete", "Order cancellation complete")
    InventoryReleased        = capitan.NewSignal("inventory.released", "Inventory released")
)

// Field keys for compensation pattern
var (
    OrderID    = capitan.NewStringKey("order_id")
    CustomerID = capitan.NewStringKey("customer_id")
    Amount     = capitan.NewIntKey("amount")
    Reason     = capitan.NewStringKey("reason")
    OrderState = capitan.NewStringKey("order_state")
)

// Business logic stubs (implement for your domain)
func reserveInventory(orderID string) bool {
    // TODO: Implement inventory reservation logic
    return true
}

func processPayment(orderID string, amount int) bool {
    // TODO: Implement payment processing logic
    return true
}

func releaseInventory(orderID string) {
    // TODO: Implement inventory release logic
}

func refundPayment(orderID string) {
    // TODO: Implement refund logic
}

type CompensationHandler struct {
    c *capitan.Capitan
}

func NewCompensationHandler(c *capitan.Capitan) *CompensationHandler {
    return &CompensationHandler{c: c}
}

func (ch *CompensationHandler) Start() {
    // Forward flow
    ch.c.Hook(OrderPlaced, ch.handleOrderPlaced)
    ch.c.Hook(InventoryReserved, ch.handleInventoryReserved)
    ch.c.Hook(PaymentProcessed, ch.handlePaymentProcessed)

    // Compensation flow
    ch.c.Hook(PaymentFailed, ch.compensatePaymentFailure)
    ch.c.Hook(OrderCancelled, ch.compensateOrderCancellation)
}

// Forward: Order → Inventory
func (ch *CompensationHandler) handleOrderPlaced(ctx context.Context, e *capitan.Event) {
    orderID := OrderID.Extract(e)

    log.Printf("[Saga] Step 1: Reserving inventory for order %s", orderID)

    if reserveInventory(orderID) {
        ch.c.Emit(ctx, InventoryReserved,
            OrderID.Field(orderID),
            CustomerID.Field(CustomerID.Extract(e)),
            Amount.Field(Amount.Extract(e)),
        )
    } else {
        ch.c.Emit(ctx, InventoryUnavailable,
            OrderID.Field(orderID),
        )
    }
}

// Forward: Inventory → Payment
func (ch *CompensationHandler) handleInventoryReserved(ctx context.Context, e *capitan.Event) {
    orderID := OrderID.Extract(e)
    amount := Amount.Extract(e)

    log.Printf("[Saga] Step 2: Processing payment for order %s", orderID)

    if processPayment(orderID, amount) {
        ch.c.Emit(ctx, PaymentProcessed,
            OrderID.Field(orderID),
            Amount.Field(amount),
        )
    } else {
        ch.c.EmitWithSeverity(ctx, capitan.SeverityError, PaymentFailed,
            OrderID.Field(orderID),
            Reason.Field("payment declined"),
        )
    }
}

// Forward: Payment → Complete
func (ch *CompensationHandler) handlePaymentProcessed(ctx context.Context, e *capitan.Event) {
    orderID := OrderID.Extract(e)

    log.Printf("[Saga] Step 3: Order %s completed", orderID)

    ch.c.Emit(ctx, OrderCompleted,
        OrderID.Field(orderID),
    )
}

// Compensation: Payment Failed → Release Inventory
func (ch *CompensationHandler) compensatePaymentFailure(ctx context.Context, e *capitan.Event) {
    orderID := OrderID.Extract(e)

    log.Printf("[Saga] Compensating payment failure for order %s", orderID)

    // Step 1: Release inventory
    releaseInventory(orderID)
    ch.c.Emit(ctx, InventoryReleased,
        OrderID.Field(orderID),
    )

    // Step 2: Mark order as failed
    ch.c.Emit(ctx, OrderFailed,
        OrderID.Field(orderID),
        Reason.Field("payment failed"),
    )
}

// Compensation: Order Cancelled → Rollback Everything
func (ch *CompensationHandler) compensateOrderCancellation(ctx context.Context, e *capitan.Event) {
    orderID := OrderID.Extract(e)
    state := OrderState.Extract(e)

    log.Printf("[Saga] Compensating cancelled order %s in state %s", orderID, state)

    switch state {
    case "payment_processed":
        // Refund payment
        refundPayment(orderID)
        ch.c.Emit(ctx, PaymentRefunded, OrderID.Field(orderID))
        fallthrough

    case "inventory_reserved":
        // Release inventory
        releaseInventory(orderID)
        ch.c.Emit(ctx, InventoryReleased, OrderID.Field(orderID))

    case "placed":
        // Nothing to compensate
    }

    ch.c.Emit(ctx, OrderCancellationComplete,
        OrderID.Field(orderID),
    )
}
```

## Circuit Breaker Pattern

```go
package circuitbreaker

import (
    "context"
    "errors"
    "log"
    "sync"
    "time"

    "github.com/zoobzio/capitan"
)

// Signals for circuit breaker pattern
var (
    ServiceCallRequested     = capitan.NewSignal("service.call.requested", "Service call requested")
    ServiceCallFailed        = capitan.NewSignal("service.call.failed", "Service call failed")
    ServiceCallSucceeded     = capitan.NewSignal("service.call.succeeded", "Service call succeeded")
    ServiceCallRejected      = capitan.NewSignal("service.call.rejected", "Service call rejected")
    CircuitBreakerOpened     = capitan.NewSignal("circuit.breaker.opened", "Circuit breaker opened")
    CircuitBreakerClosed     = capitan.NewSignal("circuit.breaker.closed", "Circuit breaker closed")
)

// Field keys for circuit breaker pattern
var (
    RequestID    = capitan.NewStringKey("request_id")
    ErrorMessage = capitan.NewStringKey("error_message")
    Reason       = capitan.NewStringKey("reason")
    FailureCount = capitan.NewIntKey("failure_count")
)

// Business logic stub (implement for your domain)
func callService(requestID string) error {
    // TODO: Implement your service call logic
    return errors.New("not implemented")
}

type CircuitBreaker struct {
    c           *capitan.Capitan
    threshold   int
    timeout     time.Duration
    failures    int
    state       string // "closed", "open", "half-open"
    lastFailure time.Time
    mu          sync.Mutex
}

func NewCircuitBreaker(c *capitan.Capitan, threshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        c:         c,
        threshold: threshold,
        timeout:   timeout,
        state:     "closed",
    }
}

func (cb *CircuitBreaker) Start() {
    cb.c.Hook(ServiceCallRequested, cb.handleRequest)
    cb.c.Hook(ServiceCallFailed, cb.recordFailure)
    cb.c.Hook(ServiceCallSucceeded, cb.recordSuccess)
}

func (cb *CircuitBreaker) handleRequest(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()

    // Transition to half-open if timeout elapsed
    if cb.state == "open" && time.Since(cb.lastFailure) > cb.timeout {
        cb.state = "half-open"
        log.Printf("[CircuitBreaker] Transitioning to half-open, allowing test request")
    }

    // Reject if open
    if cb.state == "open" {
        cb.mu.Unlock()

        log.Printf("[CircuitBreaker] Circuit is open, rejecting request")

        cb.c.EmitWithSeverity(ctx, capitan.SeverityWarn, ServiceCallRejected,
            Reason.Field("circuit breaker open"),
        )
        return
    }

    cb.mu.Unlock()

    // Allow request
    requestID := RequestID.Extract(e)

    if err := callService(requestID); err != nil {
        cb.c.EmitWithSeverity(ctx, capitan.SeverityError, ServiceCallFailed,
            RequestID.Field(requestID),
            ErrorMessage.Field(err.Error()),
        )
    } else {
        cb.c.Emit(ctx, ServiceCallSucceeded,
            RequestID.Field(requestID),
        )
    }
}

func (cb *CircuitBreaker) recordFailure(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures++
    cb.lastFailure = time.Now()

    if cb.failures >= cb.threshold {
        cb.state = "open"
        log.Printf("[CircuitBreaker] Circuit opened after %d failures", cb.failures)

        cb.c.EmitWithSeverity(ctx, capitan.SeverityCritical, CircuitBreakerOpened,
            FailureCount.Field(cb.failures),
        )
    }
}

func (cb *CircuitBreaker) recordSuccess(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == "half-open" {
        // Successful test request - close circuit
        cb.state = "closed"
        cb.failures = 0

        log.Printf("[CircuitBreaker] Circuit closed after successful test request")

        cb.c.Emit(ctx, CircuitBreakerClosed)
    } else if cb.state == "closed" {
        // Reset failure count on success
        cb.failures = 0
    }
}
```

## Dead Letter Queue

```go
package dlq

import (
    "context"
    "encoding/json"
    "log"
    "os"

    "github.com/zoobzio/capitan"
)

// Signals for DLQ pattern (reuse from retry/compensation)
var (
    APICallAbandoned = capitan.NewSignal("api.call.abandoned", "API call abandoned after max retries")
    OrderFailed      = capitan.NewSignal("order.failed", "Order failed")
)

// Field keys for DLQ pattern
var (
    RequestID    = capitan.NewStringKey("request_id")
    ErrorMessage = capitan.NewStringKey("error_message")
    RetryCount   = capitan.NewIntKey("retry_count")
    OrderID      = capitan.NewStringKey("order_id")
    Reason       = capitan.NewStringKey("reason")
)

type DeadLetterQueue struct {
    c    *capitan.Capitan
    file *os.File
}

func NewDeadLetterQueue(c *capitan.Capitan, path string) (*DeadLetterQueue, error) {
    file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }

    return &DeadLetterQueue{
        c:    c,
        file: file,
    }, nil
}

func (dlq *DeadLetterQueue) Start() {
    // Capture unrecoverable errors
    dlq.c.Hook(APICallAbandoned, dlq.handleAbandonedCall)
    dlq.c.Hook(OrderFailed, dlq.handleFailedOrder)
}

func (dlq *DeadLetterQueue) handleAbandonedCall(ctx context.Context, e *capitan.Event) {
    entry := map[string]any{
        "timestamp":   e.Timestamp(),
        "signal":      e.Signal().Name(),
        "request_id":  RequestID.Extract(e),
        "error":       ErrorMessage.Extract(e),
        "retry_count": RetryCount.Extract(e),
    }

    dlq.writeEntry(entry)
}

func (dlq *DeadLetterQueue) handleFailedOrder(ctx context.Context, e *capitan.Event) {
    entry := map[string]any{
        "timestamp": e.Timestamp(),
        "signal":    e.Signal().Name(),
        "order_id":  OrderID.Extract(e),
        "reason":    Reason.Extract(e),
    }

    dlq.writeEntry(entry)
}

func (dlq *DeadLetterQueue) writeEntry(entry map[string]any) {
    data, err := json.Marshal(entry)
    if err != nil {
        log.Printf("Error marshaling DLQ entry: %v", err)
        return
    }

    if _, err := dlq.file.Write(append(data, '\n')); err != nil {
        log.Printf("Error writing to DLQ: %v", err)
    }
}

func (dlq *DeadLetterQueue) Close() error {
    return dlq.file.Close()
}
```

## Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/zoobzio/capitan"
)

func main() {
    c := capitan.New()
    defer c.Shutdown()

    // Retry handler
    retryHandler := retry.NewRetryHandler(c, 3, retry.ExponentialBackoff)
    retryHandler.Start()

    // Compensation handler
    compensationHandler := compensation.NewCompensationHandler(c)
    compensationHandler.Start()

    // Circuit breaker
    circuitBreaker := circuitbreaker.NewCircuitBreaker(c, 5, 30*time.Second)
    circuitBreaker.Start()

    // Dead letter queue
    dlq, err := dlq.NewDeadLetterQueue(c, "/var/log/capitan-dlq.jsonl")
    if err != nil {
        log.Fatal(err)
    }
    defer dlq.Close()
    dlq.Start()

    // Trigger operations (use signals from the respective packages)
    c.Emit(context.Background(), retry.APICallRequested,
        retry.RequestID.Field("req-123"),
    )

    c.Shutdown()
}
```

## Next Steps

- [Cascading Workflows](./cascading-workflows.md) - Workflow orchestration
- [Event Patterns](../guides/patterns.md) - More error handling patterns
- [Testing Guide](../guides/testing.md) - Test error scenarios
