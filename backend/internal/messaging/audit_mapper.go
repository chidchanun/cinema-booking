package messaging

import (
	"encoding/json"
	"fmt"
	"strings"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func MapAuditEvent(
	body []byte,
) (*models.AuditLog, error) {
	envelope, err := events.DecodeEnvelope(body)
	if err != nil {
		return nil, err
	}

	switch envelope.EventType {
	case events.BookingConfirmedEventType:
		return mapBookingConfirmed(body)

	case events.SeatReleasedEventType:
		return mapSeatReleased(body)

	case events.BookingTimedOutEventType:
		return mapBookingTimedOut(body)

	case events.SystemErrorEventType:
		return mapSystemError(body)

	default:
		return nil, fmt.Errorf(
			"%w: unsupported event type %s",
			events.ErrInvalidEvent,
			envelope.EventType,
		)
	}
}

func mapBookingConfirmed(
	body []byte,
) (*models.AuditLog, error) {
	var event events.BookingConfirmed

	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf(
			"%w: decode booking confirmed: %v",
			events.ErrInvalidEvent,
			err,
		)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	bookingID, err := parseEventObjectID(
		event.Data.BookingID,
		"booking_id",
	)
	if err != nil {
		return nil, err
	}

	userID, err := parseEventObjectID(
		event.Data.UserID,
		"user_id",
	)
	if err != nil {
		return nil, err
	}

	movieID, err := parseEventObjectID(
		event.Data.MovieID,
		"movie_id",
	)
	if err != nil {
		return nil, err
	}

	showtimeID, err := parseEventObjectID(
		event.Data.ShowtimeID,
		"showtime_id",
	)
	if err != nil {
		return nil, err
	}

	return &models.AuditLog{
		EventID:   event.EventID,
		EventType: event.EventType,

		ActorType:   models.AuditActorUser,
		ActorUserID: &userID,

		EntityType: "BOOKING",
		EntityID:   &bookingID,

		Action:   "BOOKING_CONFIRMED",
		Severity: models.AuditSeverityInfo,

		Metadata: bson.M{
			"booking_code": event.Data.BookingCode,
			"movie_id":     movieID,
			"showtime_id":  showtimeID,
			"seat_code":    event.Data.SeatCode,
			"hall_name":    event.Data.HallName,
			"price":        event.Data.Price,
			"currency":     event.Data.Currency,
			"confirmed_at": event.Data.ConfirmedAt,
		},

		OccurredAt: event.OccurredAt,
	}, nil
}

func mapSeatReleased(
	body []byte,
) (*models.AuditLog, error) {
	var event events.SeatReleased

	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf(
			"%w: decode seat released: %v",
			events.ErrInvalidEvent,
			err,
		)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	userID, err := parseEventObjectID(
		event.Data.UserID,
		"user_id",
	)
	if err != nil {
		return nil, err
	}

	showtimeID, err := parseEventObjectID(
		event.Data.ShowtimeID,
		"showtime_id",
	)
	if err != nil {
		return nil, err
	}

	return &models.AuditLog{
		EventID:   event.EventID,
		EventType: event.EventType,

		ActorType:   models.AuditActorUser,
		ActorUserID: &userID,

		EntityType: "SHOWTIME",
		EntityID:   &showtimeID,

		Action:   "SEAT_RELEASED",
		Severity: models.AuditSeverityInfo,

		Metadata: bson.M{
			"seat_code": event.Data.SeatCode,
			"lock_id":   event.Data.LockID,
			"reason":    event.Data.Reason,
		},

		OccurredAt: event.OccurredAt,
	}, nil
}

func mapBookingTimedOut(
	body []byte,
) (*models.AuditLog, error) {
	var event events.BookingTimedOut

	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf(
			"%w: decode booking timeout: %v",
			events.ErrInvalidEvent,
			err,
		)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	showtimeID, err := parseEventObjectID(
		event.Data.ShowtimeID,
		"showtime_id",
	)
	if err != nil {
		return nil, err
	}

	return &models.AuditLog{
		EventID:   event.EventID,
		EventType: event.EventType,

		ActorType: models.AuditActorSystem,

		EntityType: "SHOWTIME",
		EntityID:   &showtimeID,

		Action:   "BOOKING_TIMEOUT",
		Severity: models.AuditSeverityWarning,

		Metadata: bson.M{
			"seat_code": event.Data.SeatCode,
			"reason":    event.Data.Reason,
		},

		OccurredAt: event.OccurredAt,
	}, nil
}

func mapSystemError(
	body []byte,
) (*models.AuditLog, error) {
	var event events.SystemError

	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf(
			"%w: decode system error: %v",
			events.ErrInvalidEvent,
			err,
		)
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	actorType := models.AuditActorSystem

	var actorUserID *primitive.ObjectID

	if strings.TrimSpace(event.Data.UserID) != "" {
		parsedUserID, err := parseEventObjectID(
			event.Data.UserID,
			"user_id",
		)
		if err != nil {
			return nil, err
		}

		actorType = models.AuditActorUser
		actorUserID = &parsedUserID
	}

	entityType := "SYSTEM"

	var entityID *primitive.ObjectID

	if strings.TrimSpace(event.Data.ShowtimeID) != "" {
		parsedShowtimeID, err := parseEventObjectID(
			event.Data.ShowtimeID,
			"showtime_id",
		)
		if err != nil {
			return nil, err
		}

		entityType = "SHOWTIME"
		entityID = &parsedShowtimeID
	}

	return &models.AuditLog{
		EventID:   event.EventID,
		EventType: event.EventType,

		ActorType:   actorType,
		ActorUserID: actorUserID,

		EntityType: entityType,
		EntityID:   entityID,

		Action:   "SYSTEM_ERROR",
		Severity: models.AuditSeverityError,

		Metadata: bson.M{
			"component":     event.Data.Component,
			"operation":     event.Data.Operation,
			"error_code":    event.Data.ErrorCode,
			"error_message": event.Data.ErrorMessage,
			"seat_code":     event.Data.SeatCode,
		},

		OccurredAt: event.OccurredAt,
	}, nil
}

func parseEventObjectID(
	rawValue string,
	fieldName string,
) (primitive.ObjectID, error) {
	value, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(rawValue),
	)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf(
			"%w: invalid %s",
			events.ErrInvalidEvent,
			fieldName,
		)
	}

	return value, nil
}