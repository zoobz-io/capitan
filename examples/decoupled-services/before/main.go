//go:build ignore

// Decoupled Services - BEFORE
// ============================
// The typical approach: direct function calls between packages.
//
// Problems demonstrated:
// - Tight coupling: Orders package imports Notifications, Analytics, Inventory
// - Adding a new consumer requires modifying the Orders package
// - Circular dependency risk as the system grows
// - Hard to test Orders without mocking all dependencies
// - One slow service blocks everything
//
// Run: go run .

package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== BEFORE: Tightly Coupled Services ===")
	fmt.Println()

	// Create and process an order
	order := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-789",
		Total:      299.99,
		Items:      3,
	}

	// This function knows about ALL downstream services
	ProcessOrder(order)

	fmt.Println()
	fmt.Println("Problems with this approach:")
	fmt.Println("- ProcessOrder imports: notifications, analytics, inventory, loyalty")
	fmt.Println("- Adding 'fraud detection' means editing ProcessOrder")
	fmt.Println("- Testing ProcessOrder requires mocking 4+ packages")
	fmt.Println("- Notifications being slow (500ms) blocks the entire flow")
	fmt.Println("- Import cycles are lurking as complexity grows")
}

// Order represents an e-commerce order.
type Order struct {
	ID         string
	CustomerID string
	Total      float64
	Items      int
}

// ProcessOrder handles an order and notifies ALL dependent services.
// This function has to know about every service that cares about orders.
func ProcessOrder(order Order) {
	start := time.Now()
	fmt.Printf("Processing order %s...\n", order.ID)

	// Direct call to notifications - ProcessOrder must import this
	SendNotification(order)

	// Direct call to analytics - another import
	TrackAnalytics(order)

	// Direct call to inventory - another import
	UpdateInventory(order)

	// Direct call to loyalty - another import
	UpdateLoyaltyPoints(order)

	// Adding fraud detection? Edit this function AND add another import.
	// Adding shipping integration? Edit this function AND add another import.
	// Adding tax calculation? You get the idea...

	fmt.Printf("Order %s completed in %v\n", order.ID, time.Since(start))
}

// --- These would be in separate packages in a real system ---

// SendNotification sends order confirmation (notifications/notifications.go).
func SendNotification(order Order) {
	time.Sleep(500 * time.Millisecond) // Slow external service
	fmt.Printf("  [notifications] Sent confirmation for order %s\n", order.ID)
}

// TrackAnalytics records order metrics (analytics/analytics.go).
func TrackAnalytics(order Order) {
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("  [analytics] Tracked order %s ($%.2f)\n", order.ID, order.Total)
}

// UpdateInventory adjusts stock levels (inventory/inventory.go).
func UpdateInventory(order Order) {
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  [inventory] Reserved %d items for order %s\n", order.Items, order.ID)
}

// UpdateLoyaltyPoints awards points to customer (loyalty/loyalty.go).
func UpdateLoyaltyPoints(order Order) {
	points := int(order.Total * 10)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  [loyalty] Awarded %d points for order %s\n", points, order.ID)
}
