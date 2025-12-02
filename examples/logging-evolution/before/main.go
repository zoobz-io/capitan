//go:build ignore

// Logging Evolution - BEFORE
// ===========================
// The typical approach: scattered log.Printf calls throughout the codebase.
//
// Problems demonstrated:
// - Inconsistent log formats across the codebase
// - No structured data - hard to parse and aggregate
// - Difficult to change logging behavior globally
// - No type safety - easy to log wrong types
// - Hard to test what gets logged
//
// Run: go run .

package main

import (
	"fmt"
	"log"
	"time"
)

// Order represents an e-commerce order.
type Order struct {
	ID         string
	CustomerID string
	Total      float64
	Items      int
}

// Payment represents a payment attempt.
type Payment struct {
	OrderID   string
	Amount    float64
	Method    string
	Succeeded bool
}

func main() {
	log.SetFlags(log.Ltime)

	fmt.Println("=== BEFORE: Scattered Logging ===")
	fmt.Println()

	// Simulate order flow
	order := createOrder("CUST-123", 299.99, 3)
	processPayment(order)
	shipOrder(order)

	fmt.Println()
	fmt.Println("Problems visible above:")
	fmt.Println("- Inconsistent formats (some use 'OrderID:', others use 'order=')")
	fmt.Println("- Mixed severity indicators ([INFO], [WARN], none)")
	fmt.Println("- No structured data - try grepping for all orders over $200")
	fmt.Println("- Scattered across files - changing format requires editing everywhere")
}

func createOrder(customerID string, total float64, items int) Order {
	order := Order{
		ID:         fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		CustomerID: customerID,
		Total:      total,
		Items:      items,
	}

	// Problem: Format differs from other log calls
	log.Printf("[INFO] Created order %s for customer %s, total: $%.2f",
		order.ID, order.CustomerID, order.Total)

	return order
}

func processPayment(order Order) {
	// Simulate payment processing
	time.Sleep(50 * time.Millisecond)

	// Problem: Different format again
	log.Printf("Payment processed - OrderID: %s, Amount: %.2f, Method: credit_card",
		order.ID, order.Total)

	// Problem: Warning format inconsistent
	if order.Total > 500 {
		log.Printf("[WARN] High value order %s flagged for review", order.ID)
	}
}

func shipOrder(order Order) {
	// Simulate shipping
	time.Sleep(30 * time.Millisecond)

	// Problem: Yet another format
	log.Printf("order=%s status=shipped items=%d", order.ID, order.Items)

	// Problem: Debug logs mixed with production logs
	log.Printf("DEBUG: shipOrder completed for %s at %v", order.ID, time.Now())
}
