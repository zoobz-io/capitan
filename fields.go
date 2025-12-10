package capitan

import "time"

// GenericKey is a Key implementation for any type T.
// All built-in key types (StringKey, IntKey, etc.) are aliases of GenericKey[T].
type GenericKey[T any] struct {
	name    string
	variant Variant
}

// Name returns the semantic identifier.
func (k GenericKey[T]) Name() string { return k.name }

// Variant returns the type constraint.
func (k GenericKey[T]) Variant() Variant { return k.variant }

// Field creates a GenericField with this key and the given value.
func (k GenericKey[T]) Field(value T) Field {
	return GenericField[T]{key: k, value: value, variant: k.variant}
}

// From extracts the typed value for this key from the event.
// Returns the value and true if present, or zero value and false if not present or wrong type.
func (k GenericKey[T]) From(e *Event) (T, bool) {
	var zero T
	f := e.Get(k)
	if f == nil {
		return zero, false
	}
	if gf, ok := f.(GenericField[T]); ok {
		return gf.Get(), true
	}
	return zero, false
}

// ExtractFromFields extracts the typed value for this key from a field slice.
// Returns the value if present, or the zero value if not found or wrong type.
//
// This method is primarily useful in tests when working with captured events,
// where you have a []Field slice rather than an *Event pointer.
//
// Example:
//
//	capture := capitantesting.NewEventCapture()
//	c.Hook(signal, capture.Handler())
//	// ... emit events ...
//	events := capture.Events()
//	userID := userKey.ExtractFromFields(events[0].Fields)
func (k GenericKey[T]) ExtractFromFields(fields []Field) T {
	var zero T
	for _, f := range fields {
		if f.Key().Name() == k.name {
			if gf, ok := f.(GenericField[T]); ok {
				return gf.Get()
			}
		}
	}
	return zero
}

// NewKey creates a GenericKey for any type T with the given name and variant.
// Use a namespaced variant string to avoid collisions (e.g., "myapp.OrderInfo").
//
// Example:
//
//	type OrderInfo struct { ID string; Total float64 }
//	orderKey := capitan.NewKey[OrderInfo]("order", "myapp.OrderInfo")
//	capitan.Emit(sig, orderKey.Field(OrderInfo{ID: "123", Total: 99.99}))
func NewKey[T any](name string, variant Variant) GenericKey[T] {
	return GenericKey[T]{name: name, variant: variant}
}

// StringKey is a Key implementation for string values.
type StringKey = GenericKey[string]

// NewStringKey creates a StringKey with the given name.
func NewStringKey(name string) StringKey {
	return GenericKey[string]{name: name, variant: VariantString}
}

// IntKey is a Key implementation for int values.
type IntKey = GenericKey[int]

// NewIntKey creates an IntKey with the given name.
func NewIntKey(name string) IntKey {
	return GenericKey[int]{name: name, variant: VariantInt}
}

// Int32Key is a Key implementation for int32 values.
type Int32Key = GenericKey[int32]

// NewInt32Key creates an Int32Key with the given name.
func NewInt32Key(name string) Int32Key {
	return GenericKey[int32]{name: name, variant: VariantInt32}
}

// Int64Key is a Key implementation for int64 values.
type Int64Key = GenericKey[int64]

// NewInt64Key creates an Int64Key with the given name.
func NewInt64Key(name string) Int64Key {
	return GenericKey[int64]{name: name, variant: VariantInt64}
}

// UintKey is a Key implementation for uint values.
type UintKey = GenericKey[uint]

// NewUintKey creates a UintKey with the given name.
func NewUintKey(name string) UintKey {
	return GenericKey[uint]{name: name, variant: VariantUint}
}

// Uint32Key is a Key implementation for uint32 values.
type Uint32Key = GenericKey[uint32]

// NewUint32Key creates a Uint32Key with the given name.
func NewUint32Key(name string) Uint32Key {
	return GenericKey[uint32]{name: name, variant: VariantUint32}
}

// Uint64Key is a Key implementation for uint64 values.
type Uint64Key = GenericKey[uint64]

// NewUint64Key creates a Uint64Key with the given name.
func NewUint64Key(name string) Uint64Key {
	return GenericKey[uint64]{name: name, variant: VariantUint64}
}

// Float32Key is a Key implementation for float32 values.
type Float32Key = GenericKey[float32]

// NewFloat32Key creates a Float32Key with the given name.
func NewFloat32Key(name string) Float32Key {
	return GenericKey[float32]{name: name, variant: VariantFloat32}
}

// Float64Key is a Key implementation for float64 values.
type Float64Key = GenericKey[float64]

// NewFloat64Key creates a Float64Key with the given name.
func NewFloat64Key(name string) Float64Key {
	return GenericKey[float64]{name: name, variant: VariantFloat64}
}

// BoolKey is a Key implementation for bool values.
type BoolKey = GenericKey[bool]

// NewBoolKey creates a BoolKey with the given name.
func NewBoolKey(name string) BoolKey {
	return GenericKey[bool]{name: name, variant: VariantBool}
}

// TimeKey is a Key implementation for time.Time values.
type TimeKey = GenericKey[time.Time]

// NewTimeKey creates a TimeKey with the given name.
func NewTimeKey(name string) TimeKey {
	return GenericKey[time.Time]{name: name, variant: VariantTime}
}

// DurationKey is a Key implementation for time.Duration values.
type DurationKey = GenericKey[time.Duration]

// NewDurationKey creates a DurationKey with the given name.
func NewDurationKey(name string) DurationKey {
	return GenericKey[time.Duration]{name: name, variant: VariantDuration}
}

// BytesKey is a Key implementation for []byte values.
type BytesKey = GenericKey[[]byte]

// NewBytesKey creates a BytesKey with the given name.
func NewBytesKey(name string) BytesKey {
	return GenericKey[[]byte]{name: name, variant: VariantBytes}
}

// ErrorKey is a Key implementation for error values.
type ErrorKey = GenericKey[error]

// NewErrorKey creates an ErrorKey with the given name.
func NewErrorKey(name string) ErrorKey {
	return GenericKey[error]{name: name, variant: VariantError}
}
