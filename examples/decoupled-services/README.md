# Decoupled Services

**The pain**: Direct function calls between packages create tight coupling, import cycles, and testing nightmares.

**The fix**: Emit events instead of calling functions. Consumers hook in without producers knowing.

## Run Both

```bash
# See the coupling problem
go run ./before/

# See the decoupled solution
go run ./after/
```

## Before: Direct Calls

```go
func ProcessOrder(order Order) {
    SendNotification(order)  // imports notifications package
    TrackAnalytics(order)    // imports analytics package
    UpdateInventory(order)   // imports inventory package
    UpdateLoyaltyPoints(order) // imports loyalty package
    // Adding fraud detection? Edit this function...
}
```

**Problems:**
- ProcessOrder imports 4+ packages
- Adding a consumer means editing ProcessOrder
- Testing requires mocking all dependencies
- Circular dependencies as system grows
- Slow services block the whole flow

## After: Event-Driven

```go
func ProcessOrder(order Order) {
    capitan.Emit(ctx, OrderCreated,
        orderID.Field(order.ID),
        total.Field(order.Total),
    )
    // That's it. ProcessOrder doesn't know who listens.
}
```

```go
// In notifications/notifications.go - completely separate
func init() {
    capitan.Hook(OrderCreated, func(ctx context.Context, e *capitan.Event) {
        // Handle notification
    })
}
```

**Wins:**
- ProcessOrder only imports capitan
- Adding consumers = add Hook elsewhere, zero changes to producer
- Testing = verify event emitted, no mocks needed
- No import cycles - events flow one direction
- Services are isolated

## When to Use This Pattern

- Multiple packages need to react to the same action
- You're fighting circular imports
- Testing requires too many mocks
- Adding features means editing unrelated code

## Key Insight

Events invert the dependency:

**Before:** Orders → Notifications, Analytics, Inventory, Loyalty (4 dependencies)

**After:** Orders → Events ← Notifications, Analytics, Inventory, Loyalty (1 dependency each)

The producer doesn't know about consumers. Consumers don't know about each other. Everyone just knows about events.
