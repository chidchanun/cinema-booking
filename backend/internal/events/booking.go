package events

import (
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"
)

const (
	BookingConfirmedEventType    = "booking.confirmed"
	BookingConfirmedEventVersion = 1
)



type BookingConfirmed struct {
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`

	Data BookingConfirmedData `json:"data"`
}

type BookingConfirmedData struct {
	BookingID   string `json:"booking_id"`
	BookingCode string `json:"booking_code"`

	UserID     string `json:"user_id"`
	MovieID    string `json:"movie_id"`
	ShowtimeID string `json:"showtime_id"`

	SeatCode string `json:"seat_code"`
	HallName string `json:"hall_name"`

	ShowtimeStart time.Time `json:"showtime_start"`

	Price    int64  `json:"price"`
	Currency string `json:"currency"`

	ConfirmedAt time.Time `json:"confirmed_at"`
}

func NewBookingConfirmed(
	booking *models.Booking,
) (BookingConfirmed, error) {
	if booking == nil || booking.ID.IsZero() || booking.UserID.IsZero() || booking.MovieID.IsZero() || booking.ShowtimeID.IsZero() {
		return BookingConfirmed{}, ErrInvalidEvent
	}

	event := BookingConfirmed{
		EventID: fmt.Sprintf(
			"%s:%s",
			BookingConfirmedEventType,
			booking.ID.Hex(),
		),

		EventType:  BookingConfirmedEventType,
		Version:    BookingConfirmedEventVersion,
		OccurredAt: booking.ConfirmedAt.UTC(),

		Data: BookingConfirmedData{
			BookingID:   booking.ID.Hex(),
			BookingCode: booking.BookingCode,

			UserID:     booking.UserID.Hex(),
			MovieID:    booking.MovieID.Hex(),
			ShowtimeID: booking.ShowtimeID.Hex(),

			SeatCode: booking.SeatCode,
			HallName: booking.HallName,

			ShowtimeStart: booking.ShowtimeStart.UTC(),

			Price:    booking.Price,
			Currency: booking.Currency,

			ConfirmedAt: booking.ConfirmedAt.UTC(),
		},
	}

	if err := event.Validate(); err != nil {
		return BookingConfirmed{}, err
	}

	return event, nil
}

func (e BookingConfirmed) Validate() error {
	if err := validateEnvelope(
		e.EventID,
		e.EventType,
		e.Version,
		e.OccurredAt,
	); err != nil {
		return err
	}

	if e.EventType != BookingConfirmedEventType {
		return fmt.Errorf(
			"%w: unsupported booking event type",
			ErrInvalidEvent,
		)
	}

	if e.Version != BookingConfirmedEventVersion {
		return fmt.Errorf(
			"%w: unsupported booking event version",
			ErrInvalidEvent,
		)
	}

	if strings.TrimSpace(e.Data.BookingID) == "" ||
		strings.TrimSpace(e.Data.BookingCode) == "" ||
		strings.TrimSpace(e.Data.UserID) == "" ||
		strings.TrimSpace(e.Data.MovieID) == "" ||
		strings.TrimSpace(e.Data.ShowtimeID) == "" ||
		strings.TrimSpace(e.Data.SeatCode) == "" {
		return fmt.Errorf(
			"%w: required booking data is missing",
			ErrInvalidEvent,
		)
	}

	return nil
}

func (e BookingConfirmed) ID() string {
	return e.EventID
}

func (e BookingConfirmed) Name() string {
	return e.EventType
}

func (e BookingConfirmed) SchemaVersion() int {
	return e.Version
}

func (e BookingConfirmed) HappenedAt() time.Time {
	return e.OccurredAt
}