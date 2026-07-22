package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/redislock"

	"cinema-booking/internal/realtime"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidSeatData   = errors.New("invalid seat data")
	ErrSeatNotFound      = errors.New("seat not found")
	ErrSeatAlreadyBooked = errors.New("seat is already booked")
)

type SeatShowtimeRepository interface {
	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)
}

type SeatLockRepository interface {
	Acquire(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
	) (*redislock.AcquireResult, error)

	Release(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
		lockID string,
	) error

	GetLockedSeatCodes(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCodes []string,
	) (map[string]bool, error)
}

type SeatService struct {
	showtimeRepository SeatShowtimeRepository
	lockRepository     SeatLockRepository

	realtimePublisher realtime.Publisher

	auditPublisher      AuditEventPublisher
	auditPublishTimeout time.Duration
}

func NewSeatService(
	showtimeRepository SeatShowtimeRepository,
	lockRepository SeatLockRepository,
	realtimePublisher realtime.Publisher,
	auditPublisher AuditEventPublisher,
	auditPublishTimeout time.Duration,
) *SeatService {
	if showtimeRepository == nil {
		panic("seat service: showtime repository is nil")
	}

	if lockRepository == nil {
		panic("seat service: lock repository is nil")
	}

	if realtimePublisher == nil {
		panic("seat service: realtime publisher is nil")
	}

	if auditPublisher == nil {
		panic("seat service: audit publisher is nil")
	}

	if auditPublishTimeout <= 0 {
		panic("seat service: audit timeout must be positive")
	}

	return &SeatService{
		showtimeRepository: showtimeRepository,
		lockRepository:     lockRepository,

		realtimePublisher: realtimePublisher,

		auditPublisher:      auditPublisher,
		auditPublishTimeout: auditPublishTimeout,
	}
}
func (s *SeatService) LockSeat(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
) (*redislock.AcquireResult, error) {
	seatCode = normalizeServiceSeatCode(
		seatCode,
	)

	if showtimeID.IsZero() || userID.IsZero() || seatCode == "" {
		return nil, ErrInvalidSeatData
	}

	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		showtimeID,
	)
	if err != nil {
		return nil, err
	}

	if showtime.ShowtimeStatus !=
		models.ShowtimeStatusActive {
		return nil, ErrShowtimeUnavailable
	}

	if !showtime.StartTime.After(
		time.Now().UTC(),
	) {
		return nil, ErrShowtimeUnavailable
	}

	seat, found := findSeat(
		showtime.Seats,
		seatCode,
	)
	if !found {
		return nil, ErrSeatNotFound
	}

	if seat.Status == models.SeatStatusBooked {
		return nil, ErrSeatAlreadyBooked
	}

	if seat.Status != models.SeatStatusAvailable {
		return nil, ErrInvalidSeatData
	}

	result, err := s.lockRepository.Acquire(
		ctx,
		showtimeID,
		seatCode,
		userID,
	)
	if err != nil {
		if errors.Is(
			err,
			redislock.ErrSeatLockUnavailable,
		) {
			s.publishLockSystemError(
				userID,
				showtimeID,
				seatCode,
				"ACQUIRE_SEAT_LOCK",
				"SEAT_LOCK_UNAVAILABLE",
			)
		}

		return nil, fmt.Errorf(
			"acquire seat lock: %w",
			err,
		)
	}

	if !result.AlreadyOwned {
		expiresAt := result.ExpiresAt

		publishRealtimeBestEffort(
			s.realtimePublisher,
			realtime.SeatEvent{
				Type: realtime.EventSeatStatusChanged,

				ShowtimeID: showtimeID.Hex(),
				SeatCode:   seatCode,
				Status:     models.SeatStatusLocked,

				ExpiresAt:  &expiresAt,
				OccurredAt: time.Now().UTC(),
			},
		)
	}

	return result, nil

}

func (s *SeatService) ReleaseSeat(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) error {
	seatCode = normalizeServiceSeatCode(
		seatCode,
	)
	lockID = strings.TrimSpace(lockID)

	if showtimeID.IsZero() || userID.IsZero() || seatCode == "" || lockID == "" {
		return ErrInvalidSeatData
	}

	if err := s.lockRepository.Release(
		ctx,
		showtimeID,
		seatCode,
		userID,
		lockID,
	); err != nil {
		if errors.Is(
			err,
			redislock.ErrSeatLockUnavailable,
		) {
			s.publishLockSystemError(
				userID,
				showtimeID,
				seatCode,
				"RELEASE_SEAT_LOCK",
				"SEAT_LOCK_UNAVAILABLE",
			)
		}

		return fmt.Errorf(
			"release seat lock: %w",
			err,
		)
	}

	publishRealtimeBestEffort(
		s.realtimePublisher,
		realtime.SeatEvent{
			Type: realtime.EventSeatStatusChanged,

			ShowtimeID: showtimeID.Hex(),
			SeatCode:   seatCode,
			Status:     models.SeatStatusAvailable,

			OccurredAt: time.Now().UTC(),
		},
	)

	seatReleasedEvent, eventErr :=
		events.NewSeatReleased(
			userID.Hex(),
			showtimeID.Hex(),
			seatCode,
			lockID,
			"USER_RELEASED",
		)

	if eventErr == nil {
		publishAuditBestEffort(
			s.auditPublisher,
			s.auditPublishTimeout,
			seatReleasedEvent,
		)
	}

	return nil
}

func (s *SeatService) GetSeatMap(
	ctx context.Context,
	showtimeID primitive.ObjectID,
) (*models.SeatMapResponse, error) {
	if showtimeID.IsZero() {
		return nil, ErrInvalidSeatData
	}

	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		showtimeID,
	)
	if err != nil {
		return nil, err
	}

	if showtime.ShowtimeStatus != models.ShowtimeStatusActive {
		return nil, ErrShowtimeUnavailable
	}

	seatCodes := make(
		[]string,
		0,
		len(showtime.Seats),
	)

	for _, seat := range showtime.Seats {
		if seat.Status ==
			models.SeatStatusAvailable {
			seatCodes = append(
				seatCodes,
				seat.Code,
			)
		}
	}

	lockedSeats, err :=
		s.lockRepository.GetLockedSeatCodes(
			ctx,
			showtimeID,
			seatCodes,
		)
	if err != nil {
		if errors.Is(err, redislock.ErrSeatLockUnavailable) {
			s.publishLockSystemError(
				primitive.NilObjectID,
				showtimeID,
				"",
				"READ_SEAT_LOCKS",
				"SEAT_LOCK_UNAVAILABLE",
			)
		}
		return nil, fmt.Errorf(
			"read locked seats: %w",
			err,
		)
	}

	showtimeCopy := *showtime

	showtimeCopy.Seats = append(
		[]models.Seat(nil),
		showtime.Seats...,
	)

	for index := range showtimeCopy.Seats {
		seat := &showtimeCopy.Seats[index]

		if seat.Status !=
			models.SeatStatusAvailable {
			continue
		}

		if lockedSeats[normalizeServiceSeatCode(seat.Code)] {
			seat.Status =
				models.SeatStatusLocked
		}
	}

	response := models.NewSeatMapResponse(
		&showtimeCopy,
	)

	return &response, nil
}

func findSeat(
	seats []models.Seat,
	seatCode string,
) (*models.Seat, bool) {
	seatCode = normalizeServiceSeatCode(
		seatCode,
	)

	for index := range seats {
		if normalizeServiceSeatCode(
			seats[index].Code,
		) == seatCode {
			return &seats[index], true
		}
	}

	return nil, false
}

func normalizeServiceSeatCode(
	seatCode string,
) string {
	return strings.ToUpper(
		strings.TrimSpace(seatCode),
	)
}

func (s *SeatService) HandleExpiredSeatLock(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
) error {
	seatCode = normalizeServiceSeatCode(seatCode)

	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		showtimeID,
	)
	if err != nil {
		return err
	}

	seat, found := findSeat(
		showtime.Seats,
		seatCode,
	)
	if !found {
		return ErrSeatNotFound
	}

	currentStatus := seat.Status

	/*
		Lock เก่าหมดอายุ แต่ User คนใหม่อาจ Lock ต่อทันที
		จึงต้องตรวจ Redis ก่อน Broadcast AVAILABLE
	*/
	if currentStatus == models.SeatStatusAvailable {
		lockedSeats, err :=
			s.lockRepository.GetLockedSeatCodes(
				ctx,
				showtimeID,
				[]string{seatCode},
			)
		if err != nil {
			return err
		}

		if lockedSeats[seatCode] {
			currentStatus = models.SeatStatusLocked
		}
	}

	timeoutEvent, eventErr :=
		events.NewBookingTimedOut(
			showtimeID.Hex(),
			seatCode,
			"LOCK_EXPIRED",
		)

	if eventErr == nil {
		publishAuditBestEffort(
			s.auditPublisher,
			s.auditPublishTimeout,
			timeoutEvent,
		)
	}

	return s.realtimePublisher.Publish(
		ctx,
		realtime.SeatEvent{
			Type: realtime.EventSeatStatusChanged,

			ShowtimeID: showtimeID.Hex(),
			SeatCode:   seatCode,
			Status:     currentStatus,

			OccurredAt: time.Now().UTC(),
		},
	)
}

func (s *SeatService) publishLockSystemError(
	userID primitive.ObjectID,
	showtimeID primitive.ObjectID,
	seatCode string,
	operation string,
	errorCode string,
) {
	userIDValue := ""
	if !userID.IsZero() {
		userIDValue = userID.Hex()
	}

	event, err := events.NewSystemError(
		events.SystemErrorData{
			Component:    "REDIS_LOCK",
			Operation:    operation,
			ErrorCode:    errorCode,
			ErrorMessage: "Redis seat lock operation failed",

			UserID:     userIDValue,
			ShowtimeID: showtimeID.Hex(),
			SeatCode:   seatCode,
		},
	)
	if err != nil {
		return
	}

	publishAuditBestEffort(
		s.auditPublisher,
		s.auditPublishTimeout,
		event,
	)
}
