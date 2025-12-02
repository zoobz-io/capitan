# Logging Evolution

**The pain**: Scattered `log.Printf` calls with inconsistent formats, no structure, impossible to aggregate.

**The fix**: Typed events with structured fields, single observer for consistent logging.

## Run Both

```bash
# See the problems
go run ./before/

# See the solution
go run ./after/
```

## Before: The Typical Approach

```go
log.Printf("[INFO] Created order %s for customer %s, total: $%.2f", ...)
log.Printf("Payment processed - OrderID: %s, Amount: %.2f", ...)
log.Printf("order=%s status=shipped items=%d", ...)
```

**Problems:**
- Inconsistent formats across files
- No structured data - try grepping for orders over $200
- Change logging format? Edit every file
- No type safety - `log.Printf("total: %s", 299.99)` compiles fine
- Hard to test what gets logged

## After: Structured Events

```go
capitan.Info(ctx, OrderCreated,
    orderID.Field(order.ID),
    total.Field(order.Total),
)
```

**Wins:**
- Type-safe: `total.Field("wrong")` won't compile
- Structured: Fields are machine-parseable
- Consistent: One observer controls all output format
- Testable: Hook a test observer, assert on events
- Flexible: Add JSON output, metrics, tracing - in one place

## When to Use This Pattern

- You have logging scattered across packages
- You need to aggregate logs (ELK, Datadog, etc.)
- You want to test that specific events occur
- You're tired of inconsistent log formats

## Key Insight

The observer pattern means logging logic lives in ONE place:

```go
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Change this, change ALL logging
    json.NewEncoder(os.Stdout).Encode(e)
})
```

Switch from text to JSON? One line. Add timestamps? One line. Filter by severity? One line.
