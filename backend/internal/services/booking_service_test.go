package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type atomicBookingStore struct {
	mutex sync.Mutex

	seatAlreadyBooked bool
	bookings          []models.Booking
}

func (s *atomicBookingStore) Confirm(
	ctx context.Context,
	booking *models.Booking,
) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.seatAlreadyBooked {
		return repository.ErrSeatUnavailable
	}

	s.seatAlreadyBooked = true

	bookingCopy := *booking

	s.bookings = append(
		s.bookings,
		bookingCopy,
	)

	return nil
}

func (s *atomicBookingStore) FindByIDAndUserID(
	ctx context.Context,
	bookingID primitive.ObjectID,
	userID primitive.ObjectID,
) (*models.Booking, error) {
	return nil, repository.ErrBookingNotFound
}

func (s *atomicBookingStore) FindByUserID(
	ctx context.Context,
	userID primitive.ObjectID,
	skip int64,
	limit int64,
) ([]models.Booking, int64, error) {
	return nil, 0, nil
}

func (s *atomicBookingStore) Count() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return len(s.bookings)
}

type fakeBookingLockManager struct {
	validateErr error
	releaseErr  error

	mutex       sync.Mutex
	releaseCall int
}

func (m *fakeBookingLockManager) ValidateOwnership(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) (time.Duration, error) {
	if m.validateErr != nil {
		return 0, m.validateErr
	}

	return 5 * time.Minute, nil
}

func (m *fakeBookingLockManager) Release(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) error {
	m.mutex.Lock()
	m.releaseCall++
	m.mutex.Unlock()

	return m.releaseErr
}

func TestBookingServiceConcurrentDoubleSubmit(
	t *testing.T,
) {
	showtime := newTestShowtime()

	store := &atomicBookingStore{}
	lockManager := &fakeBookingLockManager{}

	realtimePublisher := &recordingRealtimePublisher{}
	auditPublisher := &recordingAuditPublisher{}

	service := NewBookingService(
		store,
		&fakeShowtimeRepository{
			showtime: showtime,
		},
		lockManager,
		realtimePublisher,
		auditPublisher,
		time.Second,
	)

	const requestCount = 32

	userID := primitive.NewObjectID()

	type result struct {
		booking *models.Booking
		err     error
	}

	results := make(
		chan result,
		requestCount,
	)

	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	for index := 0; index < requestCount; index++ {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			<-start

			booking, err := service.ConfirmBooking(
				context.Background(),
				ConfirmBookingInput{
					ShowtimeID: showtime.ID,
					UserID:     userID,
					SeatCode:   "A1",
					LockID:     "same-lock-id",
				},
			)

			results <- result{
				booking: booking,
				err:     err,
			}
		}()
	}

	close(start)

	waitGroup.Wait()
	close(results)

	successCount := 0
	conflictCount := 0

	for result := range results {
		if result.err == nil {
			successCount++

			if result.booking == nil {
				t.Fatal(
					"successful booking returned nil",
				)
			}

			continue
		}

		if errors.Is(
			result.err,
			repository.ErrSeatUnavailable,
		) {
			conflictCount++
			continue
		}

		t.Fatalf(
			"unexpected booking error: %v",
			result.err,
		)
	}

	if successCount != 1 {
		t.Fatalf(
			"expected exactly 1 successful booking, got %d",
			successCount,
		)
	}

	if conflictCount != requestCount-1 {
		t.Fatalf(
			"expected %d conflicts, got %d",
			requestCount-1,
			conflictCount,
		)
	}

	if store.Count() != 1 {
		t.Fatalf(
			"expected exactly 1 stored booking, got %d",
			store.Count(),
		)
	}

	realtimeEvents := realtimePublisher.Snapshot()

	if len(realtimeEvents) != 1 {
		t.Fatalf(
			"expected exactly 1 realtime event, got %d",
			len(realtimeEvents),
		)
	}

	if realtimeEvents[0].Status != models.SeatStatusBooked {
		t.Fatalf(
			"expected BOOKED event, got %s",
			realtimeEvents[0].Status,
		)
	}

	auditEvents := auditPublisher.Snapshot()

	if len(auditEvents) != 1 {
		t.Fatalf(
			"expected exactly 1 audit event, got %d",
			len(auditEvents),
		)
	}

	if auditEvents[0].Name() !=
		events.BookingConfirmedEventType {
		t.Fatalf(
			"expected event %s, got %s",
			events.BookingConfirmedEventType,
			auditEvents[0].Name(),
		)
	}
}

func (s *atomicBookingStore) ConfirmMany(
	ctx context.Context,
	bookings []*models.Booking,
) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.seatAlreadyBooked {
		return repository.ErrSeatUnavailable
	}
	s.seatAlreadyBooked = true
	for _, booking := range bookings {
		s.bookings = append(s.bookings, *booking)
	}
	return nil
}
