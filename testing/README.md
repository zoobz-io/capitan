# Testing Infrastructure for capitan

This directory contains the comprehensive testing infrastructure for the capitan package, designed to support robust development and maintenance of the event coordination system.

## Directory Structure

```
testing/
├── README.md             # This file - testing strategy overview
├── helpers.go            # Shared test utilities and mocks
├── integration/          # Integration and end-to-end tests
│   ├── README.md        # Integration testing documentation
│   ├── event_flows_test.go          # Event emission and propagation tests
│   ├── resilience_test.go           # Panic recovery and error handling
│   ├── real_world_test.go           # Real-world scenario tests
│   └── concurrency_test.go          # Concurrent access patterns
└── benchmarks/          # Performance benchmarks
    ├── README.md        # Benchmark documentation
    └── core_performance_test.go     # Core component benchmarks

```

## Testing Strategy

### Unit Tests (Root Package)
- **Location**: Alongside source files (`*_test.go`)
- **Purpose**: Test individual components in isolation
- **Coverage Goal**: 100% (currently achieved)
- **Current Coverage**: 100.0% with race detection

### Integration Tests (`testing/integration/`)
- **Purpose**: Test component interactions and real-world event coordination patterns
- **Scope**: Multi-signal workflows, complex observer patterns, concurrent scenarios
- **Focus**: Ensure components work together correctly under various conditions

### Benchmarks (`testing/benchmarks/`)
- **Purpose**: Measure and track performance characteristics
- **Scope**: Individual components, event emission patterns, observer overhead
- **Focus**: Prevent performance regressions and validate zero-allocation paths

### Test Helpers (`testing/helpers.go`)
- **Purpose**: Provide reusable testing utilities for capitan users
- **Scope**: Mock listeners, assertion helpers, event capture utilities
- **Focus**: Make testing capitan-based applications easier and more thorough

## Running Tests

### All Tests
```bash
# Run all tests with coverage
go test -v -coverprofile=coverage.out ./...

# Generate coverage report
go tool cover -html=coverage.out
```

### Unit Tests Only
```bash
# Run only unit tests (root package)
go test -v .
```

### Integration Tests Only
```bash
# Run integration tests
go test -v ./testing/integration/...

# With race detection
go test -v -race ./testing/integration/...
```

### Benchmarks Only
```bash
# Run all benchmarks
go test -v -bench=. ./testing/benchmarks/...

# Run with memory allocation tracking
go test -v -bench=. -benchmem ./testing/benchmarks/...

# Run multiple times for statistical significance
go test -v -bench=. -count=5 ./testing/benchmarks/...
```

### Performance Monitoring
```bash
# Compare benchmarks over time
go test -bench=. -count=5 ./testing/benchmarks/... > current.txt
# ... after changes ...
go test -bench=. -count=5 ./testing/benchmarks/... > new.txt
benchcmp current.txt new.txt
```

## Test Dependencies

### Required for Basic Testing
- Go 1.23+ (for testing features)
- Standard library only

### Required for Advanced Testing
- Race detector: `go test -race`
- Coverage tools: `go tool cover`

### Optional Performance Tools
- `benchcmp` for comparing benchmark results
- `pprof` for performance profiling

## Test Data and Fixtures

### Test Data Patterns
- Use table-driven tests for multiple scenarios
- Create test data that represents real-world usage
- Include edge cases and boundary conditions
- Use consistent signal/key definitions across tests

### Fixture Guidelines
- Keep fixtures small and focused
- Make fixtures self-documenting
- Define signals and keys as test-scoped constants
- Avoid external file dependencies

## Best Practices

### Test Organization
1. **Hierarchical naming**: Use descriptive test names that form a hierarchy
2. **Consistent structure**: Follow the same pattern across all test files
3. **Isolated tests**: Each test should be completely independent
4. **Fast tests**: Keep unit tests fast, put slow tests in integration

### Event Testing
1. **Test both emission and reception**
2. **Verify field types and values**
3. **Test event propagation through observers**
4. **Include context cancellation scenarios**

### Concurrency Testing
1. **Use `-race` flag regularly**
2. **Test concurrent Hook/Emit/Close operations**
3. **Verify proper cleanup in failure scenarios**
4. **Test observer attachment during signal creation**

### Performance Testing
1. **Benchmark realistic event patterns**
2. **Include both CPU and memory allocation metrics**
3. **Test worker scaling behavior**
4. **Validate zero-allocation paths**

## Continuous Integration

### Pre-commit Checks
```bash
# Run this before committing
make test          # All tests with race detection
make lint          # Code quality checks
make coverage      # Coverage verification
```

### CI Pipeline Requirements
- All tests must pass
- Coverage must remain 100%
- No race conditions detected
- Benchmarks must not regress significantly

## Contributing Test Infrastructure

### Adding New Tests
1. Determine appropriate test category (unit/integration/benchmark)
2. Follow existing naming conventions
3. Update relevant README files
4. Ensure tests are deterministic and fast

### Adding New Helpers
1. Add to `testing/helpers.go`
2. Include comprehensive documentation
3. Provide usage examples
4. Ensure thread safety if applicable

### Performance Considerations
- Unit tests should complete in milliseconds
- Integration tests should complete in seconds
- Benchmarks should run long enough for stable measurements
- Avoid sleeps in tests unless testing timing behavior

---

This testing infrastructure ensures capitan remains reliable, performant, and easy to use while providing comprehensive tools for users to test their own capitan-based applications.
