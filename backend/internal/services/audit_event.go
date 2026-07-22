package services

import (
	"context"
	"log"
	"time"

	"cinema-booking/internal/events"
)

type AuditEventPublisher interface {
	Publish(
		ctx context.Context,
		event events.Message,
	) error
}

func publishAuditBestEffort(
	publisher AuditEventPublisher,
	timeout time.Duration,
	event events.Message,
) {
	if publisher == nil || event == nil {
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancel()

	if err := publisher.Publish(ctx, event); err != nil {
		log.Printf(
			"publish audit event %s failed: %v",
			event.Name(),
			err,
		)
	}
}