package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidAdminBookingFilter = errors.New(
	"Invalid admin booking filter",
)

type AdminBookingRepository interface {
	FindAllAdmin(
		ctx context.Context,
		filter repository.AdminBookingFilter,
		skip int64,
		limit int64,
	) ([]models.AdminBookingView, int64, error)

	FindAdminByID(
		ctx context.Context,
		bookingID primitive.ObjectID,
	) (*models.AdminBookingView, error)
}

type AdminBookingFilter struct {
	UserID     *primitive.ObjectID
	MovieID    *primitive.ObjectID
	ShowtimeID *primitive.ObjectID

	Status *models.BookingStatus

	From *time.Time
	To   *time.Time
}

type AdminBookingListResult struct {
	Bookings []models.AdminBookingView

	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

type AdminBookingService struct {
	bookingRepository AdminBookingRepository
}

func NewAdminBookingService(
	bookingRepository AdminBookingRepository,
) *AdminBookingService {
	if bookingRepository == nil {
		panic(
			"admin booking service: repository is nil",
		)
	}

	return &AdminBookingService{
		bookingRepository: bookingRepository,
	}
}

func (s *AdminBookingService) ListBookings(
	ctx context.Context,
	filter AdminBookingFilter,
	page int,
	limit int,
) (*AdminBookingListResult, error) {
	if err := validateAdminBookingFilter(filter); err != nil {
		return nil, err
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	skip := int64((page - 1) * limit)

	bookings, total, err :=
		s.bookingRepository.FindAllAdmin(
			ctx,
			repository.AdminBookingFilter{
				UserID:     filter.UserID,
				MovieID:    filter.MovieID,
				ShowtimeID: filter.ShowtimeID,
				Status:     filter.Status,
				From:       filter.From,
				To:         filter.To,
			},
			skip,
			int64(limit),
		)
	if err != nil {
		return nil, fmt.Errorf(
			"list admin bookings: %w",
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

	return &AdminBookingListResult{
		Bookings: bookings,

		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *AdminBookingService) GetBooking(
	ctx context.Context,
	bookingID primitive.ObjectID,
) (*models.AdminBookingView, error) {
	if bookingID.IsZero() {
		return nil, ErrInvalidAdminBookingFilter
	}

	return s.bookingRepository.FindAdminByID(
		ctx,
		bookingID,
	)
}

func validateAdminBookingFilter(
	filter AdminBookingFilter,
) error {
	if filter.Status != nil &&
		!filter.Status.IsValid() {
		return fmt.Errorf(
			"%w: invalid booking status",
			ErrInvalidAdminBookingFilter,
		)
	}

	if filter.From != nil &&
		filter.To != nil &&
		!filter.To.After(*filter.From) {
		return fmt.Errorf(
			"%w: to must be after from",
			ErrInvalidAdminBookingFilter,
		)
	}

	if filter.From != nil &&
		filter.To != nil &&
		filter.To.Sub(*filter.From) >
			366*24*time.Hour {
		return fmt.Errorf(
			"%w: date range must not exceed 366 days",
			ErrInvalidAdminBookingFilter,
		)
	}

	return nil
}
