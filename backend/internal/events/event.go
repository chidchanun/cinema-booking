package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidEvent = errors.New("invalid domain event")

type Message interface {
	Validate() error

	ID() string
	Name() string
	SchemaVersion() int
	HappenedAt() time.Time
}

type Envelope struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Version   int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
}

func DecodeEnvelope(body []byte) (*Envelope, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf(
			"%w: message body is empty",
			ErrInvalidEvent,
		)
	}

	var envelope Envelope

	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf(
			"%w: decode envelope: %v",
			ErrInvalidEvent,
			err,
		)
	}

	if err := validateEnvelope(
		envelope.EventID,
		envelope.EventType,
		envelope.Version,
		envelope.OccurredAt,
	); err != nil {
		return nil, err
	}

	return &envelope, nil
}

func validateEnvelope(
	eventID string,
	eventType string,
	version int,
	occurredAt time.Time,
) error {
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf(
			"%w: event_id is required",
			ErrInvalidEvent,
		)
	}

	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf(
			"%w: event_type is required",
			ErrInvalidEvent,
		)
	}

	if version < 1 {
		return fmt.Errorf(
			"%w: version must be greater than zero",
			ErrInvalidEvent,
		)
	}

	if occurredAt.IsZero() {
		return fmt.Errorf(
			"%w: occurred_at is required",
			ErrInvalidEvent,
		)
	}

	return nil
}