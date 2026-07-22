package events

import (
	"fmt"
	"strings"
	"time"
)

const (
	AuditEventVersion = 1

	SeatReleasedEventType   = "seat.released"
	BookingTimedOutEventType = "booking.timeout"
	SystemErrorEventType    = "system.error"
)

type SeatReleased struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`

	Data SeatReleasedData `json:"data"`
}

type SeatReleasedData struct {
	UserID     string `json:"user_id"`
	ShowtimeID string `json:"showtime_id"`
	SeatCode   string `json:"seat_code"`
	LockID     string `json:"lock_id"`
	Reason     string `json:"reason"`
}

func NewSeatReleased(
	userID string,
	showtimeID string,
	seatCode string,
	lockID string,
	reason string,
) (SeatReleased, error) {
	now := time.Now().UTC()

	event := SeatReleased{
		EventID: fmt.Sprintf(
			"%s:%s:%s:%s",
			SeatReleasedEventType,
			strings.TrimSpace(showtimeID),
			strings.ToUpper(strings.TrimSpace(seatCode)),
			strings.TrimSpace(lockID),
		),
		EventType: SeatReleasedEventType,
		Version:   AuditEventVersion,
		OccurredAt: now,

		Data: SeatReleasedData{
			UserID:     strings.TrimSpace(userID),
			ShowtimeID: strings.TrimSpace(showtimeID),
			SeatCode: strings.ToUpper(
				strings.TrimSpace(seatCode),
			),
			LockID: strings.TrimSpace(lockID),
			Reason: strings.ToUpper(
				strings.TrimSpace(reason),
			),
		},
	}

	if event.Data.Reason == "" {
		event.Data.Reason = "USER_RELEASED"
	}

	if err := event.Validate(); err != nil {
		return SeatReleased{}, err
	}

	return event, nil
}

func (e SeatReleased) Validate() error {
	if err := validateEnvelope(
		e.EventID,
		e.EventType,
		e.Version,
		e.OccurredAt,
	); err != nil {
		return err
	}

	if e.EventType != SeatReleasedEventType ||
		e.Version != AuditEventVersion {
		return fmt.Errorf(
			"%w: invalid seat released event",
			ErrInvalidEvent,
		)
	}

	if e.Data.UserID == "" ||
		e.Data.ShowtimeID == "" ||
		e.Data.SeatCode == "" ||
		e.Data.LockID == "" {
		return fmt.Errorf(
			"%w: required seat released data is missing",
			ErrInvalidEvent,
		)
	}

	return nil
}

func (e SeatReleased) ID() string {
	return e.EventID
}

func (e SeatReleased) Name() string {
	return e.EventType
}

func (e SeatReleased) SchemaVersion() int {
	return e.Version
}

func (e SeatReleased) HappenedAt() time.Time {
	return e.OccurredAt
}

type BookingTimedOut struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`

	Data BookingTimedOutData `json:"data"`
}

type BookingTimedOutData struct {
	ShowtimeID string `json:"showtime_id"`
	SeatCode   string `json:"seat_code"`
	Reason     string `json:"reason"`
}

func NewBookingTimedOut(
	showtimeID string,
	seatCode string,
	reason string,
) (BookingTimedOut, error) {
	now := time.Now().UTC()

	event := BookingTimedOut{
		EventID: fmt.Sprintf(
			"%s:%s:%s:%d",
			BookingTimedOutEventType,
			strings.TrimSpace(showtimeID),
			strings.ToUpper(strings.TrimSpace(seatCode)),
			now.UnixNano(),
		),
		EventType: BookingTimedOutEventType,
		Version:   AuditEventVersion,
		OccurredAt: now,

		Data: BookingTimedOutData{
			ShowtimeID: strings.TrimSpace(showtimeID),
			SeatCode: strings.ToUpper(
				strings.TrimSpace(seatCode),
			),
			Reason: strings.ToUpper(
				strings.TrimSpace(reason),
			),
		},
	}

	if event.Data.Reason == "" {
		event.Data.Reason = "LOCK_EXPIRED"
	}

	if err := event.Validate(); err != nil {
		return BookingTimedOut{}, err
	}

	return event, nil
}

func (e BookingTimedOut) Validate() error {
	if err := validateEnvelope(
		e.EventID,
		e.EventType,
		e.Version,
		e.OccurredAt,
	); err != nil {
		return err
	}

	if e.EventType != BookingTimedOutEventType ||
		e.Version != AuditEventVersion {
		return fmt.Errorf(
			"%w: invalid booking timeout event",
			ErrInvalidEvent,
		)
	}

	if e.Data.ShowtimeID == "" ||
		e.Data.SeatCode == "" {
		return fmt.Errorf(
			"%w: required booking timeout data is missing",
			ErrInvalidEvent,
		)
	}

	return nil
}

func (e BookingTimedOut) ID() string {
	return e.EventID
}

func (e BookingTimedOut) Name() string {
	return e.EventType
}

func (e BookingTimedOut) SchemaVersion() int {
	return e.Version
}

func (e BookingTimedOut) HappenedAt() time.Time {
	return e.OccurredAt
}

type SystemError struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`

	Data SystemErrorData `json:"data"`
}

type SystemErrorData struct {
	Component    string `json:"component"`
	Operation    string `json:"operation"`
	ErrorCode   string `json:"error_code"`
	ErrorMessage string `json:"error_message"`

	UserID     string `json:"user_id,omitempty"`
	ShowtimeID string `json:"showtime_id,omitempty"`
	SeatCode   string `json:"seat_code,omitempty"`
}

func NewSystemError(
	data SystemErrorData,
) (SystemError, error) {
	now := time.Now().UTC()

	data.Component = strings.ToUpper(
		strings.TrimSpace(data.Component),
	)
	data.Operation = strings.ToUpper(
		strings.TrimSpace(data.Operation),
	)
	data.ErrorCode = strings.ToUpper(
		strings.TrimSpace(data.ErrorCode),
	)
	data.ErrorMessage = truncateText(
		strings.TrimSpace(data.ErrorMessage),
		500,
	)
	data.UserID = strings.TrimSpace(data.UserID)
	data.ShowtimeID = strings.TrimSpace(data.ShowtimeID)
	data.SeatCode = strings.ToUpper(
		strings.TrimSpace(data.SeatCode),
	)

	event := SystemError{
		EventID: fmt.Sprintf(
			"%s:%d",
			SystemErrorEventType,
			now.UnixNano(),
		),
		EventType: SystemErrorEventType,
		Version:   AuditEventVersion,
		OccurredAt: now,
		Data:       data,
	}

	if err := event.Validate(); err != nil {
		return SystemError{}, err
	}

	return event, nil
}

func (e SystemError) Validate() error {
	if err := validateEnvelope(
		e.EventID,
		e.EventType,
		e.Version,
		e.OccurredAt,
	); err != nil {
		return err
	}

	if e.EventType != SystemErrorEventType ||
		e.Version != AuditEventVersion {
		return fmt.Errorf(
			"%w: invalid system error event",
			ErrInvalidEvent,
		)
	}

	if e.Data.Component == "" ||
		e.Data.Operation == "" ||
		e.Data.ErrorCode == "" ||
		e.Data.ErrorMessage == "" {
		return fmt.Errorf(
			"%w: required system error data is missing",
			ErrInvalidEvent,
		)
	}

	return nil
}

func (e SystemError) ID() string {
	return e.EventID
}

func (e SystemError) Name() string {
	return e.EventType
}

func (e SystemError) SchemaVersion() int {
	return e.Version
}

func (e SystemError) HappenedAt() time.Time {
	return e.OccurredAt
}

func truncateText(
	value string,
	maxLength int,
) string {
	characters := []rune(value)

	if len(characters) <= maxLength {
		return value
	}

	return string(characters[:maxLength])
}