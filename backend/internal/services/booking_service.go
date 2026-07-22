package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/realtime"
	"cinema-booking/internal/redislock"
	"cinema-booking/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidBookingData = errors.New("invalid booking data")
	ErrBookingLockExpired = errors.New("booking seat lock expired")
)

type BookingStore interface {
	Confirm(
		ctx context.Context,
		booking *models.Booking,
	) error
	ConfirmMany(ctx context.Context, bookings []*models.Booking) error

	FindByIDAndUserID(
		ctx context.Context,
		bookingID primitive.ObjectID,
		userID primitive.ObjectID,
	) (*models.Booking, error)

	FindByUserID(
		ctx context.Context,
		userID primitive.ObjectID,
		skip int64,
		limit int64,
	) ([]models.Booking, int64, error)
}

type BookingShowtimeRepository interface {
	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)
}

type BookingSeatLockManager interface {
	ValidateOwnership(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
		lockID string,
	) (time.Duration, error)

	Release(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
		lockID string,
	) error
}

type BookingService struct {
	bookingRepository  BookingStore
	showtimeRepository BookingShowtimeRepository
	seatLockManager    BookingSeatLockManager

	realtimePublisher realtime.Publisher
	auditPublisher    AuditEventPublisher

	auditPublishTimeout time.Duration
}

type ConfirmBookingInput struct {
	ShowtimeID primitive.ObjectID
	UserID     primitive.ObjectID

	SeatCode string
	LockID   string
}

type ConfirmSeatInput struct {
	SeatCode string
	LockID   string
}

type ConfirmBookingsInput struct {
	ShowtimeID primitive.ObjectID
	UserID     primitive.ObjectID
	Seats      []ConfirmSeatInput
}

func (s *BookingService) ConfirmBookings(
	ctx context.Context,
	input ConfirmBookingsInput,
) ([]*models.Booking, error) {
	if input.ShowtimeID.IsZero() || input.UserID.IsZero() || len(input.Seats) < 1 || len(input.Seats) > 10 {
		return nil, ErrInvalidBookingData
	}

	showtime, err := s.showtimeRepository.FindByID(ctx, input.ShowtimeID)
	if err != nil {
		return nil, err
	}
	if showtime.ShowtimeStatus != models.ShowtimeStatusActive || !showtime.StartTime.After(time.Now().UTC()) {
		return nil, ErrShowtimeUnavailable
	}

	seen := make(map[string]struct{}, len(input.Seats))
	bookings := make([]*models.Booking, 0, len(input.Seats))
	confirmedAt := time.Now().UTC()
	for _, requestedSeat := range input.Seats {
		seatCode := normalizeServiceSeatCode(requestedSeat.SeatCode)
		lockID := strings.TrimSpace(requestedSeat.LockID)
		if seatCode == "" || lockID == "" {
			return nil, ErrInvalidBookingData
		}
		if _, duplicate := seen[seatCode]; duplicate {
			return nil, ErrInvalidBookingData
		}
		seen[seatCode] = struct{}{}

		seat, found := findSeat(showtime.Seats, seatCode)
		if !found {
			return nil, ErrSeatNotFound
		}
		if seat.Status == models.SeatStatusBooked {
			return nil, ErrSeatAlreadyBooked
		}
		if seat.Status != models.SeatStatusAvailable {
			return nil, repository.ErrSeatUnavailable
		}

		remainingTTL, err := s.seatLockManager.ValidateOwnership(
			ctx, input.ShowtimeID, seatCode, input.UserID, lockID,
		)
		if err != nil {
			return nil, fmt.Errorf("validate booking seat lock: %w", err)
		}
		if remainingTTL <= 0 {
			return nil, ErrBookingLockExpired
		}

		bookingID := primitive.NewObjectID()
		bookings = append(bookings, &models.Booking{
			ID: bookingID, BookingCode: generateBookingCode(bookingID),
			UserID: input.UserID, MovieID: showtime.MovieID, ShowtimeID: showtime.ID,
			SeatCode: seatCode, HallName: showtime.HallName, ShowtimeStart: showtime.StartTime,
			Price: showtime.Price, Currency: showtime.Currency,
			Status: models.BookingStatusBooked, ConfirmedAt: confirmedAt,
			CreatedAt: confirmedAt, UpdatedAt: confirmedAt,
		})
	}

	if err := s.bookingRepository.ConfirmMany(ctx, bookings); err != nil {
		return nil, fmt.Errorf("confirm bookings: %w", err)
	}

	for index, booking := range bookings {
		requestedSeat := input.Seats[index]
		_ = s.seatLockManager.Release(
			ctx, input.ShowtimeID, booking.SeatCode, input.UserID, strings.TrimSpace(requestedSeat.LockID),
		)
		publishRealtimeBestEffort(s.realtimePublisher, realtime.SeatEvent{
			Type: realtime.EventSeatStatusChanged, ShowtimeID: booking.ShowtimeID.Hex(),
			SeatCode: booking.SeatCode, Status: models.SeatStatusBooked,
			BookingID: booking.ID.Hex(), OccurredAt: time.Now().UTC(),
		})
		if event, eventErr := events.NewBookingConfirmed(booking); eventErr == nil {
			publishAuditBestEffort(s.auditPublisher, s.auditPublishTimeout, event)
		}
	}

	return bookings, nil
}

type BookingListResult struct {
	Bookings []models.Booking

	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

func NewBookingService(
	bookingRepository BookingStore,
	showtimeRepository BookingShowtimeRepository,
	seatLockManager BookingSeatLockManager,
	realtimePublisher realtime.Publisher,
	auditPublisher AuditEventPublisher,
	auditPublishTimeout time.Duration,
) *BookingService {
	if bookingRepository == nil {
		panic("booking service: booking repository is nil")
	}

	if showtimeRepository == nil {
		panic("booking service: showtime repository is nil")
	}

	if seatLockManager == nil {
		panic("booking service: seat lock manager is nil")
	}

	return &BookingService{
		bookingRepository:  bookingRepository,
		showtimeRepository: showtimeRepository,
		seatLockManager:    seatLockManager,

		realtimePublisher: realtimePublisher,
		auditPublisher:    auditPublisher,

		auditPublishTimeout: auditPublishTimeout,
	}
}

func (s *BookingService) ConfirmBooking(
	ctx context.Context,
	input ConfirmBookingInput,
) (*models.Booking, error) {
	input.SeatCode = normalizeServiceSeatCode(
		input.SeatCode,
	)
	input.LockID = strings.TrimSpace(input.LockID)

	if input.ShowtimeID.IsZero() ||
		input.UserID.IsZero() ||
		input.SeatCode == "" ||
		input.LockID == "" {
		return nil, ErrInvalidBookingData
	}

	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		input.ShowtimeID,
	)
	if err != nil {
		return nil, err
	}

	if showtime.ShowtimeStatus != models.ShowtimeStatusActive ||
		!showtime.StartTime.After(time.Now().UTC()) {
		return nil, ErrShowtimeUnavailable
	}

	seat, found := findSeat(
		showtime.Seats,
		input.SeatCode,
	)
	if !found {
		return nil, ErrSeatNotFound
	}

	if seat.Status == models.SeatStatusBooked {
		return nil, ErrSeatAlreadyBooked
	}

	if seat.Status != models.SeatStatusAvailable {
		return nil, repository.ErrSeatUnavailable
	}

	remainingTTL, err := s.seatLockManager.ValidateOwnership(
		ctx,
		input.ShowtimeID,
		input.SeatCode,
		input.UserID,
		input.LockID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"validate booking seat lock: %w",
			err,
		)
	}

	if remainingTTL <= 0 {
		return nil, ErrBookingLockExpired
	}

	bookingID := primitive.NewObjectID()
	confirmedAt := time.Now().UTC()

	booking := &models.Booking{
		ID: bookingID,

		BookingCode: generateBookingCode(
			bookingID,
		),

		UserID:     input.UserID,
		MovieID:    showtime.MovieID,
		ShowtimeID: showtime.ID,

		SeatCode: input.SeatCode,
		HallName: showtime.HallName,

		ShowtimeStart: showtime.StartTime,

		Price:    showtime.Price,
		Currency: showtime.Currency,

		Status:      models.BookingStatusBooked,
		ConfirmedAt: confirmedAt,
		CreatedAt:   confirmedAt,
		UpdatedAt:   confirmedAt,
	}

	err = s.bookingRepository.Confirm(
		ctx,
		booking,
	)
	if err != nil {
		if errors.Is(err, repository.ErrSeatUnavailable) ||
			errors.Is(err, repository.ErrBookingConflict) {
			/*
				ที่นั่งถูก Booking ไปแล้ว จึงลบ Lock เดิม
				เพื่อลดเวลาที่สถานะ LOCKED ค้างอยู่
			*/
			_ = s.seatLockManager.Release(
				ctx,
				input.ShowtimeID,
				input.SeatCode,
				input.UserID,
				input.LockID,
			)
		}

		return nil, fmt.Errorf(
			"confirm booking: %w",
			err,
		)
	}

	/*
		Booking สำเร็จแล้ว MongoDB เป็น Source of Truth

		หากลบ Redis Lock ไม่สำเร็จ ไม่ควรตอบว่า Booking ล้มเหลว
		เพราะ Lock จะหมดอายุตาม TTL และ Seat Map จะยึด BOOKED
		จาก MongoDB เป็นหลัก
	*/
	if err := s.seatLockManager.Release(
		ctx,
		input.ShowtimeID,
		input.SeatCode,
		input.UserID,
		input.LockID,
	); err != nil &&
		!errors.Is(err, redislock.ErrSeatLockNotFound) {
		log.Printf(
			"release Redis lock after booking %s: %v",
			booking.BookingCode,
			err,
		)
	}

	publishRealtimeBestEffort(
		s.realtimePublisher,
		realtime.SeatEvent{
			Type: realtime.EventSeatStatusChanged,

			ShowtimeID: booking.ShowtimeID.Hex(),
			SeatCode:   booking.SeatCode,
			Status:     models.SeatStatusBooked,

			BookingID:  booking.ID.Hex(),
			OccurredAt: time.Now().UTC(),
		},
	)

	bookingConfirmedEvent, eventErr :=
		events.NewBookingConfirmed(booking)

	if eventErr != nil {
		log.Printf(
			"create booking confirmed event %s: %v",
			booking.BookingCode,
			eventErr,
		)
	} else {
		publishAuditBestEffort(
			s.auditPublisher,
			s.auditPublishTimeout,
			bookingConfirmedEvent,
		)
	}

	return booking, nil
}

func (s *BookingService) GetMyBooking(
	ctx context.Context,
	bookingID primitive.ObjectID,
	userID primitive.ObjectID,
) (*models.Booking, error) {
	if bookingID.IsZero() || userID.IsZero() {
		return nil, ErrInvalidBookingData
	}

	return s.bookingRepository.FindByIDAndUserID(
		ctx,
		bookingID,
		userID,
	)
}

func (s *BookingService) ListMyBookings(
	ctx context.Context,
	userID primitive.ObjectID,
	page int,
	limit int,
) (*BookingListResult, error) {
	if userID.IsZero() {
		return nil, ErrInvalidBookingData
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 10
	}

	if limit > 100 {
		limit = 100
	}

	skip := int64((page - 1) * limit)

	bookings, total, err := s.bookingRepository.FindByUserID(
		ctx,
		userID,
		skip,
		int64(limit),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list user bookings: %w",
			err,
		)
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(limit) - 1) /
				int64(limit),
		)
	}

	return &BookingListResult{
		Bookings: bookings,

		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func generateBookingCode(
	bookingID primitive.ObjectID,
) string {
	now := time.Now().UTC()

	idPart := strings.ToUpper(
		bookingID.Hex()[12:],
	)

	return fmt.Sprintf(
		"BK-%s-%s",
		now.Format("20060102"),
		idPart,
	)
}
