//go:build ignore

// Async Notifications - AFTER
// ============================
// Using capitan for fire-and-forget event processing.
//
// Improvements demonstrated:
// - API response time = only critical path (payment)
// - Notifications happen async - user doesn't wait
// - Per-signal workers provide isolation
// - One slow service doesn't affect others
// - Panics in listeners don't crash the request
//
// Run: go run .

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/capitan"
)

// Events.
var (
	OrderPaid = capitan.NewSignal("order.paid", "Order payment completed")
)

// Keys.
var (
	orderIDKey = capitan.NewStringKey("order_id")
	emailKey   = capitan.NewStringKey("email")
	phoneKey   = capitan.NewStringKey("phone")
)

func main() {
	fmt.Println("=== AFTER: Async Event-Driven Notifications ===")
	fmt.Println()

	// Register notification handlers - they process async
	setupNotifications()

	// Simulate an API request
	start := time.Now()
	response := handleCheckout("ORD-12345", "user@example.com", "+1234567890")
	apiLatency := time.Since(start)

	fmt.Printf("\nAPI Response: %s\n", response)
	fmt.Printf("API latency: %v (user sees response NOW)\n", apiLatency)

	fmt.Println("\nNotifications processing in background...")

	// In a real server, you wouldn't shutdown - workers run for server lifetime.
	// Here we wait to show the async processing completing.
	capitan.Shutdown()

	fmt.Println()
	fmt.Println("Improvements with this approach:")
	fmt.Println("- User waited only ~50ms (payment only)")
	fmt.Println("- Notifications processed after response sent")
	fmt.Println("- Each notification type is isolated (per-signal workers)")
	fmt.Println("- Slow email doesn't block SMS or push")
	fmt.Println("- P99 latency = only critical path")
}

// handleCheckout processes checkout, emits event, returns immediately.
// User sees response in ~50ms, not 1.2+ seconds.
func handleCheckout(id, emailAddr, phoneNum string) string {
	fmt.Printf("Processing checkout for %s...\n", id)

	// Payment - this MUST be sync (critical path)
	processPayment(id)

	// Emit event - returns immediately!
	// All notifications happen async in background workers.
	capitan.Emit(context.Background(), OrderPaid,
		orderIDKey.Field(id),
		emailKey.Field(emailAddr),
		phoneKey.Field(phoneNum),
	)

	// User gets response NOW, notifications continue in background
	return fmt.Sprintf("Order %s confirmed", id)
}

func processPayment(id string) {
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  [payment] Charged for %s (50ms)\n", id)
}

// --- Notification handlers (process async) ---

func setupNotifications() {
	// Each Hook processes events in its signal's worker goroutine.
	// They run concurrently with each other and with the main request.

	capitan.Hook(OrderPaid, func(_ context.Context, e *capitan.Event) {
		id, _ := orderIDKey.From(e)
		addr, _ := emailKey.From(e)
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("  [email] Sent to %s for %s (500ms) - ASYNC\n", addr, id)
	})

	capitan.Hook(OrderPaid, func(_ context.Context, e *capitan.Event) {
		id, _ := orderIDKey.From(e)
		num, _ := phoneKey.From(e)
		time.Sleep(300 * time.Millisecond)
		fmt.Printf("  [sms] Sent to %s for %s (300ms) - ASYNC\n", num, id)
	})

	capitan.Hook(OrderPaid, func(_ context.Context, e *capitan.Event) {
		id, _ := orderIDKey.From(e)
		time.Sleep(200 * time.Millisecond)
		fmt.Printf("  [push] Sent for %s (200ms) - ASYNC\n", id)
	})

	capitan.Hook(OrderPaid, func(_ context.Context, e *capitan.Event) {
		id, _ := orderIDKey.From(e)
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("  [analytics] Tracked %s (100ms) - ASYNC\n", id)
	})

	capitan.Hook(OrderPaid, func(_ context.Context, e *capitan.Event) {
		id, _ := orderIDKey.From(e)
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("  [warehouse] Notified for %s (100ms) - ASYNC\n", id)
	})
}
