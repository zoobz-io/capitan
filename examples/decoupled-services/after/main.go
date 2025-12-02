//go:build ignore

// Decoupled Services - AFTER
// ===========================
// Using capitan for event-driven decoupling between services.
//
// Improvements demonstrated:
// - Loose coupling: Orders package only imports capitan, not consumers
// - Adding a new consumer requires NO changes to Orders package
// - No circular dependency risk - events flow one direction
// - Easy to test: just verify the right events are emitted
// - Consumers can process in parallel (see async-notifications example)
//
// Run: go run .

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/capitan"
)

// --- Events package (shared) ---
// In reality: events/events.go
// This is the ONLY shared dependency between services.

// Signals define what events exist in the system.
var (
	OrderCreated = capitan.NewSignal("order.created", "New order has been created")
)

// Keys define the data shape for events.
var (
	orderID    = capitan.NewStringKey("order_id")
	customerID = capitan.NewStringKey("customer_id")
	total      = capitan.NewFloat64Key("total")
	itemCount  = capitan.NewIntKey("item_count")
)

// Order represents an e-commerce order.
type Order struct {
	ID         string
	CustomerID string
	Total      float64
	Items      int
}

func main() {
	fmt.Println("=== AFTER: Event-Driven Decoupling ===")
	fmt.Println()

	// Each service registers its own listener.
	// The Orders package doesn't know these exist!
	setupNotifications()
	setupAnalytics()
	setupInventory()
	setupLoyalty()

	// Process an order - only knows about events, not consumers
	order := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-789",
		Total:      299.99,
		Items:      3,
	}

	ProcessOrder(order)

	// Wait for async processing
	capitan.Shutdown()

	fmt.Println()
	fmt.Println("Improvements with this approach:")
	fmt.Println("- ProcessOrder only imports: capitan (and shared events)")
	fmt.Println("- Adding 'fraud detection' = new Hook, zero changes to Orders")
	fmt.Println("- Testing ProcessOrder = verify event emitted, no mocks needed")
	fmt.Println("- Services are isolated - easy to reason about")
	fmt.Println("- No import cycles possible - events flow one direction")
}

// ProcessOrder handles an order by emitting an event.
// It has NO knowledge of what services consume this event.
func ProcessOrder(order Order) {
	start := time.Now()
	fmt.Printf("Processing order %s...\n", order.ID)

	// Emit event - that's it!
	// No imports of notifications, analytics, inventory, loyalty...
	capitan.Emit(context.Background(), OrderCreated,
		orderID.Field(order.ID),
		customerID.Field(order.CustomerID),
		total.Field(order.Total),
		itemCount.Field(order.Items),
	)

	fmt.Printf("Order %s emitted in %v (listeners process async)\n", order.ID, time.Since(start))

	// Adding fraud detection? Just add a Hook somewhere else.
	// Adding shipping integration? Just add a Hook somewhere else.
	// ProcessOrder never changes.
}

// --- These would be in separate packages in a real system ---

// setupNotifications registers notification handlers (notifications/notifications.go).
func setupNotifications() {
	capitan.Hook(OrderCreated, func(_ context.Context, e *capitan.Event) {
		id, _ := orderID.From(e)
		time.Sleep(500 * time.Millisecond) // Slow external service
		fmt.Printf("  [notifications] Sent confirmation for order %s\n", id)
	})
}

// setupAnalytics registers analytics handlers (analytics/analytics.go).
func setupAnalytics() {
	capitan.Hook(OrderCreated, func(_ context.Context, e *capitan.Event) {
		id, _ := orderID.From(e)
		amount, _ := total.From(e)
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("  [analytics] Tracked order %s ($%.2f)\n", id, amount)
	})
}

// setupInventory registers inventory handlers (inventory/inventory.go).
func setupInventory() {
	capitan.Hook(OrderCreated, func(_ context.Context, e *capitan.Event) {
		id, _ := orderID.From(e)
		items, _ := itemCount.From(e)
		time.Sleep(50 * time.Millisecond)
		fmt.Printf("  [inventory] Reserved %d items for order %s\n", items, id)
	})
}

// setupLoyalty registers loyalty handlers (loyalty/loyalty.go).
func setupLoyalty() {
	capitan.Hook(OrderCreated, func(_ context.Context, e *capitan.Event) {
		id, _ := orderID.From(e)
		amount, _ := total.From(e)
		points := int(amount * 10)
		time.Sleep(50 * time.Millisecond)
		fmt.Printf("  [loyalty] Awarded %d points for order %s\n", points, id)
	})
}
