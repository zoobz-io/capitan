package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobz-io/capitan"
	capitantesting "github.com/zoobz-io/capitan/testing"
)

// TestRealWorld_AuditLog demonstrates an audit logging system using capitan.
func TestRealWorld_AuditLog(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals
	userCreated := capitan.NewSignal("user.created", "User account created")
	userUpdated := capitan.NewSignal("user.updated", "User account updated")
	userDeleted := capitan.NewSignal("user.deleted", "User account deleted")

	// Define keys
	userID := capitan.NewStringKey("user_id")
	email := capitan.NewStringKey("email")
	action := capitan.NewStringKey("action")

	// Capture audit events - use same handler for all signals
	capture := capitantesting.NewEventCapture()
	handler := capture.Handler()
	c.Hook(userCreated, handler)
	c.Hook(userUpdated, handler)
	c.Hook(userDeleted, handler)

	// Emit user lifecycle events
	c.Emit(context.Background(), userCreated,
		userID.Field("user-123"),
		email.Field("john@example.com"),
		action.Field("signup"),
	)

	c.Emit(context.Background(), userUpdated,
		userID.Field("user-123"),
		email.Field("john.doe@example.com"),
		action.Field("email_change"),
	)

	c.Emit(context.Background(), userDeleted,
		userID.Field("user-123"),
		action.Field("account_deletion"),
	)

	// Shutdown to drain queues and ensure all events are processed
	c.Shutdown()

	// Verify all events captured
	events := capture.Events()
	if len(events) != 3 {
		t.Fatalf("expected 3 audit events, got %d", len(events))
	}

	// Verify event signals are present (listener delivery order not guaranteed across signals)
	signals := make(map[capitan.Signal]capitantesting.CapturedEvent)
	for _, event := range events {
		signals[event.Signal] = event
	}

	createdEvent, hasCreated := signals[userCreated]
	updatedEvent, hasUpdated := signals[userUpdated]
	deletedEvent, hasDeleted := signals[userDeleted]

	if !hasCreated {
		t.Error("missing user.created event")
	}
	if !hasUpdated {
		t.Error("missing user.updated event")
	}
	if !hasDeleted {
		t.Error("missing user.deleted event")
	}

	// Verify emission timestamps are in chronological order (captured at emit time)

	if hasCreated && hasUpdated {
		if !updatedEvent.Timestamp.After(createdEvent.Timestamp) {
			t.Error("userUpdated should be emitted after userCreated")
		}
	}
	if hasUpdated && hasDeleted {
		if !deletedEvent.Timestamp.After(updatedEvent.Timestamp) {
			t.Error("userDeleted should be emitted after userUpdated")
		}
	}

	// Verify all events have user_id (check fields in captured event)
	for sig, event := range signals {
		found := false
		for _, field := range event.Fields {
			if field.Key().Name() == "user_id" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("event %s missing user_id field", sig.Name())
		}
	}
}

// TestRealWorld_EventDrivenMicroservices demonstrates microservice communication.
func TestRealWorld_EventDrivenMicroservices(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals
	orderPlaced := capitan.NewSignal("order.placed", "Order placed by customer")
	orderValidated := capitan.NewSignal("order.validated", "Order validated by inventory service")
	orderShipped := capitan.NewSignal("order.shipped", "Order shipped by warehouse")

	// Define keys
	orderID := capitan.NewStringKey("order_id")
	customerID := capitan.NewStringKey("customer_id")
	status := capitan.NewStringKey("status")
	amount := capitan.NewFloat64Key("amount")

	// Track service interactions
	var inventoryChecked atomic.Bool
	var warehouseNotified atomic.Bool
	var customerNotified atomic.Bool

	// Inventory Service: validates orders
	c.Hook(orderPlaced, func(_ context.Context, e *capitan.Event) {
		orderIDVal, _ := orderID.From(e)
		inventoryChecked.Store(true)

		// Emit validation event
		c.Emit(context.Background(), orderValidated,
			orderID.Field(orderIDVal),
			status.Field("validated"),
		)
	})

	// Warehouse Service: ships validated orders
	c.Hook(orderValidated, func(_ context.Context, e *capitan.Event) {
		orderIDVal, _ := orderID.From(e)
		warehouseNotified.Store(true)

		// Emit shipping event
		c.Emit(context.Background(), orderShipped,
			orderID.Field(orderIDVal),
			status.Field("shipped"),
		)
	})

	// Notification Service: notifies customers
	c.Hook(orderShipped, func(_ context.Context, _ *capitan.Event) {
		customerNotified.Store(true)
	})

	// Customer places order
	c.Emit(context.Background(), orderPlaced,
		orderID.Field("order-456"),
		customerID.Field("customer-789"),
		amount.Field(99.99),
	)

	// Give time for cascading async events to process
	time.Sleep(50 * time.Millisecond)
	c.Shutdown()

	// Verify entire flow completed
	if !inventoryChecked.Load() {
		t.Error("inventory service should have checked order")
	}
	if !warehouseNotified.Load() {
		t.Error("warehouse service should have been notified")
	}
	if !customerNotified.Load() {
		t.Error("customer should have been notified")
	}
}

// TestRealWorld_UserActivityTracking demonstrates user activity tracking with aggregation.
func TestRealWorld_UserActivityTracking(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals
	pageView := capitan.NewSignal("user.page_view", "User viewed a page")
	buttonClick := capitan.NewSignal("user.button_click", "User clicked a button")
	formSubmit := capitan.NewSignal("user.form_submit", "User submitted a form")

	// Define keys
	userID := capitan.NewStringKey("user_id")
	page := capitan.NewStringKey("page")
	button := capitan.NewStringKey("button")
	form := capitan.NewStringKey("form")

	// Activity counter by user (with mutex for concurrent access)
	activityCounts := make(map[string]int64)
	var activityMu sync.Mutex
	counter := capitantesting.NewEventCounter()

	// Aggregate all user events
	c.Observe(func(_ context.Context, e *capitan.Event) {
		if id, ok := userID.From(e); ok {
			activityMu.Lock()
			activityCounts[id]++
			activityMu.Unlock()
		}
		counter.Handler()(context.Background(), e)
	}, pageView, buttonClick, formSubmit)

	// Simulate user activity
	c.Emit(context.Background(), pageView,
		userID.Field("user-1"),
		page.Field("/home"),
	)

	c.Emit(context.Background(), buttonClick,
		userID.Field("user-1"),
		button.Field("signup"),
	)

	c.Emit(context.Background(), formSubmit,
		userID.Field("user-1"),
		form.Field("registration"),
	)

	c.Emit(context.Background(), pageView,
		userID.Field("user-2"),
		page.Field("/pricing"),
	)

	// Shutdown to ensure all async events are processed
	c.Shutdown()

	// Verify activity counts
	activityMu.Lock()
	user1Count := activityCounts["user-1"]
	user2Count := activityCounts["user-2"]
	activityMu.Unlock()

	if user1Count != 3 {
		t.Errorf("expected 3 events for user-1, got %d", user1Count)
	}
	if user2Count != 1 {
		t.Errorf("expected 1 event for user-2, got %d", user2Count)
	}
	if counter.Count() != 4 {
		t.Errorf("expected 4 total events, got %d", counter.Count())
	}
}

// TestRealWorld_SystemMonitoring demonstrates system monitoring and alerting.
func TestRealWorld_SystemMonitoring(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals with severity
	cpuHigh := capitan.NewSignal("system.cpu_high", "CPU usage above threshold")
	memoryHigh := capitan.NewSignal("system.memory_high", "Memory usage above threshold")
	diskFull := capitan.NewSignal("system.disk_full", "Disk space critical")

	// Define keys
	hostname := capitan.NewStringKey("hostname")
	value := capitan.NewFloat64Key("value")
	threshold := capitan.NewFloat64Key("threshold")
	message := capitan.NewStringKey("message")

	// Track alerts (with mutex for concurrent access)
	var alerts []string
	var alertsMu sync.Mutex

	// Alert handler for critical events
	alertHandler := func(_ context.Context, e *capitan.Event) {
		if e.Severity() == capitan.SeverityError {
			if msg, ok := message.From(e); ok {
				alertsMu.Lock()
				alerts = append(alerts, msg)
				alertsMu.Unlock()
			}
		}
	}

	c.Observe(alertHandler, cpuHigh, memoryHigh, diskFull)

	// Emit monitoring events
	c.Warn(context.Background(), cpuHigh,
		hostname.Field("server-01"),
		value.Field(75.5),
		threshold.Field(80.0),
		message.Field("CPU usage elevated"),
	)

	c.Error(context.Background(), memoryHigh,
		hostname.Field("server-01"),
		value.Field(92.3),
		threshold.Field(90.0),
		message.Field("Memory usage critical"),
	)

	c.Error(context.Background(), diskFull,
		hostname.Field("server-02"),
		value.Field(98.7),
		threshold.Field(95.0),
		message.Field("Disk space critical"),
	)

	// Shutdown to ensure all async events are processed
	c.Shutdown()

	// Verify only ERROR severity events generated alerts
	alertsMu.Lock()
	alertCount := len(alerts)
	alertsCopy := make([]string, len(alerts))
	copy(alertsCopy, alerts)
	alertsMu.Unlock()

	if alertCount != 2 {
		t.Fatalf("expected 2 critical alerts, got %d", alertCount)
	}

	expectedAlerts := map[string]bool{
		"Memory usage critical": false,
		"Disk space critical":   false,
	}

	for _, alert := range alertsCopy {
		if _, ok := expectedAlerts[alert]; ok {
			expectedAlerts[alert] = true
		}
	}

	for alert, received := range expectedAlerts {
		if !received {
			t.Errorf("expected alert %q not received", alert)
		}
	}
}

// TestRealWorld_MultiTenantEventRouting demonstrates multi-tenant event routing.
func TestRealWorld_MultiTenantEventRouting(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals
	dataCreated := capitan.NewSignal("data.created", "Data created in tenant")
	dataUpdated := capitan.NewSignal("data.updated", "Data updated in tenant")

	// Define keys
	tenantID := capitan.NewStringKey("tenant_id")
	dataID := capitan.NewStringKey("data_id")

	// Track events per tenant
	tenant1Events := capitantesting.NewEventCounter()
	tenant2Events := capitantesting.NewEventCounter()

	// Tenant-specific handlers
	tenant1Handler := func(_ context.Context, e *capitan.Event) {
		if id, ok := tenantID.From(e); ok && id == "tenant-1" {
			tenant1Events.Handler()(context.Background(), e)
		}
	}

	tenant2Handler := func(_ context.Context, e *capitan.Event) {
		if id, ok := tenantID.From(e); ok && id == "tenant-2" {
			tenant2Events.Handler()(context.Background(), e)
		}
	}

	c.Observe(tenant1Handler, dataCreated, dataUpdated)
	c.Observe(tenant2Handler, dataCreated, dataUpdated)

	// Emit events for both tenants
	c.Emit(context.Background(), dataCreated,
		tenantID.Field("tenant-1"),
		dataID.Field("data-1"),
	)

	c.Emit(context.Background(), dataCreated,
		tenantID.Field("tenant-2"),
		dataID.Field("data-2"),
	)

	c.Emit(context.Background(), dataUpdated,
		tenantID.Field("tenant-1"),
		dataID.Field("data-1"),
	)

	// Shutdown to ensure all async events are processed
	c.Shutdown()

	// Verify tenant isolation
	if tenant1Events.Count() != 2 {
		t.Errorf("tenant-1 should have 2 events, got %d", tenant1Events.Count())
	}
	if tenant2Events.Count() != 1 {
		t.Errorf("tenant-2 should have 1 event, got %d", tenant2Events.Count())
	}
}

// TestRealWorld_ComplexWorkflow demonstrates a complex multi-stage workflow.
func TestRealWorld_ComplexWorkflow(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define workflow signals
	workflowStart := capitan.NewSignal("workflow.start", "Workflow started")
	step1Complete := capitan.NewSignal("workflow.step1", "Step 1 completed")
	step2Complete := capitan.NewSignal("workflow.step2", "Step 2 completed")
	step3Complete := capitan.NewSignal("workflow.step3", "Step 3 completed")
	workflowComplete := capitan.NewSignal("workflow.complete", "Workflow completed")

	// Define keys
	workflowID := capitan.NewStringKey("workflow_id")
	stepName := capitan.NewStringKey("step")
	duration := capitan.NewInt64Key("duration_ms")

	// Track workflow progression
	var step1Done, step2Done, step3Done, workflowDone atomic.Bool

	// Step 1: Validation
	c.Hook(workflowStart, func(_ context.Context, e *capitan.Event) {
		id, _ := workflowID.From(e)
		step1Done.Store(true)

		c.Emit(context.Background(), step1Complete,
			workflowID.Field(id),
			stepName.Field("validation"),
			duration.Field(int64(10)),
		)
	})

	// Step 2: Processing
	c.Hook(step1Complete, func(_ context.Context, e *capitan.Event) {
		id, _ := workflowID.From(e)
		step2Done.Store(true)

		c.Emit(context.Background(), step2Complete,
			workflowID.Field(id),
			stepName.Field("processing"),
			duration.Field(int64(50)),
		)
	})

	// Step 3: Finalization
	c.Hook(step2Complete, func(_ context.Context, e *capitan.Event) {
		id, _ := workflowID.From(e)
		step3Done.Store(true)

		c.Emit(context.Background(), step3Complete,
			workflowID.Field(id),
			stepName.Field("finalization"),
			duration.Field(int64(5)),
		)
	})

	// Workflow completion
	c.Hook(step3Complete, func(_ context.Context, _ *capitan.Event) {
		workflowDone.Store(true)

		c.Emit(context.Background(), workflowComplete,
			workflowID.Field("wf-001"),
			duration.Field(int64(65)), // Total duration
		)
	})

	// Track all workflow events
	capture := capitantesting.NewEventCapture()
	c.Observe(capture.Handler())

	// Start workflow
	c.Emit(context.Background(), workflowStart,
		workflowID.Field("wf-001"),
	)

	// Give time for cascading async events to process
	time.Sleep(50 * time.Millisecond)
	c.Shutdown()

	// Verify all steps completed
	if !step1Done.Load() {
		t.Error("step 1 should have completed")
	}
	if !step2Done.Load() {
		t.Error("step 2 should have completed")
	}
	if !step3Done.Load() {
		t.Error("step 3 should have completed")
	}
	if !workflowDone.Load() {
		t.Error("workflow should have completed")
	}

	// Verify event count (start + 3 steps + complete = 5)
	events := capture.Events()
	if len(events) != 5 {
		t.Errorf("expected 5 workflow events, got %d", len(events))
	}
}

// TestRealWorld_ErrorHandlingAndRecovery demonstrates error handling patterns.
func TestRealWorld_ErrorHandlingAndRecovery(t *testing.T) {
	c := capitantesting.TestCapitan()

	// Define signals
	operationStart := capitan.NewSignal("operation.start", "Operation started")
	operationError := capitan.NewSignal("operation.error", "Operation error occurred")
	operationRetry := capitan.NewSignal("operation.retry", "Operation retry attempt")
	operationSuccess := capitan.NewSignal("operation.success", "Operation succeeded")

	// Define keys
	operationID := capitan.NewStringKey("operation_id")
	errorMsg := capitan.NewErrorKey("error")
	retryCount := capitan.NewIntKey("retry_count")

	// Simulate retry logic with pointer for sync mode
	var retries int
	maxRetries := 3

	c.Hook(operationStart, func(_ context.Context, e *capitan.Event) {
		id, _ := operationID.From(e)

		// Simulate failure
		c.Error(context.Background(), operationError,
			operationID.Field(id),
			errorMsg.Field(fmt.Errorf("connection timeout")),
		)
	})

	c.Hook(operationError, func(_ context.Context, e *capitan.Event) {
		id, _ := operationID.From(e)
		retries++

		if retries <= maxRetries {
			c.Warn(context.Background(), operationRetry,
				operationID.Field(id),
				retryCount.Field(retries),
			)
		}
	})

	c.Hook(operationRetry, func(_ context.Context, e *capitan.Event) {
		id, _ := operationID.From(e)
		count, _ := retryCount.From(e)

		if count == maxRetries {
			// Final retry succeeds
			c.Info(context.Background(), operationSuccess,
				operationID.Field(id),
			)
		} else {
			// Retry fails
			c.Error(context.Background(), operationError,
				operationID.Field(id),
				errorMsg.Field(fmt.Errorf("connection timeout")),
			)
		}
	})

	// Track outcome
	var succeeded atomic.Bool
	c.Hook(operationSuccess, func(_ context.Context, _ *capitan.Event) {
		succeeded.Store(true)
	})

	// Start operation
	c.Emit(context.Background(), operationStart,
		operationID.Field("op-123"),
	)

	// Give time for cascading async events to process
	time.Sleep(50 * time.Millisecond)
	c.Shutdown()

	// Verify retry logic worked
	if retries != maxRetries {
		t.Errorf("expected %d retries, got %d", maxRetries, retries)
	}
	if !succeeded.Load() {
		t.Error("operation should have eventually succeeded")
	}
}

// TestRealWorld_TimestampOrdering verifies events are timestamped.
func TestRealWorld_TimestampOrdering(t *testing.T) {
	c := capitantesting.TestCapitan()

	sig := capitan.NewSignal("test.timestamp", "Test timestamp ordering")
	counter := capitan.NewIntKey("counter")

	capture := capitantesting.NewEventCapture()
	c.Hook(sig, capture.Handler())

	// Emit events with delays and counter fields to track order
	for i := 0; i < 5; i++ {
		c.Emit(context.Background(), sig, counter.Field(i))
		time.Sleep(time.Millisecond)
	}

	events := capture.Events()
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	// Verify all events have timestamps
	baseTime := time.Now().Add(-time.Minute)
	for i, event := range events {
		ts := event.Timestamp
		if ts.Before(baseTime) {
			t.Errorf("event %d has invalid timestamp %v", i, ts)
		}
		if ts.After(time.Now().Add(time.Second)) {
			t.Errorf("event %d timestamp %v is in the future", i, ts)
		}
	}
}
