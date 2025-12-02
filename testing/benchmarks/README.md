# Benchmark Suite for capitan

This directory contains comprehensive performance benchmarks for the capitan package. The benchmarks are organized to measure different aspects of event coordination performance and provide comparisons with traditional approaches.

## Benchmark Categories

### Core Performance (`core_performance_test.go`)
Benchmarks fundamental capitan components in isolation:
- **Event Emission**: Emit to signals with various field counts
- **Worker Creation**: Lazy worker goroutine initialization
- **Event Processing**: Listener invocation overhead
- **Event Pooling**: Memory allocation patterns
- **Observer Attachment**: Dynamic observer registration
- **Field Access**: Typed field extraction performance

All benchmarks in `core_performance_test.go` measure fundamental operations in isolation to establish performance baselines and track regressions.

## Running Benchmarks

### All Benchmarks
```bash
# Run all benchmarks
go test -v -bench=. ./testing/benchmarks/...

# Run with memory allocation tracking
go test -v -bench=. -benchmem ./testing/benchmarks/...

# Run multiple times for statistical significance
go test -v -bench=. -count=5 ./testing/benchmarks/...
```

### Specific Categories
```bash
# All benchmarks
go test -v -bench=Benchmark ./testing/benchmarks/...
```

### Performance Profiling
```bash
# CPU profiling
go test -bench=BenchmarkSpecific -cpuprofile=cpu.prof ./testing/benchmarks/...
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkSpecific -memprofile=mem.prof ./testing/benchmarks/...
go tool pprof mem.prof

# Block profiling for concurrency
go test -bench=BenchmarkConcurrent -blockprofile=block.prof ./testing/benchmarks/...
go tool pprof block.prof
```

## Benchmark Metrics

### Performance Targets
Based on expected performance, capitan aims for:
- **Event Emission (async)**: < 500ns/op, < 2 allocs/op
- **Event Emission (sync)**: < 100ns/op, < 1 alloc/op
- **Worker Creation**: < 10µs/op (lazy, one-time cost)
- **Listener Invocation**: < 50ns/op overhead
- **Field Extraction**: < 10ns/op, 0 allocs/op
- **Observer Attachment**: < 100ns/op per signal

### Zero-Allocation Paths
These operations should have zero allocations:
- Field extraction from events
- Event pooling (Get/Put)
- Signal comparison
- Stats reading (except map copies)

### Overhead Measurements
- **Per-event overhead**: Event pooling + field storage
- **Per-listener overhead**: Callback invocation
- **Worker overhead**: Goroutine + channel buffering
- **Observer overhead**: Additional listener per signal

## Benchmark Methodology

### Comparison Fairness
All comparison benchmarks follow strict equivalence principles:

1. **Functional Equivalence**: Traditional implementations produce identical results to capitan equivalents
2. **No Artificial Complexity**: Traditional code uses reasonable, production-quality patterns
3. **Same Concurrency Model**: Both approaches use goroutines and channels where appropriate
4. **Equivalent Resource Usage**: No intentional resource waste in traditional implementations

### Measurement Accuracy
- **Setup Exclusion**: Setup time excluded via `b.ResetTimer()`
- **Allocation Tracking**: Memory allocations measured with `b.ReportAllocs()`
- **Compiler Optimization**: Results stored to prevent dead code elimination
- **Statistical Validity**: Multiple runs recommended for stable results

### Performance Claims
Performance claims are based on:
1. **Absolute Measurements**: capitan overhead in isolation
2. **Fair Comparisons**: Against equivalent traditional implementations
3. **Realistic Workloads**: Representative event patterns and field usage
4. **Documented Conditions**: Clear measurement environment and methodology

## Benchmark Data Analysis

### Interpreting Results
```
BenchmarkEmit-8    2000000    750 ns/op    128 B/op    2 allocs/op
```
- **Name**: BenchmarkEmit-8 (8 CPU cores used)
- **Iterations**: 2,000,000 iterations run
- **Time per op**: 750 nanoseconds per operation
- **Memory per op**: 128 bytes allocated per operation
- **Allocations per op**: 2 allocations per operation

### Performance Regression Detection
Use `benchcmp` to compare results over time:
```bash
# Save baseline
go test -bench=. ./testing/benchmarks/... > baseline.txt

# After changes
go test -bench=. ./testing/benchmarks/... > current.txt

# Compare
benchcmp baseline.txt current.txt
```

### Statistical Significance
Run multiple iterations for stable results:
```bash
# Run 10 times and analyze variance
go test -bench=BenchmarkSpecific -count=10 ./testing/benchmarks/... | tee results.txt
```

## Benchmark Environment

### Requirements
- Go 1.23+ for testing features
- Stable system load (avoid running during builds/deployments)
- Consistent CPU frequency (disable CPU scaling if needed)
- Sufficient RAM to avoid swapping

### System Configuration
For reproducible benchmarks:
```bash
# Set CPU governor to performance mode
echo performance | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# Disable CPU turbo boost
echo 1 | sudo tee /sys/devices/system/cpu/intel_pstate/no_turbo

# Set process priority
nice -n -10 go test -bench=...
```

## Custom Benchmarks

### Adding New Benchmarks
Follow these patterns when adding benchmarks:

1. **Benchmark naming**: `BenchmarkCategory_Specific`
2. **Setup/teardown**: Use `b.ResetTimer()` after setup
3. **Loop structure**: Standard `for i := 0; i < b.N; i++` loop
4. **Error handling**: Use `b.Fatal(err)` for setup errors
5. **Memory measurement**: Include `-benchmem` for relevant benchmarks

### Example Benchmark Structure
```go
func BenchmarkEmit_SingleField(b *testing.B) {
    // Setup (not measured)
    c := capitan.New()
    defer c.Shutdown()

    sig := capitan.NewSignal("test", "Test signal")
    key := capitan.NewStringKey("key")

    c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
        // Minimal listener
    })

    b.ResetTimer() // Start timing here
    b.ReportAllocs() // Include allocation metrics

    for i := 0; i < b.N; i++ {
        capitan.Emit(context.Background(), sig, key.Field("value"))
    }
}
```

## Performance Optimization Guidelines

### Hot Path Optimization
1. **Event pooling eliminates allocations**
2. **Minimize allocations in listener callbacks**
3. **Use value types for fields where possible**
4. **Avoid string concatenation in hot paths**
5. **Cache frequently accessed field values**

### Concurrent Performance
1. **Per-signal workers prevent contention**
2. **Use buffered channels for backpressure**
3. **Minimize lock duration in listeners**
4. **Prefer RWMutex for read-heavy workloads**

### Memory Management
1. **Event pooling reduces GC pressure**
2. **Reuse field slices where possible**
3. **Avoid unbounded signal creation**
4. **Call Shutdown() to release resources**

## Continuous Integration

### Automated Benchmarking
Include benchmark runs in CI:
```yaml
# Example GitHub Actions step
- name: Run benchmarks
  run: |
    go test -bench=. -benchmem ./testing/benchmarks/... > benchmarks.txt
    cat benchmarks.txt
```

### Performance Alerts
Set up alerts for performance regressions:
- **CPU time increase**: > 10% slower
- **Memory usage increase**: > 20% more allocations
- **Allocation count increase**: Any increase in zero-allocation paths

### Historical Tracking
Track performance metrics over time:
- Store benchmark results in artifact storage
- Graph performance trends
- Correlate with code changes

---

The benchmark suite ensures capitan maintains excellent performance characteristics while providing comprehensive measurement tools for optimization work.
