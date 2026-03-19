// Package capitan provides type-safe event coordination for Go with zero dependencies.
//
// At its core, capitan offers three operations: Emit events with typed fields,
// Hook listeners to specific signals, and Observe all signals. Events are processed
// asynchronously with per-signal worker goroutines for isolation and performance.
//
// Context Support: All events carry a context.Context for cancellation, timeouts,
// and request-scoped values. Canceled contexts prevent event queueing and processing.
//
// Quick example:
//
//	sig := capitan.NewSignal("order.created", "New order has been created")
//	orderID := capitan.NewStringKey("order_id")
//
//	capitan.Hook(sig, func(ctx context.Context, e *capitan.Event) {
//	    id, _ := orderID.From(e)
//	    // Process order...
//	})
//
//	capitan.Emit(context.Background(), sig, orderID.Field("ORDER-123"))
//	capitan.Shutdown() // Drain pending events
//
// See https://github.com/zoobz-io/capitan for full documentation.
package capitan

// Signal represents an event type identifier used for routing events to listeners.
type Signal struct {
	name        string
	description string
}

// NewSignal creates a new Signal with the given name and description.
// The description is used as the human-readable message when converting to logs.
func NewSignal(name, description string) Signal {
	return Signal{
		name:        name,
		description: description,
	}
}

// Name returns the signal's identifier.
func (s Signal) Name() string {
	return s.name
}

// Description returns the signal's human-readable description.
func (s Signal) Description() string {
	return s.description
}

// Severity represents the logging severity level of an event.
// Compatible with standard logging conventions and OpenTelemetry.
type Severity string

// Severity levels for events.
const (
	// SeverityDebug is for development and troubleshooting information.
	SeverityDebug Severity = "DEBUG"
	// SeverityInfo is for normal operational messages (default for Emit).
	SeverityInfo Severity = "INFO"
	// SeverityWarn is for warning conditions that may need attention.
	SeverityWarn Severity = "WARN"
	// SeverityError is for error conditions requiring immediate action.
	SeverityError Severity = "ERROR"
)

// severityOrder maps severity levels to numeric values for comparison.
var severityOrder = map[Severity]int{
	SeverityDebug: 0,
	SeverityInfo:  1,
	SeverityWarn:  2,
	SeverityError: 3,
}

// severityAtLeast returns true if s is at or above the minimum severity level.
func severityAtLeast(s, minLevel Severity) bool {
	return severityOrder[s] >= severityOrder[minLevel]
}

// Key represents a typed semantic identifier for a field.
// Each Key implementation is bound to a specific Variant, ensuring type safety.
type Key interface {
	// Name returns the semantic identifier for this key.
	Name() string

	// Variant returns the type constraint for this key.
	Variant() Variant
}

// Variant is a discriminator for the Field interface implementation type.
// Used for runtime type identification when type assertions are needed.
type Variant string

// Built-in variants for primitive types. Custom types should use namespaced
// variant strings (e.g., "myapp.OrderInfo") to avoid collisions.
const (
	VariantString   Variant = "string"
	VariantInt      Variant = "int"
	VariantInt32    Variant = "int32"
	VariantInt64    Variant = "int64"
	VariantUint     Variant = "uint"
	VariantUint32   Variant = "uint32"
	VariantUint64   Variant = "uint64"
	VariantFloat32  Variant = "float32"
	VariantFloat64  Variant = "float64"
	VariantBool     Variant = "bool"
	VariantTime     Variant = "time.Time"
	VariantDuration Variant = "time.Duration"
	VariantBytes    Variant = "[]byte"
	VariantError    Variant = "error"
)

// Field represents a typed value with semantic meaning in an Event.
// Library authors can implement custom Field types while maintaining type safety.
// Use type assertions to access concrete field types and their typed accessor methods.
type Field interface {
	// Variant returns the discriminator for this field's concrete type.
	Variant() Variant

	// Key returns the semantic identifier for this field.
	Key() Key

	// Value returns the underlying value as any.
	Value() any
}

// GenericField is a generic implementation of Field for typed values.
type GenericField[T any] struct {
	key     Key
	value   T
	variant Variant
}

// Variant returns the discriminator for this field's type.
func (f GenericField[T]) Variant() Variant { return f.variant }

// Key returns the semantic identifier for this field.
func (f GenericField[T]) Key() Key { return f.key }

// Value returns the underlying value as any.
func (f GenericField[T]) Value() any { return f.value }

// Get returns the typed value.
func (f GenericField[T]) Get() T { return f.value }

// workerState manages the lifecycle of a signal's worker goroutine.
type workerState struct {
	events  chan *Event        // buffered channel for queuing events
	done    chan struct{}      // signals worker to drain and exit
	markers chan chan struct{} // synchronization points for drain-on-close

	// Per-signal configuration (resolved once at worker creation)
	config      SignalConfig
	rateLimiter rateLimiter

	// Cached listener slice to avoid allocation on every event
	cachedListeners []*Listener
	listenerVersion uint64
}

// rateLimiter implements a token bucket for rate limiting events.
type rateLimiter struct {
	tokens    float64
	lastCheck int64 // Unix nano timestamp
}

// Stats provides runtime metrics for a Capitan instance.
type Stats struct {
	// ActiveWorkers is the number of worker goroutines currently running.
	ActiveWorkers int

	// SignalCount is the total number of unique signals that have been registered.
	SignalCount int

	// DroppedEvents is the total number of events dropped due to no listeners.
	DroppedEvents uint64

	// QueueDepths maps each signal to the number of events queued in its buffer.
	QueueDepths map[Signal]int

	// ListenerCounts maps each signal to the number of registered listeners.
	ListenerCounts map[Signal]int

	// EmitCounts maps each signal to the total number of times it has been emitted.
	EmitCounts map[Signal]uint64

	// FieldSchemas maps each signal to the keys of fields from its first emission.
	// This provides a schema of what fields are available on each signal.
	FieldSchemas map[Signal][]Key
}
