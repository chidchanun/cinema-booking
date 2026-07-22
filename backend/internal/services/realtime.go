package services

import (
	"context"
	"log"
	"time"

	"cinema-booking/internal/realtime"
)

func publishRealtimeBestEffort(
	publisher realtime.Publisher,
	event realtime.SeatEvent,
) {
	if publisher == nil {
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	if err := publisher.Publish(ctx, event); err != nil {
		log.Printf(
			"publish realtime event failed: %v",
			err,
		)
	}
}
