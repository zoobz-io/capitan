# Async Notifications

**The pain**: Synchronous notification calls block API responses. User waits for email, SMS, analytics... all to complete.

**The fix**: Fire-and-forget events. User gets response immediately, notifications process in background.

## Run Both

```bash
# See the slow sync approach (~1.2s response time)
go run ./before/

# See the fast async approach (~50ms response time)
go run ./after/
```

## Before: Synchronous Calls

```go
func handleCheckout(orderID string) string {
    processPayment(orderID)       // 50ms  - must wait
    sendEmail(email, orderID)     // 500ms - user waiting...
    sendSMS(phone, orderID)       // 300ms - still waiting...
    sendPushNotification(orderID) // 200ms - still waiting...
    updateAnalytics(orderID)      // 100ms - still waiting...
    return "Order confirmed"      // 1150ms later!
}
```

**Problems:**
- API latency = sum of ALL operations (~1.2 seconds)
- User stares at loading spinner
- Slow SMTP server degrades checkout UX
- One timeout can fail the whole request

## After: Async Events

```go
func handleCheckout(orderID string) string {
    processPayment(orderID)  // 50ms - must wait

    capitan.Emit(ctx, OrderPaid,  // Returns immediately!
        orderID.Field(orderID),
        email.Field(email),
    )

    return "Order confirmed"  // 50ms total!
}
```

**Wins:**
- API latency = only critical path (~50ms)
- User sees confirmation immediately
- Notifications process in background workers
- Slow services don't affect user experience
- Per-signal workers provide isolation

## The Numbers

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| API Response | 1150ms | 50ms | **23x faster** |
| User Wait | 1150ms | 50ms | **23x faster** |
| P99 Latency | Worst service | Critical path only | Predictable |

## When to Use This Pattern

- API responses are slow due to downstream calls
- Users wait for non-critical operations (email, analytics)
- You need to decouple response time from notification time
- P99 latency is dominated by slow external services

## Key Insight

Not everything needs to block the response:

**Must be sync (critical path):**
- Payment processing
- Inventory reservation
- Data validation

**Can be async (fire-and-forget):**
- Email notifications
- SMS/Push notifications
- Analytics tracking
- Audit logging
- Warehouse notifications

capitan makes this trivial: `Emit()` returns immediately, listeners process async.
