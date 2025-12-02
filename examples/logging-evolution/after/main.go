// Logging Evolution - AFTER
// ==========================
// Using capitan for structured, type-safe event logging.
//
// Improvements demonstrated:
// - Consistent event structure across the entire codebase
// - Typed fields - compile-time safety for event data
// - Single observer handles all logging - change format in one place
// - Severity levels built-in (Debug, Info, Warn, Error)
// - Easy to aggregate, filter, and analyze
// - Testable - mock the observer to verify events
//
// Run: go run .

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/capitan"
)

// Define signals as package-level constants.
// Signals are event types - they describe WHAT happened.
var (
	OrderCreated    = capitan.NewSignal("order.created", "New order has been created")
	PaymentComplete = capitan.NewSignal("payment.complete", "Payment successfully processed")
	HighValueOrder  = capitan.NewSignal("order.high_value", "High value order flagged for review")
	OrderShipped    = capitan.NewSignal("order.shipped", "Order has been shipped")
)

// Define typed keys for event fields.
// Keys are field names - they describe the DATA in events.
var (
	orderID    = capitan.NewStringKey("order_id")
	customerID = capitan.NewStringKey("customer_id")
	total      = capitan.NewFloat64Key("total")
	itemCount  = capitan.NewIntKey("item_count")
	method     = capitan.NewStringKey("payment_method")
)

func main() {
	fmt.Println("=== AFTER: Structured Event Logging ===")
	fmt.Println()

	// Single observer handles ALL logging - consistent format everywhere.
	// Change this one function to change logging across the entire app.
	capitan.Observe(func(_ context.Context, e *capitan.Event) {
		// Extract timestamp
		ts := e.Timestamp().Format("15:04:05.000")

		// Build structured log line
		fmt.Printf("%s [%s] %s", ts, e.Severity(), e.Signal().Description())

		// Append all fields in consistent format
		for _, field := range e.Fields() {
			fmt.Printf(" %s=%v", field.Key().Name(), field.Value())
		}
		fmt.Println()
	})

	// Simulate order flow - same business logic, better observability
	order := createOrder("CUST-123", 299.99, 3)
	processPayment(order)
	shipOrder(order)

	// Graceful shutdown - ensures all events are processed
	capitan.Shutdown()

	fmt.Println()
	fmt.Println("Improvements visible above:")
	fmt.Println("- Consistent format for ALL events")
	fmt.Println("- Structured fields - easy to parse and aggregate")
	fmt.Println("- Severity levels are standardized")
	fmt.Println("- Change logging format in ONE place (the observer)")
	fmt.Println("- Type-safe: orderID.Field(123) won't compile")
}

// Order represents an e-commerce order.
type Order struct {
	ID         string
	CustomerID string
	Total      float64
	Items      int
}

func createOrder(custID string, orderTotal float64, items int) Order {
	order := Order{
		ID:         fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		CustomerID: custID,
		Total:      orderTotal,
		Items:      items,
	}

	// Emit structured event - type-safe fields
	capitan.Info(context.Background(), OrderCreated,
		orderID.Field(order.ID),
		customerID.Field(order.CustomerID),
		total.Field(order.Total),
		itemCount.Field(order.Items),
	)

	return order
}

func processPayment(order Order) {
	time.Sleep(50 * time.Millisecond)

	// Payment complete event
	capitan.Info(context.Background(), PaymentComplete,
		orderID.Field(order.ID),
		total.Field(order.Total),
		method.Field("credit_card"),
	)

	// High value orders get a warning - same observer handles it
	if order.Total > 500 {
		capitan.Warn(context.Background(), HighValueOrder,
			orderID.Field(order.ID),
			total.Field(order.Total),
		)
	}
}

func shipOrder(order Order) {
	time.Sleep(30 * time.Millisecond)

	// Shipping event
	capitan.Info(context.Background(), OrderShipped,
		orderID.Field(order.ID),
		itemCount.Field(order.Items),
	)

	// Debug events can be filtered by severity in the observer
	capitan.Debug(context.Background(), OrderShipped,
		orderID.Field(order.ID),
	)
}
