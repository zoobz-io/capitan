package benchmarks

import (
	"context"
	"testing"

	"github.com/zoobzio/capitan"
)

// BenchmarkEmit measures the performance of event emission.
func BenchmarkEmit(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.emit", "Test emit signal")
	key := capitan.NewStringKey("key")

	// Hook a minimal listener
	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		// Minimal processing
	})

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Emit(ctx, sig, key.Field("value"))
	}
}

// BenchmarkEmit_SyncMode measures synchronous emission performance.
func BenchmarkEmit_SyncMode(b *testing.B) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.sync", "Test sync signal")
	key := capitan.NewStringKey("key")

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		// Minimal processing
	})

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Emit(ctx, sig, key.Field("value"))
	}
}

// BenchmarkEmit_MultipleFields measures emission with various field counts.
func BenchmarkEmit_MultipleFields(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.fields", "Test fields signal")
	strKey := capitan.NewStringKey("string")
	intKey := capitan.NewIntKey("int")
	boolKey := capitan.NewBoolKey("bool")
	floatKey := capitan.NewFloat64Key("float")

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

	ctx := context.Background()

	b.Run("1_field", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Emit(ctx, sig, strKey.Field("value"))
		}
	})

	b.Run("4_fields", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Emit(ctx, sig,
				strKey.Field("value"),
				intKey.Field(42),
				boolKey.Field(true),
				floatKey.Field(3.14),
			)
		}
	})

	b.Run("8_fields", func(b *testing.B) {
		key1 := capitan.NewStringKey("key1")
		key2 := capitan.NewStringKey("key2")
		key3 := capitan.NewStringKey("key3")
		key4 := capitan.NewStringKey("key4")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Emit(ctx, sig,
				strKey.Field("value"),
				intKey.Field(42),
				boolKey.Field(true),
				floatKey.Field(3.14),
				key1.Field("a"),
				key2.Field("b"),
				key3.Field("c"),
				key4.Field("d"),
			)
		}
	})
}

// BenchmarkHook measures the performance of hooking listeners.
func BenchmarkHook(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.hook", "Test hook signal")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		listener := c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})
		listener.Close()
	}
}

// BenchmarkObserve measures the performance of observer creation.
func BenchmarkObserve(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	// Create some signals
	for i := 0; i < 10; i++ {
		sig := capitan.NewSignal("test.observe."+string(rune('a'+i)), "Test signal")
		c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		observer := c.Observe(func(_ context.Context, _ *capitan.Event) {})
		observer.Close()
	}
}

// BenchmarkFieldExtraction measures field access performance.
func BenchmarkFieldExtraction(b *testing.B) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.extract", "Test extract signal")
	strKey := capitan.NewStringKey("string")
	intKey := capitan.NewIntKey("int")
	boolKey := capitan.NewBoolKey("bool")

	var result string
	var count int
	var flag bool

	c.Hook(sig, func(_ context.Context, e *capitan.Event) {
		result, _ = strKey.From(e)
		count, _ = intKey.From(e)
		flag, _ = boolKey.From(e)
	})

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Emit(ctx, sig,
			strKey.Field("value"),
			intKey.Field(42),
			boolKey.Field(true),
		)
	}

	// Prevent optimization
	_ = result
	_ = count
	_ = flag
}

// BenchmarkListenerInvocation measures listener callback overhead.
func BenchmarkListenerInvocation(b *testing.B) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.listener", "Test listener signal")
	key := capitan.NewStringKey("key")

	b.Run("1_listener", func(b *testing.B) {
		c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			c.Emit(ctx, sig, key.Field("value"))
		}
	})

	b.Run("5_listeners", func(b *testing.B) {
		for i := 0; i < 4; i++ {
			c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})
		}

		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			c.Emit(ctx, sig, key.Field("value"))
		}
	})
}

// BenchmarkSeverityMethods measures severity helper method overhead.
func BenchmarkSeverityMethods(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.severity", "Test severity signal")
	key := capitan.NewStringKey("key")

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

	ctx := context.Background()

	b.Run("Debug", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Debug(ctx, sig, key.Field("value"))
		}
	})

	b.Run("Info", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Info(ctx, sig, key.Field("value"))
		}
	})

	b.Run("Warn", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Warn(ctx, sig, key.Field("value"))
		}
	})

	b.Run("Error", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c.Error(ctx, sig, key.Field("value"))
		}
	})
}

// BenchmarkStats measures stats collection overhead.
func BenchmarkStats(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	// Create some signals
	for i := 0; i < 10; i++ {
		sig := capitan.NewSignal("test.stats."+string(rune('a'+i)), "Test signal")
		c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})
		c.Emit(context.Background(), sig, capitan.NewStringKey("key").Field("value"))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stats := c.Stats()
		_ = stats
	}
}

// BenchmarkWorkerCreation measures lazy worker initialization cost.
func BenchmarkWorkerCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c := capitan.New()

		sig := capitan.NewSignal("test.worker", "Test worker signal")
		c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

		// First emit triggers worker creation
		c.Emit(context.Background(), sig, capitan.NewStringKey("key").Field("value"))

		c.Shutdown()
	}
}

// BenchmarkContextCancellation measures context cancellation handling.
func BenchmarkContextCancellation(b *testing.B) {
	c := capitan.New()
	defer c.Shutdown()

	sig := capitan.NewSignal("test.cancel", "Test cancel signal")
	key := capitan.NewStringKey("key")

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		c.Emit(ctx, sig, key.Field("value"))
	}
}

// BenchmarkPanicRecovery measures panic recovery overhead.
func BenchmarkPanicRecovery(b *testing.B) {
	c := capitan.New(capitan.WithSyncMode())
	defer c.Shutdown()

	sig := capitan.NewSignal("test.panic", "Test panic signal")
	key := capitan.NewStringKey("key")

	c.Hook(sig, func(_ context.Context, _ *capitan.Event) {
		panic("test panic")
	})

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Emit(ctx, sig, key.Field("value"))
	}
}
