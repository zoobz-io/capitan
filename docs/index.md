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

- **[Getting Started](./2.tutorials/2.getting-started.md)** - Build your first event system in 10 minutes
- **[Core Concepts](./1.learn/2.core-concepts.md)** - Understand signals, listeners, and observers
- **[API Reference](./5.reference/1.api.md)** - Complete API documentation

## Learn

- **[Introduction](./1.learn/1.introduction.md)** - Why capitan exists and what problems it solves
- **[Core Concepts](./1.learn/2.core-concepts.md)** - Mental models and fundamentals
- **[Architecture](./1.learn/3.architecture.md)** - System design and worker isolation

## Tutorials

- **[Quickstart](./2.tutorials/1.quickstart.md)** - Your first event system in 5 minutes
- **[Getting Started](./2.tutorials/2.getting-started.md)** - Complete introduction with examples
- **[First Event System](./2.tutorials/3.first-event-system.md)** - Complete worked example

## Guides

- **[Testing](./3.guides/1.testing.md)** - Testing patterns with capitan helpers
- **[Event Patterns](./3.guides/2.patterns.md)** - Common event coordination patterns
- **[Observers](./3.guides/3.observers.md)** - Dynamic signal observation
- **[Concurrency](./3.guides/4.concurrency.md)** - Worker isolation and thread safety
- **[Performance](./3.guides/5.performance.md)** - Optimization and benchmarking
- **[Best Practices](./3.guides/6.best-practices.md)** - Production-ready patterns

## Cookbook

- **[Cookbook Overview](./4.cookbook/1.overview.md)** - Recipe collection for common patterns
- **[Audit Logging](./4.cookbook/2.audit-logging.md)** - Observer-based audit trails
- **[Error Handling](./4.cookbook/3.error-handling.md)** - Retry and recovery patterns
- **[Cascading Workflows](./4.cookbook/4.cascading-workflows.md)** - Event-driven workflows
- **[Multi-Tenant Events](./4.cookbook/5.multi-tenant-events.md)** - Tenant-specific routing

## Reference

- **[API Reference](./5.reference/1.api.md)** - Complete API documentation
- **[Field Types](./5.reference/2.fields.md)** - Typed event fields and extraction
- **[Configuration](./5.reference/3.configuration.md)** - Capitan options and tuning

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
