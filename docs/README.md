---
title: Capitan Documentation
description: Type-safe event coordination for Go with per-signal worker isolation and zero-allocation event pooling.
author: Capitan Team
published: 2025-12-01
tags: [Documentation, Overview, Getting Started]
---

# capitan Documentation

Welcome to capitan - a type-safe event coordination library for Go.

## Start Here

- **[Getting Started](./tutorials/getting-started.md)** - Build your first event system in 10 minutes
- **[Core Concepts](./learn/core-concepts.md)** - Understand signals, listeners, and observers
- **[API Reference](./reference/api.md)** - Complete API documentation

## Learn

- **[Core Concepts](./learn/core-concepts.md)** - Mental models and fundamentals
- **[Architecture](./learn/architecture.md)** - System design and worker isolation
- **[Introduction](./learn/introduction.md)** - Why capitan exists and what problems it solves

## Tutorials

- **[Getting Started](./tutorials/getting-started.md)** - Complete introduction with examples
- **[Quickstart](./tutorials/quickstart.md)** - Your first event system in 5 minutes
- **[First Event System](./tutorials/first-event-system.md)** - Complete worked example

## Guides

- **[Testing](./guides/testing.md)** - Testing patterns with capitan helpers
- **[Event Patterns](./guides/patterns.md)** - Common event coordination patterns
- **[Observers](./guides/observers.md)** - Dynamic signal observation
- **[Concurrency](./guides/concurrency.md)** - Worker isolation and thread safety
- **[Best Practices](./guides/best-practices.md)** - Production-ready patterns
- **[Performance](./guides/performance.md)** - Optimization and benchmarking

## Cookbook

- **[Cookbook Index](./cookbook/README.md)** - Recipe collection for common patterns
- **[Audit Logging](./cookbook/audit-logging.md)** - Observer-based audit trails
- **[Multi-Tenant Events](./cookbook/multi-tenant-events.md)** - Tenant-specific routing
- **[Cascading Workflows](./cookbook/cascading-workflows.md)** - Event-driven workflows
- **[Error Handling](./cookbook/error-handling.md)** - Retry and recovery patterns

## Reference

- **[API Reference](./reference/api.md)** - Complete API documentation
- **[Field Types](./reference/fields.md)** - Typed event fields and extraction
- **[Configuration](./reference/configuration.md)** - Capitan options and tuning

## Quick Links

- [GitHub Repository](https://github.com/zoobzio/capitan)
- [Go Package Documentation](https://pkg.go.dev/github.com/zoobzio/capitan)
- [Test Coverage Report](https://codecov.io/gh/zoobzio/capitan)

## Features

### Type-Safe Events

Events carry strongly-typed fields with compile-time safety:

```go
key := capitan.NewStringKey("user_id")
c.Emit(ctx, signal, key.Field("user-123"))

// Extract with type safety
userID := key.Extract(event) // Returns string
```

### Per-Signal Workers

Each signal gets its own goroutine worker for perfect isolation:

- No cross-signal contention
- Independent processing guarantees
- Ordered event delivery per signal

### Zero-Allocation Pooling

Events are pooled via `sync.Pool` for high-throughput scenarios:

- No allocations in hot paths
- Automatic memory reuse
- Minimal GC pressure

### Dynamic Observers

Observe all signals (or a whitelist) without pre-registration:

```go
observer := c.Observe(func(ctx context.Context, e *capitan.Event) {
    // Receives events from all signals
})
```

### Built-In Testing

Comprehensive testing infrastructure with helpers that solve event pooling issues:

```go
capture := capitantesting.NewEventCapture()
c.Hook(sig, capture.Handler())

// Defensive copying handles event pooling automatically
events := capture.Events()
```
