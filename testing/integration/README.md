# Integration Tests for capitan

This directory contains integration tests that verify how capitan components work together in real-world scenarios. These tests focus on component interactions, complex event coordination patterns, and end-to-end workflows.

## Test Categories

### Event Flow Tests (`event_flows_test.go`)
Tests fundamental event coordination patterns:
- Multiple signals with shared observers
- Dynamic signal creation and observer attachment
- Event propagation through complex listener networks
- Field passing through multi-stage processing

### Resilience Tests (`resilience_test.go`)
Tests how capitan handles failures and edge cases:
- Panic recovery in listeners
- Context cancellation propagation
- Listener removal during emission
- Observer closure during event processing
- Shutdown with pending events

### Real-World Scenario Tests (`real_world_test.go`)
Tests realistic application scenarios:
- Audit logging system
- Event-driven microservice communication
- User activity tracking
- System monitoring and alerting
- Multi-tenant event routing

### Concurrency Tests (`concurrency_test.go`)
Tests concurrent access patterns and race conditions:
- Concurrent Hook/Emit/Close operations
- Observer creation during signal emission
- Multiple goroutines emitting to same signal
- Worker queue saturation and backpressure
- Shutdown under load

## Running Integration Tests

### All Integration Tests
```bash
go test -v ./testing/integration/...
```

### With Race Detection
```bash
go test -v -race ./testing/integration/...
```

### Specific Test Categories
```bash
# Test event flows
go test -v -run TestEventFlows ./testing/integration/...

# Test resilience patterns
go test -v -run TestResilience ./testing/integration/...

# Test real-world scenarios
go test -v -run TestRealWorld ./testing/integration/...

# Test concurrency patterns
go test -v -run TestConcurrency ./testing/integration/...
```

### With Extended Timeout
```bash
# Run with extended timeout for stress tests
go test -v -timeout=5m -run TestConcurrency ./testing/integration/...
```

## Test Data and Fixtures

Integration tests use realistic event patterns:

### Common Signal Patterns
- System events: `system.startup`, `system.shutdown`
- User events: `user.created`, `user.updated`, `user.deleted`
- Order events: `order.placed`, `order.shipped`, `order.completed`
- Error events: `error.validation`, `error.processing`, `error.system`

### Common Field Keys
- Identifiers: `user_id`, `order_id`, `session_id`
- Metadata: `timestamp`, `source`, `priority`
- Payloads: `message`, `data`, `context`
- Errors: `error`, `error_code`, `stack_trace`

## Guidelines for Adding Integration Tests

### When to Add Integration Tests
- Testing interactions between 3+ listeners
- Testing complex observer scenarios
- Verifying end-to-end event flows
- Testing concurrent access patterns

### Test Structure
1. **Setup**: Create test signals, keys, and listeners
2. **Execute**: Emit events and observe behavior
3. **Verify**: Check event counts, field values, and state
4. **Cleanup**: Call Shutdown() and verify cleanup

### Naming Conventions
- `TestEventFlows_<Scenario>`
- `TestResilience_<Pattern>`
- `TestRealWorld_<UseCase>`
- `TestConcurrency_<Aspect>`

### Best Practices
- Use table-driven tests for multiple scenarios
- Test both success and failure paths
- Include timeout and cancellation scenarios
- Verify proper event propagation
- Check resource cleanup and shutdown behavior

## Integration Test Dependencies

### Required
- Go 1.23+ for testing features
- Context support for cancellation testing
- Race detector for concurrency testing

### Optional
- Stress testing utilities for load testing
- Profiling tools for performance analysis

## Performance Considerations

- Integration tests should complete in seconds, not minutes
- Use realistic but manageable event volumes
- Focus on correctness rather than throughput
- Include memory usage verification for long-running tests

## Debugging Integration Tests

### Verbose Output
```bash
go test -v -run TestSpecificCase ./testing/integration/...
```

### With Tracing
```bash
go test -v -trace=trace.out ./testing/integration/...
go tool trace trace.out
```

### Memory Profiling
```bash
go test -memprofile=mem.prof ./testing/integration/...
go tool pprof mem.prof
```

### Race Detection Details
```bash
go test -v -race -run TestConcurrency ./testing/integration/...
```

---

Integration tests ensure that capitan components work correctly together and provide confidence that real-world usage patterns will function as expected.
