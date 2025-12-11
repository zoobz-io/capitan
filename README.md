# capitan

[![CI Status](https://github.com/zoobzio/capitan/workflows/CI/badge.svg)](https://github.com/zoobzio/capitan/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/capitan/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/capitan)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/capitan)](https://goreportcard.com/report/github.com/zoobzio/capitan)
[![CodeQL](https://github.com/zoobzio/capitan/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/capitan/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/capitan.svg)](https://pkg.go.dev/github.com/zoobzio/capitan)
[![License](https://img.shields.io/github/license/zoobzio/capitan)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/capitan)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/capitan)](https://github.com/zoobzio/capitan/releases)

Type-safe event coordination for Go with zero dependencies.

Emit events with typed fields, hook listeners, and let capitan handle the rest with async processing and backpressure.

## Three Operations

```go
// Emit an event
capitan.Emit(ctx, signal, fields...)

// Hook a listener
listener := capitan.Hook(signal, func(ctx context.Context, e *capitan.Event) {
    // Handle event
})

// Observe all signals
observer := capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    // Handle any event
})
```

No schemas to declare, no complex configuration—just events and listeners.

## Installation

```bash
go get github.com/zoobzio/capitan
```

Requires Go 1.23+.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/zoobzio/capitan"
)

// Define signals and keys as package-level variables
var (
    orderCreated = capitan.NewSignal("order.created", "New order placed")
    orderID      = capitan.NewStringKey("order_id")
    total        = capitan.NewFloat64Key("total")
)

func main() {
    // Hook a listener
    capitan.Hook(orderCreated, func(ctx context.Context, e *capitan.Event) {
        id, _ := orderID.From(e)
        amount, _ := total.From(e)
        fmt.Printf("Order %s: $%.2f\n", id, amount)
    })

    // Emit an event (async with backpressure)
    capitan.Emit(context.Background(), orderCreated,
        orderID.Field("ORDER-123"),
        total.Field(99.99),
    )

    // Gracefully drain pending events
    capitan.Shutdown()
}
```

## Custom Types

Any type works with full type safety using `NewKey[T]`:

```go
type Order struct {
    ID       string
    Customer string
    Total    float64
    Items    int
}

var orderKey = capitan.NewKey[Order]("order", "myapp.Order")

capitan.Hook(orderCreated, func(ctx context.Context, e *capitan.Event) {
    order, ok := orderKey.From(e)  // order is type Order, not any
    if ok {
        fmt.Printf("%s ordered %d items ($%.2f)\n", order.Customer, order.Items, order.Total)
    }
})

capitan.Emit(ctx, orderCreated, orderKey.Field(Order{
    ID:       "ORD-456",
    Customer: "Alice",
    Total:    149.99,
    Items:    3,
}))
```

## Why capitan?

- **Type-safe** — Typed fields with compile-time safety, including custom structs
- **Zero dependencies** — Standard library only
- **Async by default** — Per-signal workers with backpressure
- **Isolated** — Slow listeners don't affect other signals
- **Panic-safe** — Listener panics recovered, system stays running
- **Testable** — Sync mode and capture utilities for deterministic tests

## Documentation

Full documentation is available in the [docs/](docs/) directory:

### Learn
- [Quickstart](docs/2.learn/1.quickstart.md) — Get started in minutes
- [Core Concepts](docs/2.learn/2.concepts.md) — Signals, keys, fields, listeners, observers
- [Architecture](docs/2.learn/3.architecture.md) — Per-signal workers, event pooling, backpressure

### Guides
- [Configuration](docs/3.guides/1.configuration.md) — Buffer sizes, panic handlers, runtime metrics
- [Context](docs/3.guides/2.context.md) — Request tracing, cancellation, timeouts
- [Errors](docs/3.guides/3.errors.md) — Error propagation, severity levels, retry patterns
- [Testing](docs/3.guides/4.testing.md) — Sync mode, event capture, isolated instances
- [Best Practices](docs/3.guides/5.best-practices.md) — Signal design, listener lifecycle, performance

### Cookbook
- [Observability](docs/4.cookbook/1.observability.md) — Centralized logging without coupling
- [Workflows](docs/4.cookbook/2.workflows.md) — Event-driven choreography
- [Distribution](docs/4.cookbook/3.distribution.md) — Kafka, NATS, and message broker integration
- [Persistence](docs/4.cookbook/4.persistence.md) — Event storage and replay

### Reference
- [API Reference](docs/5.reference/1.api.md) — Complete function and type documentation
- [Fields Reference](docs/5.reference/2.fields.md) — All built-in and custom field types
- [Testing Reference](docs/5.reference/3.testing.md) — Test utilities API

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `go test ./...`
- Code is formatted: `go fmt ./...`
- No lint errors: `golangci-lint run`

## License

MIT License — see [LICENSE](LICENSE) for details.
