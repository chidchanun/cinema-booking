package services

import (
	"context"
	"sync"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/realtime"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeShowtimeRepository struct {
	showtime *models.Showtime
	err      error
}

func (f *fakeShowtimeRepository) FindByID(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Showtime, error) {
	if f.err != nil {
		return nil, f.err
	}

	if f.showtime == nil {
		return nil, nil
	}

	showtimeCopy := *f.showtime

	showtimeCopy.Seats = append(
		[]models.Seat(nil),
		f.showtime.Seats...,
	)

	return &showtimeCopy, nil
}

type recordingRealtimePublisher struct {
	mutex  sync.Mutex
	events []realtime.SeatEvent
	err    error
}

func (p *recordingRealtimePublisher) Publish(
	ctx context.Context,
	event realtime.SeatEvent,
) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.err != nil {
		return p.err
	}

	p.events = append(p.events, event)

	return nil
}

func (p *recordingRealtimePublisher) Snapshot() []realtime.SeatEvent {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	result := make(
		[]realtime.SeatEvent,
		len(p.events),
	)

	copy(result, p.events)

	return result
}

type recordingAuditPublisher struct {
	mutex  sync.Mutex
	events []events.Message
	err    error
}

func (p *recordingAuditPublisher) Publish(
	ctx context.Context,
	event events.Message,
) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.err != nil {
		return p.err
	}

	p.events = append(p.events, event)

	return nil
}

func (p *recordingAuditPublisher) Snapshot() []events.Message {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	result := make(
		[]events.Message,
		len(p.events),
	)

	copy(result, p.events)

	return result
}

func newTestShowtime() *models.Showtime {
	return &models.Showtime{
		ID:      primitive.NewObjectID(),
		MovieID: primitive.NewObjectID(),

		HallName: "Hall 1",

		StartTime: time.Now().
			UTC().
			Add(2 * time.Hour),

		EndTime: time.Now().
			UTC().
			Add(4 * time.Hour),

		Price:    250,
		Currency: "THB",

		SeatRows:    1,
		SeatsPerRow: 2,
		TotalSeats:  2,

		Seats: []models.Seat{
			{
				Code:   "A1",
				Row:    "A",
				Number: 1,
				Status: models.SeatStatusAvailable,
			},
			{
				Code:   "A2",
				Row:    "A",
				Number: 2,
				Status: models.SeatStatusAvailable,
			},
		},

		ShowtimeStatus: models.ShowtimeStatusActive,
	}
}