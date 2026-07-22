package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/redislock"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeSeatLockRepository struct {
	acquireResult *redislock.AcquireResult
	acquireErr    error
	releaseErr    error
	getLockedErr  error

	lockedSeats map[string]bool
}

func (f *fakeSeatLockRepository) Acquire(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
) (*redislock.AcquireResult, error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}

	if f.acquireResult != nil {
		resultCopy := *f.acquireResult
		return &resultCopy, nil
	}

	return &redislock.AcquireResult{
		LockID:    primitive.NewObjectID().Hex(),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
		ExpiresIn: 300,
	}, nil
}

func (f *fakeSeatLockRepository) Release(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) error {
	return f.releaseErr
}

func (f *fakeSeatLockRepository) GetLockedSeatCodes(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCodes []string,
) (map[string]bool, error) {
	if f.getLockedErr != nil {
		return nil, f.getLockedErr
	}

	result := make(map[string]bool)

	for seatCode, locked := range f.lockedSeats {
		result[seatCode] = locked
	}

	return result, nil
}

func TestSeatServiceLockAndReleasePublishesEvents(
	t *testing.T,
) {
	showtime := newTestShowtime()

	showtimeRepository := &fakeShowtimeRepository{
		showtime: showtime,
	}

	lockRepository := &fakeSeatLockRepository{
		acquireResult: &redislock.AcquireResult{
			LockID: "test-lock-id",

			ExpiresAt: time.Now().
				UTC().
				Add(5 * time.Minute),

			ExpiresIn: 300,
		},
	}

	realtimePublisher := &recordingRealtimePublisher{}
	auditPublisher := &recordingAuditPublisher{}

	service := NewSeatService(
		showtimeRepository,
		lockRepository,
		realtimePublisher,
		auditPublisher,
		time.Second,
	)

	userID := primitive.NewObjectID()

	result, err := service.LockSeat(
		context.Background(),
		showtime.ID,
		"A1",
		userID,
	)
	if err != nil {
		t.Fatalf("lock seat failed: %v", err)
	}

	if result.LockID != "test-lock-id" {
		t.Fatalf(
			"expected lock ID test-lock-id, got %s",
			result.LockID,
		)
	}

	realtimeEvents := realtimePublisher.Snapshot()

	if len(realtimeEvents) != 1 {
		t.Fatalf(
			"expected 1 realtime event, got %d",
			len(realtimeEvents),
		)
	}

	if realtimeEvents[0].Status != models.SeatStatusLocked {
		t.Fatalf(
			"expected LOCKED status, got %s",
			realtimeEvents[0].Status,
		)
	}

	err = service.ReleaseSeat(
		context.Background(),
		showtime.ID,
		"A1",
		userID,
		result.LockID,
	)
	if err != nil {
		t.Fatalf("release seat failed: %v", err)
	}

	realtimeEvents = realtimePublisher.Snapshot()

	if len(realtimeEvents) != 2 {
		t.Fatalf(
			"expected 2 realtime events, got %d",
			len(realtimeEvents),
		)
	}

	if realtimeEvents[1].Status != models.SeatStatusAvailable {
		t.Fatalf(
			"expected AVAILABLE status, got %s",
			realtimeEvents[1].Status,
		)
	}

	auditEvents := auditPublisher.Snapshot()

	if len(auditEvents) != 1 {
		t.Fatalf(
			"expected 1 audit event, got %d",
			len(auditEvents),
		)
	}

	if auditEvents[0].Name() != events.SeatReleasedEventType {
		t.Fatalf(
			"expected event %s, got %s",
			events.SeatReleasedEventType,
			auditEvents[0].Name(),
		)
	}
}

func TestSeatServiceGetSeatMapOverlaysRedisLock(
	t *testing.T,
) {
	showtime := newTestShowtime()

	lockRepository := &fakeSeatLockRepository{
		lockedSeats: map[string]bool{
			"A1": true,
		},
	}

	service := NewSeatService(
		&fakeShowtimeRepository{
			showtime: showtime,
		},
		lockRepository,
		&recordingRealtimePublisher{},
		&recordingAuditPublisher{},
		time.Second,
	)

	response, err := service.GetSeatMap(
		context.Background(),
		showtime.ID,
	)
	if err != nil {
		t.Fatalf("get seat map failed: %v", err)
	}

	if response.Summary.Locked != 1 {
		t.Fatalf(
			"expected 1 locked seat, got %d",
			response.Summary.Locked,
		)
	}

	if response.Summary.Available != 1 {
		t.Fatalf(
			"expected 1 available seat, got %d",
			response.Summary.Available,
		)
	}

	if response.Seats[0].Status != models.SeatStatusLocked {
		t.Fatalf(
			"expected A1 to be LOCKED, got %s",
			response.Seats[0].Status,
		)
	}

	/*
		ตรวจว่า Service ไม่ได้แก้ไข Model ต้นฉบับ
	*/
	if showtime.Seats[0].Status != models.SeatStatusAvailable {
		t.Fatalf(
			"source showtime was unexpectedly modified",
		)
	}
}

func TestSeatServiceRedisFailurePublishesSystemError(
	t *testing.T,
) {
	showtime := newTestShowtime()

	auditPublisher := &recordingAuditPublisher{}

	service := NewSeatService(
		&fakeShowtimeRepository{
			showtime: showtime,
		},
		&fakeSeatLockRepository{
			acquireErr: redislock.ErrSeatLockUnavailable,
		},
		&recordingRealtimePublisher{},
		auditPublisher,
		time.Second,
	)

	_, err := service.LockSeat(
		context.Background(),
		showtime.ID,
		"A1",
		primitive.NewObjectID(),
	)

	if !errors.Is(
		err,
		redislock.ErrSeatLockUnavailable,
	) {
		t.Fatalf(
			"expected ErrSeatLockUnavailable, got %v",
			err,
		)
	}

	auditEvents := auditPublisher.Snapshot()

	if len(auditEvents) != 1 {
		t.Fatalf(
			"expected 1 system error event, got %d",
			len(auditEvents),
		)
	}

	if auditEvents[0].Name() != events.SystemErrorEventType {
		t.Fatalf(
			"expected event %s, got %s",
			events.SystemErrorEventType,
			auditEvents[0].Name(),
		)
	}
}

func TestSeatServiceSeatMapRedisFailurePublishesSystemError(t *testing.T) {
	showtime := newTestShowtime()
	auditPublisher := &recordingAuditPublisher{}
	lockRepository := &fakeSeatLockRepository{
		getLockedErr: redislock.ErrSeatLockUnavailable,
	}
	service := NewSeatService(
		&fakeShowtimeRepository{showtime: showtime},
		lockRepository,
		&recordingRealtimePublisher{},
		auditPublisher,
		time.Second,
	)

	_, err := service.GetSeatMap(context.Background(), showtime.ID)
	if !errors.Is(err, redislock.ErrSeatLockUnavailable) {
		t.Fatalf("expected ErrSeatLockUnavailable, got %v", err)
	}

	auditEvents := auditPublisher.Snapshot()
	if len(auditEvents) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(auditEvents))
	}
	if auditEvents[0].Name() != events.SystemErrorEventType {
		t.Fatalf("expected %s, got %s", events.SystemErrorEventType, auditEvents[0].Name())
	}
}
