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

## Send a Signal, Listen Anywhere

Events carry typed fields with compile-time safety.

```go
// Define typed keys
orderID := capitan.NewStringKey("order_id")
total   := capitan.NewFloat64Key("total")

// Define a signal
orderCreated := capitan.NewSignal("order.created", "New order placed")

// Emit with typed fields
capitan.Emit(ctx, orderCreated,
    orderID.Field("ORD-123"),
    total.Field(99.99),
)
```

Each signal queues to its own worker — isolated, async, backpressure-aware.

```go
// Hook a listener — extract typed values directly
capitan.Hook(orderCreated, func(ctx context.Context, e *capitan.Event) {
    id, _ := orderID.From(e)      // string
    amount, _ := total.From(e)    // float64
    process(id, amount)
})

// Observe all signals — unified visibility across the system
capitan.Observe(func(ctx context.Context, e *capitan.Event) {
    log.Info("event", "signal", e.Signal().Name())
})
```

Type-safe at the edges. Async and isolated in between.

## Installation

```bash
go get github.com/zoobzio/capitan
```

Requires Go 1.24+.

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

## Capabilities

| Feature            | Description                                                   | Docs                                                |
| ------------------ | ------------------------------------------------------------- | --------------------------------------------------- |
| Typed Fields       | Built-in keys for primitives; `NewKey[T]` for any custom type | [Fields](docs/5.reference/2.fields.md)              |
| Per-Signal Workers | Each signal gets its own goroutine and buffered queue         | [Architecture](docs/2.learn/3.architecture.md)      |
| Observers          | Cross-cutting handlers that see all signals (or a whitelist)  | [Concepts](docs/2.learn/2.concepts.md)              |
| Configuration      | Buffer sizes, rate limits, drop policies, panic handlers      | [Configuration](docs/3.guides/1.configuration.md)   |
| Graceful Shutdown  | Drain pending events before exit                              | [Best Practices](docs/3.guides/5.best-practices.md) |
| Testing Utilities  | Sync mode, event capture, isolated instances                  | [Testing](docs/3.guides/4.testing.md)               |

## Why capitan?

- **Type-safe** — Typed fields with compile-time safety, including custom structs
- **Zero dependencies** — Standard library only
- **Async by default** — Per-signal workers with backpressure
- **Isolated** — Slow listeners don't affect other signals
- **Panic-safe** — Listener panics recovered, system stays running
- **Testable** — Sync mode and capture utilities for deterministic tests

## Decoupled Coordination

Capitan enables a pattern: **packages emit, concerned parties listen, services observe**.

Your domain packages emit events when meaningful things happen. Other packages hook the signals they care about. Service-level concerns — audit trails, structured logging, metrics collection — observe everything through a unified stream.

```go
// In your order package
capitan.Emit(ctx, orderCreated, orderID.Field(id), total.Field(amount))

// In your notification package
capitan.Hook(orderCreated, sendConfirmationEmail)

// In your audit service
capitan.Observe(writeToAuditLog)
```

Three packages, one event flow, zero direct imports between them.

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. Run `make help` for available commands.

## License

MIT License — see [LICENSE](LICENSE) for details.
