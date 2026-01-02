package capitan

import (
	"context"
	"testing"
	"time"
)

const testValue = "test"

func TestSignalNameAndDescription(t *testing.T) {
	sig := NewSignal("test.signal", "Test signal description")

	if sig.Name() != "test.signal" {
		t.Errorf("expected name %q, got %q", "test.signal", sig.Name())
	}

	if sig.Description() != "Test signal description" {
		t.Errorf("expected description %q, got %q", "Test signal description", sig.Description())
	}
}

func TestEventGet(t *testing.T) {
	sig := NewSignal("test.get", "Test get signal")
	strKey := NewStringKey("name")
	intKey := NewIntKey("count")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), strKey.Field(testValue), intKey.Field(42))

	nameField := event.Get(strKey)
	if nameField == nil {
		t.Fatal("name field not found")
	}

	sf, ok := nameField.(GenericField[string])
	if !ok {
		t.Fatal("name field wrong type")
	}
	if sf.Get() != testValue {
		t.Errorf("expected %q, got %q", testValue, sf.Get())
	}

	countField := event.Get(intKey)
	if countField == nil {
		t.Fatal("count field not found")
	}

	cf, ok := countField.(GenericField[int])
	if !ok {
		t.Fatal("count field wrong type")
	}
	if cf.Get() != 42 {
		t.Errorf("expected %d, got %d", 42, cf.Get())
	}
}

func TestEventGetMissingKey(t *testing.T) {
	sig := NewSignal("test.missing", "Test missing key signal")
	strKey := NewStringKey("existing")
	missingKey := NewStringKey("missing")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), strKey.Field(testValue))

	field := event.Get(missingKey)
	if field != nil {
		t.Errorf("expected nil for missing key, got %v", field)
	}
}

func TestEventFields(t *testing.T) {
	sig := NewSignal("test.fields", "Test fields signal")
	strKey := NewStringKey("name")
	intKey := NewIntKey("count")
	boolKey := NewBoolKey("active")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(),
		strKey.Field(testValue),
		intKey.Field(42),
		boolKey.Field(true),
	)

	fields := event.Fields()

	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	// Check all fields present (order not guaranteed)
	foundStr, foundInt, foundBool := false, false, false
	for _, field := range fields {
		switch f := field.(type) {
		case GenericField[string]:
			if f.Get() == testValue {
				foundStr = true
			}
		case GenericField[int]:
			if f.Get() == 42 {
				foundInt = true
			}
		case GenericField[bool]:
			if f.Get() == true {
				foundBool = true
			}
		}
	}

	if !foundStr {
		t.Error("string field not found in Fields()")
	}
	if !foundInt {
		t.Error("int field not found in Fields()")
	}
	if !foundBool {
		t.Error("bool field not found in Fields()")
	}
}

func TestEventSignal(t *testing.T) {
	sig := NewSignal("test.signal", "Test signal")
	key := NewStringKey("value")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field(testValue))

	if event.Signal() != sig {
		t.Errorf("expected signal %q, got %q", sig, event.Signal())
	}
}

func TestEventTimestamp(t *testing.T) {
	sig := NewSignal("test.timestamp", "Test timestamp signal")
	key := NewStringKey("value")

	before := time.Now()
	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field(testValue))
	after := time.Now()

	if event.Timestamp().Before(before) || event.Timestamp().After(after) {
		t.Errorf("timestamp %v not between %v and %v", event.Timestamp(), before, after)
	}
}

func TestEventPooling(t *testing.T) {
	sig := NewSignal("test.pool", "Test pooling signal")
	key := NewStringKey("value")

	// Create first event with "first" value
	event1 := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field("first"))
	field1 := event1.Get(key).(GenericField[string])
	if field1.Get() != "first" {
		t.Errorf("event1: expected %q, got %q", "first", field1.Get())
	}

	// Return to pool
	eventPool.Put(event1)

	// Create second event with "second" value
	event2 := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field("second"))
	field2 := event2.Get(key).(GenericField[string])
	if field2.Get() != "second" {
		t.Errorf("event2: expected %q, got %q - pool not cleared properly", "second", field2.Get())
	}

	// Verify field was cleared (whether pooled or not, fields should be correct)
	if field2.Get() == "first" {
		t.Error("fields not cleared when reusing pooled event")
	}
}

func TestEventFieldsDefensiveCopy(t *testing.T) {
	sig := NewSignal("test.defensive", "Test defensive copy signal")
	key := NewStringKey("value")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field(testValue))

	fields1 := event.Fields()
	fields2 := event.Fields()

	// Should be different slices
	if &fields1[0] == &fields2[0] {
		t.Error("Fields() not returning defensive copy")
	}
}

func TestEventContext(t *testing.T) {
	sig := NewSignal("test.event.context", "Test event context signal")
	key := NewStringKey("value")

	type ctxKey string
	const testKey ctxKey = "test_key"
	expectedValue := "test_value"

	ctx := context.WithValue(context.Background(), testKey, expectedValue)
	event := newEvent(ctx, sig, SeverityInfo, time.Now(), key.Field(testValue))

	eventCtx := event.Context()
	if eventCtx == nil {
		t.Fatal("Event.Context() returned nil")
	}

	receivedValue := eventCtx.Value(testKey)
	if receivedValue != expectedValue {
		t.Errorf("expected context value %q, got %v", expectedValue, receivedValue)
	}
}

func TestEventContextBackground(t *testing.T) {
	sig := NewSignal("test.background", "Test background context signal")
	key := NewStringKey("value")

	event := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field(testValue))

	ctx := event.Context()
	if ctx == nil {
		t.Fatal("Event.Context() returned nil")
	}

	if ctx.Err() != nil {
		t.Errorf("background context should not have error, got %v", ctx.Err())
	}
}

func TestEventSeverity(t *testing.T) {
	sig := NewSignal("test.severity", "Test severity signal")
	key := NewStringKey("value")

	tests := []struct {
		name     string
		severity Severity
	}{
		{"Debug", SeverityDebug},
		{"Info", SeverityInfo},
		{"Warn", SeverityWarn},
		{"Error", SeverityError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newEvent(context.Background(), sig, tt.severity, time.Now(), key.Field(testValue))
			if event.Severity() != tt.severity {
				t.Errorf("expected severity %q, got %q", tt.severity, event.Severity())
			}
		})
	}
}

func TestEventDefaultSeverity(t *testing.T) {
	sig := NewSignal("test.default", "Test default severity signal")
	key := NewStringKey("value")

	// When using Emit(), severity should default to Info
	c := New()
	var receivedSeverity Severity
	c.Hook(sig, func(_ context.Context, e *Event) {
		receivedSeverity = e.Severity()
	})

	c.Emit(context.Background(), sig, key.Field(testValue))
	c.Shutdown()

	if receivedSeverity != SeverityInfo {
		t.Errorf("expected default severity %q, got %q", SeverityInfo, receivedSeverity)
	}
}

func TestSeverityMethods(t *testing.T) {
	sig := NewSignal("test.severity.methods", "Test severity methods signal")
	key := NewStringKey("value")

	tests := []struct {
		name     string
		emitFunc func(*Capitan, context.Context, Signal, ...Field)
		expected Severity
	}{
		{"Debug", (*Capitan).Debug, SeverityDebug},
		{"Info", (*Capitan).Info, SeverityInfo},
		{"Warn", (*Capitan).Warn, SeverityWarn},
		{"Error", (*Capitan).Error, SeverityError},
		{"Emit", (*Capitan).Emit, SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			var receivedSeverity Severity
			c.Hook(sig, func(_ context.Context, e *Event) {
				receivedSeverity = e.Severity()
			})

			tt.emitFunc(c, context.Background(), sig, key.Field(testValue))
			c.Shutdown()

			if receivedSeverity != tt.expected {
				t.Errorf("expected severity %q, got %q", tt.expected, receivedSeverity)
			}
		})
	}
}

func TestNewEvent(t *testing.T) {
	sig := NewSignal("test.newevent", "Test NewEvent signal")
	orderID := NewStringKey("order_id")
	total := NewFloat64Key("total")

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	e := NewEvent(sig, SeverityWarn, timestamp,
		orderID.Field("ORD-123"),
		total.Field(99.99),
	)

	if e.Signal() != sig {
		t.Errorf("expected signal %v, got %v", sig, e.Signal())
	}

	if e.Severity() != SeverityWarn {
		t.Errorf("expected severity %v, got %v", SeverityWarn, e.Severity())
	}

	if !e.Timestamp().Equal(timestamp) {
		t.Errorf("expected timestamp %v, got %v", timestamp, e.Timestamp())
	}

	id, ok := orderID.From(e)
	if !ok {
		t.Fatal("order_id field not found")
	}
	if id != "ORD-123" {
		t.Errorf("expected order_id %q, got %q", "ORD-123", id)
	}

	amt, ok := total.From(e)
	if !ok {
		t.Fatal("total field not found")
	}
	if amt != 99.99 {
		t.Errorf("expected total %v, got %v", 99.99, amt)
	}
}

func TestNewEventNotPooled(t *testing.T) {
	sig := NewSignal("test.newevent.notpooled", "Test NewEvent not pooled signal")
	key := NewStringKey("value")

	// NewEvent should not be pooled - safe to hold reference
	e := NewEvent(sig, SeverityInfo, time.Now(), key.Field(testValue))

	// Verify IsReplay is false initially (set to true by Replay)
	if e.IsReplay() {
		t.Error("new event should not be marked as replay before Replay() is called")
	}
}

func TestIsReplay(t *testing.T) {
	sig := NewSignal("test.isreplay", "Test IsReplay signal")
	key := NewStringKey("value")

	// Regular emitted events should not be replays
	c := New(WithSyncMode())
	defer c.Shutdown()

	var emittedEvent *Event
	c.Hook(sig, func(_ context.Context, e *Event) {
		emittedEvent = e
	})

	c.Emit(context.Background(), sig, key.Field(testValue))

	if emittedEvent == nil {
		t.Fatal("event not received")
	}
	if emittedEvent.IsReplay() {
		t.Error("emitted event should not be marked as replay")
	}
}

func TestEventClone(t *testing.T) {
	sig := NewSignal("test.clone", "Test clone signal")
	strKey := NewStringKey("name")
	intKey := NewIntKey("count")

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	original := newEvent(context.Background(), sig, SeverityWarn, timestamp,
		strKey.Field(testValue),
		intKey.Field(42),
	)

	clone := original.Clone()

	// Verify all fields are copied
	if clone.Signal() != original.Signal() {
		t.Errorf("signal mismatch: expected %v, got %v", original.Signal(), clone.Signal())
	}
	if clone.Severity() != original.Severity() {
		t.Errorf("severity mismatch: expected %v, got %v", original.Severity(), clone.Severity())
	}
	if !clone.Timestamp().Equal(original.Timestamp()) {
		t.Errorf("timestamp mismatch: expected %v, got %v", original.Timestamp(), clone.Timestamp())
	}
	if clone.Context() != original.Context() {
		t.Errorf("context mismatch")
	}
	if clone.IsReplay() != original.IsReplay() {
		t.Errorf("replay mismatch: expected %v, got %v", original.IsReplay(), clone.IsReplay())
	}

	// Verify field values
	strVal, ok := strKey.From(clone)
	if !ok || strVal != testValue {
		t.Errorf("string field mismatch: expected 'test', got '%s'", strVal)
	}
	intVal, ok := intKey.From(clone)
	if !ok || intVal != 42 {
		t.Errorf("int field mismatch: expected 42, got %d", intVal)
	}

	// Verify clone is independent (different map instance)
	if len(clone.Fields()) != len(original.Fields()) {
		t.Errorf("fields count mismatch: expected %d, got %d", len(original.Fields()), len(clone.Fields()))
	}
}

func TestEventCloneIndependence(t *testing.T) {
	sig := NewSignal("test.clone.independence", "Test clone independence signal")
	key := NewStringKey("value")

	original := newEvent(context.Background(), sig, SeverityInfo, time.Now(), key.Field("original"))
	clone := original.Clone()

	// Return original to pool
	eventPool.Put(original)

	// Clone should still be valid
	val, ok := key.From(clone)
	if !ok {
		t.Fatal("clone field not found after original returned to pool")
	}
	if val != "original" {
		t.Errorf("expected 'original', got '%s'", val)
	}
}

func TestEventCloneReplay(t *testing.T) {
	sig := NewSignal("test.clone.replay", "Test clone replay signal")
	key := NewStringKey("value")

	original := NewEvent(sig, SeverityInfo, time.Now(), key.Field(testValue))
	original.replay = true

	clone := original.Clone()

	if !clone.IsReplay() {
		t.Error("clone should preserve replay flag")
	}
}
