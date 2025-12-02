---
title: Event Patterns
description: Common event coordination patterns and best practices for capitan applications.
author: Capitan Team
published: 2025-12-01
tags: [Patterns, Guide, Best Practices]
---

# Event Patterns

Common event coordination patterns for capitan applications.

## Table of Contents

- [Cascading Events](#cascading-events)
- [Event Aggregation](#event-aggregation)
- [Saga Pattern](#saga-pattern)
- [Retry Pattern](#retry-pattern)
- [Circuit Breaker Pattern](#circuit-breaker-pattern)
- [Fan-Out/Fan-In](#fan-outfan-in)
- [Event Filtering](#event-filtering)
- [Multi-Tenant Routing](#multi-tenant-routing)
- [Audit Trail](#audit-trail)
- [Debouncing](#debouncing)

## Cascading Events

Events trigger other events in sequence.

```go
orderPlaced := capitan.NewSignal("order.placed", "Order placed")
inventoryChecked := capitan.NewSignal("inventory.checked", "Inventory checked")
paymentProcessed := capitan.NewSignal("payment.processed", "Payment processed")
orderShipped := capitan.NewSignal("order.shipped", "Order shipped")

orderIDKey := capitan.NewStringKey("order_id")

// Step 1: Order → Inventory
c.Hook(orderPlaced, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if checkInventory(orderID) {
        c.Emit(ctx, inventoryChecked, orderIDKey.Field(orderID))
    }
})

// Step 2: Inventory → Payment
c.Hook(inventoryChecked, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if processPayment(orderID) {
        c.Emit(ctx, paymentProcessed, orderIDKey.Field(orderID))
    }
})

// Step 3: Payment → Shipping
c.Hook(paymentProcessed, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)
    shipOrder(orderID)
    c.Emit(ctx, orderShipped, orderIDKey.Field(orderID))
})
```

**Use Cases**:
- Order processing workflows
- Multi-step approval processes
- State machine implementations

## Event Aggregation

Collect multiple events before acting.

```go
type OrderAggregator struct {
    pending map[string]*OrderState
    mu      sync.Mutex
}

type OrderState struct {
    validated bool
    paid      bool
    shipped   bool
}

func (a *OrderAggregator) Start(c *capitan.Capitan) {
    c.Hook(orderValidated, a.handleValidated)
    c.Hook(paymentProcessed, a.handlePaid)
    c.Hook(orderShipped, a.handleShipped)
}

func (a *OrderAggregator) handleValidated(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    a.mu.Lock()
    defer a.mu.Unlock()

    if _, ok := a.pending[orderID]; !ok {
        a.pending[orderID] = &OrderState{}
    }

    a.pending[orderID].validated = true
    a.checkComplete(ctx, orderID)
}

func (a *OrderAggregator) checkComplete(ctx context.Context, orderID string) {
    state := a.pending[orderID]

    if state.validated && state.paid && state.shipped {
        c.Emit(ctx, orderCompleted, orderIDKey.Field(orderID))
        delete(a.pending, orderID)
    }
}
```

**Use Cases**:
- Distributed transaction coordination
- Waiting for multiple async operations
- Complex state tracking

## Saga Pattern

Compensating transactions for distributed workflows.

```go
// Success path
var (
    OrderCreated      = capitan.NewSignal("order.created", "Order created")
    InventoryReserved = capitan.NewSignal("inventory.reserved", "Inventory reserved")
    PaymentCharged    = capitan.NewSignal("payment.charged", "Payment charged")
    OrderCompleted    = capitan.NewSignal("order.completed", "Order completed")
)

// Failure path (compensation)
var (
    OrderFailed       = capitan.NewSignal("order.failed", "Order failed")
    InventoryReleased = capitan.NewSignal("inventory.released", "Inventory released")
    PaymentRefunded   = capitan.NewSignal("payment.refunded", "Payment refunded")
)

// Forward flow
c.Hook(OrderCreated, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if reserveInventory(orderID) {
        c.Emit(ctx, InventoryReserved, orderIDKey.Field(orderID))
    } else {
        c.Emit(ctx, OrderFailed, orderIDKey.Field(orderID))
    }
})

c.Hook(InventoryReserved, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)

    if chargePayment(orderID) {
        c.Emit(ctx, PaymentCharged, orderIDKey.Field(orderID))
    } else {
        // Compensate: release inventory
        c.Emit(ctx, InventoryReleased, orderIDKey.Field(orderID))
        c.Emit(ctx, OrderFailed, orderIDKey.Field(orderID))
    }
})

// Compensation handlers
c.Hook(OrderFailed, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)
    log.Printf("Order %s failed, cleaning up", orderID)
})

c.Hook(InventoryReleased, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)
    releaseInventory(orderID)
})

c.Hook(PaymentRefunded, func(ctx context.Context, e *capitan.Event) {
    orderID := orderIDKey.Extract(e)
    refundPayment(orderID)
})
```

**Use Cases**:
- Distributed transactions
- Multi-step business processes
- Rollback scenarios

## Retry Pattern

Retry failed operations with backoff.

```go
type RetryHandler struct {
    maxRetries int
    attempts   map[string]int
    mu         sync.Mutex
}

func NewRetryHandler(maxRetries int) *RetryHandler {
    return &RetryHandler{
        maxRetries: maxRetries,
        attempts:   make(map[string]int),
    }
}

func (r *RetryHandler) Start(c *capitan.Capitan) {
    c.Hook(APICallRequested, r.handleRequest)
    c.Hook(APICallFailed, r.handleFailure)
}

func (r *RetryHandler) handleRequest(ctx context.Context, e *capitan.Event) {
    requestID := requestIDKey.Extract(e)

    if err := callExternalAPI(requestID); err != nil {
        c.Emit(ctx, APICallFailed,
            requestIDKey.Field(requestID),
            errorKey.Field(err.Error()),
        )
    } else {
        c.Emit(ctx, APICallSucceeded, requestIDKey.Field(requestID))
    }
}

func (r *RetryHandler) handleFailure(ctx context.Context, e *capitan.Event) {
    requestID := requestIDKey.Extract(e)

    r.mu.Lock()
    r.attempts[requestID]++
    attempts := r.attempts[requestID]
    r.mu.Unlock()

    if attempts < r.maxRetries {
        // Exponential backoff
        backoff := time.Duration(attempts*attempts) * time.Second

        log.Printf("Retry attempt %d/%d for %s after %v",
            attempts, r.maxRetries, requestID, backoff)

        time.Sleep(backoff)

        // Retry
        c.Emit(ctx, APICallRequested, requestIDKey.Field(requestID))
    } else {
        log.Printf("Max retries reached for %s", requestID)
        c.Emit(ctx, APICallAbandoned, requestIDKey.Field(requestID))

        r.mu.Lock()
        delete(r.attempts, requestID)
        r.mu.Unlock()
    }
}
```

**Use Cases**:
- External API calls
- Transient failures
- Network operations

## Circuit Breaker Pattern

Prevent cascading failures by short-circuiting failing operations.

```go
type CircuitBreaker struct {
    threshold int
    failures  int
    state     string // "closed", "open", "half-open"
    lastFail  time.Time
    timeout   time.Duration
    mu        sync.Mutex
}

func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        threshold: threshold,
        state:     "closed",
        timeout:   timeout,
    }
}

func (cb *CircuitBreaker) Start(c *capitan.Capitan) {
    c.Hook(ServiceCallRequested, cb.handleRequest)
    c.Hook(ServiceCallFailed, cb.recordFailure)
    c.Hook(ServiceCallSucceeded, cb.recordSuccess)
}

func (cb *CircuitBreaker) handleRequest(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    // Check if circuit should transition to half-open
    if cb.state == "open" && time.Since(cb.lastFail) > cb.timeout {
        cb.state = "half-open"
        log.Printf("Circuit breaker half-open, trying request")
    }

    // Reject if open
    if cb.state == "open" {
        c.EmitWithSeverity(ctx, capitan.SeverityWarn, ServiceCallRejected,
            reasonKey.Field("circuit breaker open"),
        )
        return
    }

    // Allow request
    if err := callService(); err != nil {
        c.Emit(ctx, ServiceCallFailed)
    } else {
        c.Emit(ctx, ServiceCallSucceeded)
    }
}

func (cb *CircuitBreaker) recordFailure(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures++
    cb.lastFail = time.Now()

    if cb.failures >= cb.threshold {
        cb.state = "open"
        log.Printf("Circuit breaker opened after %d failures", cb.failures)
    }
}

func (cb *CircuitBreaker) recordSuccess(ctx context.Context, e *capitan.Event) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == "half-open" {
        cb.state = "closed"
        cb.failures = 0
        log.Printf("Circuit breaker closed after successful request")
    }
}
```

**Use Cases**:
- External service calls
- Preventing cascading failures
- System resilience

## Fan-Out/Fan-In

Emit to multiple workers, collect results.

```go
// Fan-out: Send to multiple workers
c.Hook(JobReceived, func(ctx context.Context, e *capitan.Event) {
    jobID := jobIDKey.Extract(e)

    // Split into chunks
    chunks := splitJob(jobID)

    for i, chunk := range chunks {
        c.Emit(ctx, ChunkProcessing,
            jobIDKey.Field(jobID),
            chunkIDKey.Field(fmt.Sprintf("%s-%d", jobID, i)),
            dataKey.Field(chunk),
        )
    }
})

// Fan-in: Collect results
type ResultCollector struct {
    results map[string][]Result
    mu      sync.Mutex
}

func (rc *ResultCollector) Start(c *capitan.Capitan) {
    c.Hook(ChunkCompleted, rc.handleChunk)
}

func (rc *ResultCollector) handleChunk(ctx context.Context, e *capitan.Event) {
    jobID := jobIDKey.Extract(e)
    result := resultKey.Extract(e)

    rc.mu.Lock()
    defer rc.mu.Unlock()

    rc.results[jobID] = append(rc.results[jobID], result)

    // Check if all chunks done
    if len(rc.results[jobID]) == expectedChunks {
        finalResult := mergeResults(rc.results[jobID])
        c.Emit(ctx, JobCompleted,
            jobIDKey.Field(jobID),
            resultKey.Field(finalResult),
        )
        delete(rc.results, jobID)
    }
}
```

**Use Cases**:
- Parallel processing
- Map-reduce operations
- Batch job coordination

## Event Filtering

Filter events based on criteria.

```go
type EventFilter struct {
    minSeverity capitan.Severity
}

func (f *EventFilter) Start(c *capitan.Capitan) {
    // Observe all events
    c.Observe(f.filterBySeverity)
}

func (f *EventFilter) filterBySeverity(ctx context.Context, e *capitan.Event) {
    if e.Severity() >= f.minSeverity {
        // Forward to alerting
        c.Emit(ctx, AlertTriggered,
            signalKey.Field(e.Signal().Name()),
            severityKey.Field(int(e.Severity())),
        )
    }
}

// Field-based filtering
c.Hook(UserAction, func(ctx context.Context, e *capitan.Event) {
    userType := userTypeKey.Extract(e)

    if userType == "premium" {
        c.Emit(ctx, PremiumUserAction, /* fields */)
    } else {
        c.Emit(ctx, StandardUserAction, /* fields */)
    }
})
```

**Use Cases**:
- Severity-based routing
- User type filtering
- Feature flag checks

## Multi-Tenant Routing

Route events based on tenant ID.

```go
type TenantRouter struct {
    tenants map[string]*TenantHandler
    mu      sync.RWMutex
}

func (tr *TenantRouter) Start(c *capitan.Capitan) {
    c.Hook(DataUpdated, tr.routeByTenant)
}

func (tr *TenantRouter) routeByTenant(ctx context.Context, e *capitan.Event) {
    tenantID := tenantIDKey.Extract(e)

    tr.mu.RLock()
    handler, ok := tr.tenants[tenantID]
    tr.mu.RUnlock()

    if !ok {
        log.Printf("No handler for tenant %s", tenantID)
        return
    }

    handler.ProcessUpdate(ctx, e)
}

// Tenant-specific caching
c.Hook(DataUpdated, func(ctx context.Context, e *capitan.Event) {
    tenantID := tenantIDKey.Extract(e)
    dataKey := dataIDKey.Extract(e)

    tenantCache[tenantID].Invalidate(dataKey)
})
```

**Use Cases**:
- SaaS applications
- Tenant isolation
- Per-tenant customization

## Audit Trail

Observer-based audit logging.

```go
type AuditLogger struct {
    storage AuditStorage
}

func (al *AuditLogger) Start(c *capitan.Capitan) {
    // Observe all events
    observer := c.Observe(al.logEvent)
    // Keep observer alive for application lifetime
}

func (al *AuditLogger) logEvent(ctx context.Context, e *capitan.Event) {
    entry := AuditEntry{
        Timestamp: e.Timestamp(),
        Signal:    e.Signal().Name(),
        Severity:  e.Severity(),
        Fields:    e.Fields(),
    }

    // Extract user context if available
    if userID, ok := ctx.Value("user_id").(string); ok {
        entry.UserID = userID
    }

    al.storage.Save(entry)
}

// Sensitive event auditing
sensitiveSignals := []capitan.Signal{
    UserLogin,
    UserLogout,
    PasswordChanged,
    PermissionGranted,
    DataExported,
}

securityAudit := c.Observe(securityAuditHandler, sensitiveSignals...)
```

**Use Cases**:
- Compliance requirements
- Security monitoring
- Debug tracing

## Debouncing

Prevent rapid-fire events.

```go
type Debouncer struct {
    delay     time.Duration
    timers    map[string]*time.Timer
    mu        sync.Mutex
}

func NewDebouncer(delay time.Duration) *Debouncer {
    return &Debouncer{
        delay:  delay,
        timers: make(map[string]*time.Timer),
    }
}

func (d *Debouncer) Start(c *capitan.Capitan) {
    c.Hook(SearchQueryChanged, d.debounceSearch)
}

func (d *Debouncer) debounceSearch(ctx context.Context, e *capitan.Event) {
    queryID := queryIDKey.Extract(e)

    d.mu.Lock()
    defer d.mu.Unlock()

    // Cancel existing timer
    if timer, ok := d.timers[queryID]; ok {
        timer.Stop()
    }

    // Create new timer
    d.timers[queryID] = time.AfterFunc(d.delay, func() {
        // Execute after delay
        c.Emit(ctx, SearchExecuted, queryIDKey.Field(queryID))

        d.mu.Lock()
        delete(d.timers, queryID)
        d.mu.Unlock()
    })
}
```

**Use Cases**:
- Search input handling
- Auto-save functionality
- Rate limiting user actions

## Pattern Combinations

Combine patterns for complex scenarios:

```go
// Retry + Circuit Breaker
type ResilientService struct {
    retry   *RetryHandler
    breaker *CircuitBreaker
}

// Fan-out + Aggregation
type ParallelProcessor struct {
    collector *ResultCollector
}

// Saga + Retry
type ResilientSaga struct {
    retry *RetryHandler
}

// Audit + Filter
auditObserver := c.Observe(auditHandler)
filterHandler := &EventFilter{minSeverity: capitan.SeverityWarn}
```

## Next Steps

- [Testing Guide](./testing.md) - Test event patterns
- [Best Practices](./best-practices.md) - Production patterns
- [Observers Guide](./observers.md) - Advanced observer usage
- [Cookbook](../cookbook/README.md) - Complete examples
