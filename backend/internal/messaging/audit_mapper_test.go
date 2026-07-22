package messaging

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMapBookingConfirmedEvent(t *testing.T) {
	now := time.Now().UTC()

	booking := &models.Booking{
		ID:          primitive.NewObjectID(),
		BookingCode: "BK-TEST-001",

		UserID:     primitive.NewObjectID(),
		MovieID:    primitive.NewObjectID(),
		ShowtimeID: primitive.NewObjectID(),

		SeatCode: "A1",
		HallName: "Hall 1",

		ShowtimeStart: now.Add(2 * time.Hour),

		Price:    250,
		Currency: "THB",

		Status:      models.BookingStatusBooked,
		ConfirmedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	event, err := events.NewBookingConfirmed(booking)
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}

	auditLog, err := MapAuditEvent(body)
	if err != nil {
		t.Fatalf("map audit event failed: %v", err)
	}

	if auditLog.Action != "BOOKING_CONFIRMED" {
		t.Fatalf(
			"expected BOOKING_CONFIRMED, got %s",
			auditLog.Action,
		)
	}

	if auditLog.ActorType != models.AuditActorUser {
		t.Fatalf(
			"expected USER actor, got %s",
			auditLog.ActorType,
		)
	}

	if auditLog.ActorUserID == nil ||
		*auditLog.ActorUserID != booking.UserID {
		t.Fatal("unexpected actor user ID")
	}

	if auditLog.Severity != models.AuditSeverityInfo {
		t.Fatalf(
			"expected INFO severity, got %s",
			auditLog.Severity,
		)
	}
}

func TestMapSeatReleasedEvent(t *testing.T) {
	userID := primitive.NewObjectID()
	showtimeID := primitive.NewObjectID()

	event, err := events.NewSeatReleased(
		userID.Hex(),
		showtimeID.Hex(),
		"A1",
		"lock-id",
		"USER_RELEASED",
	)
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}

	auditLog, err := MapAuditEvent(body)
	if err != nil {
		t.Fatalf("map event failed: %v", err)
	}

	if auditLog.Action != "SEAT_RELEASED" {
		t.Fatalf(
			"expected SEAT_RELEASED, got %s",
			auditLog.Action,
		)
	}

	if auditLog.EntityType != "SHOWTIME" {
		t.Fatalf(
			"expected SHOWTIME entity, got %s",
			auditLog.EntityType,
		)
	}

	if auditLog.EntityID == nil ||
		*auditLog.EntityID != showtimeID {
		t.Fatal("unexpected showtime entity ID")
	}
}

func TestMapBookingTimeoutEvent(t *testing.T) {
	showtimeID := primitive.NewObjectID()

	event, err := events.NewBookingTimedOut(
		showtimeID.Hex(),
		"A2",
		"LOCK_EXPIRED",
	)
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}

	auditLog, err := MapAuditEvent(body)
	if err != nil {
		t.Fatalf("map event failed: %v", err)
	}

	if auditLog.Action != "BOOKING_TIMEOUT" {
		t.Fatalf(
			"expected BOOKING_TIMEOUT, got %s",
			auditLog.Action,
		)
	}

	if auditLog.ActorType != models.AuditActorSystem {
		t.Fatalf(
			"expected SYSTEM actor, got %s",
			auditLog.ActorType,
		)
	}

	if auditLog.Severity !=
		models.AuditSeverityWarning {
		t.Fatalf(
			"expected WARNING severity, got %s",
			auditLog.Severity,
		)
	}
}

func TestMapSystemErrorEvent(t *testing.T) {
	showtimeID := primitive.NewObjectID()

	event, err := events.NewSystemError(
		events.SystemErrorData{
			Component:    "REDIS_LOCK",
			Operation:    "ACQUIRE_SEAT_LOCK",
			ErrorCode:    "SEAT_LOCK_UNAVAILABLE",
			ErrorMessage: "Redis unavailable",

			ShowtimeID: showtimeID.Hex(),
			SeatCode:   "A3",
		},
	)
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event failed: %v", err)
	}

	auditLog, err := MapAuditEvent(body)
	if err != nil {
		t.Fatalf("map event failed: %v", err)
	}

	if auditLog.Action != "SYSTEM_ERROR" {
		t.Fatalf(
			"expected SYSTEM_ERROR, got %s",
			auditLog.Action,
		)
	}

	if auditLog.Severity != models.AuditSeverityError {
		t.Fatalf(
			"expected ERROR severity, got %s",
			auditLog.Severity,
		)
	}
}

func TestMapAuditEventRejectsUnknownEvent(t *testing.T) {
	body := []byte(`{
		"event_id": "unknown:1",
		"event_type": "unknown.event",
		"version": 1,
		"occurred_at": "2026-07-19T00:00:00Z"
	}`)

	_, err := MapAuditEvent(body)

	if !errors.Is(err, events.ErrInvalidEvent) {
		t.Fatalf(
			"expected ErrInvalidEvent, got %v",
			err,
		)
	}
}
