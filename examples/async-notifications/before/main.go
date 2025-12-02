// Async Notifications - BEFORE
// =============================
// The typical approach: synchronous calls blocking the API response.
//
// Problems demonstrated:
// - API response time = sum of ALL notification times
// - User waits for email, SMS, push, analytics... all to complete
// - One slow service degrades the entire user experience
// - Timeout in one service can cascade to request timeout
// - P99 latency is dominated by slowest downstream service
//
// Run: go run .

package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== BEFORE: Synchronous Notifications ===")
	fmt.Println()

	// Simulate an API request
	start := time.Now()
	response := handleCheckout("ORD-12345", "user@example.com", "+1234567890")
	apiLatency := time.Since(start)

	fmt.Printf("\nAPI Response: %s\n", response)
	fmt.Printf("Total API latency: %v\n", apiLatency)

	fmt.Println()
	fmt.Println("Problems with this approach:")
	fmt.Println("- User waited 1.2+ seconds for checkout to complete")
	fmt.Println("- Email service being slow (500ms) directly impacts UX")
	fmt.Println("- If SMS times out, the whole checkout could fail")
	fmt.Println("- P99 latency = worst case of ALL services combined")
}

// handleCheckout processes a checkout and sends ALL notifications synchronously.
// User must wait for everything to complete before seeing "order confirmed".
func handleCheckout(orderID, email, phone string) string {
	fmt.Printf("Processing checkout for %s...\n", orderID)
	processStart := time.Now()

	// Payment processing - this MUST be sync
	processPayment(orderID)

	// But why are these sync? User doesn't need to wait for them!
	sendEmail(email, orderID)     // 500ms - user waiting...
	sendSMS(phone, orderID)       // 300ms - still waiting...
	sendPushNotification(orderID) // 200ms - still waiting...
	updateAnalytics(orderID)      // 100ms - still waiting...
	notifyWarehouse(orderID)      // 100ms - finally done!

	fmt.Printf("All notifications complete in %v\n", time.Since(processStart))

	return fmt.Sprintf("Order %s confirmed", orderID)
}

func processPayment(orderID string) {
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  [payment] Charged for %s (50ms)\n", orderID)
}

func sendEmail(email, orderID string) {
	time.Sleep(500 * time.Millisecond) // Slow SMTP server
	fmt.Printf("  [email] Sent to %s for %s (500ms)\n", email, orderID)
}

func sendSMS(phone, orderID string) {
	time.Sleep(300 * time.Millisecond) // External SMS API
	fmt.Printf("  [sms] Sent to %s for %s (300ms)\n", phone, orderID)
}

func sendPushNotification(orderID string) {
	time.Sleep(200 * time.Millisecond) // Push service
	fmt.Printf("  [push] Sent for %s (200ms)\n", orderID)
}

func updateAnalytics(orderID string) {
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("  [analytics] Tracked %s (100ms)\n", orderID)
}

func notifyWarehouse(orderID string) {
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("  [warehouse] Notified for %s (100ms)\n", orderID)
}
