package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"
	"cinema-booking/internal/redislock"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BookingService interface {
	ConfirmBooking(
		ctx context.Context,
		input services.ConfirmBookingInput,
	) (*models.Booking, error)
	ConfirmBookings(
		ctx context.Context,
		input services.ConfirmBookingsInput,
	) ([]*models.Booking, error)

	GetMyBooking(
		ctx context.Context,
		bookingID primitive.ObjectID,
		userID primitive.ObjectID,
	) (*models.Booking, error)

	ListMyBookings(
		ctx context.Context,
		userID primitive.ObjectID,
		page int,
		limit int,
	) (*services.BookingListResult, error)
}

type BookingHandler struct {
	bookingService BookingService
}

type ConfirmBookingRequest struct {
	ShowtimeID string `json:"showtime_id" binding:"required"`
	SeatCode   string `json:"seat_code" binding:"required"`
	LockID     string `json:"lock_id" binding:"required"`
}

type ConfirmManyBookingsRequest struct {
	ShowtimeID string `json:"showtime_id" binding:"required"`
	Seats      []struct {
		SeatCode string `json:"seat_code" binding:"required"`
		LockID   string `json:"lock_id" binding:"required"`
	} `json:"seats" binding:"required,min=1,max=10,dive"`
}

func (h *BookingHandler) ConfirmMany(c *gin.Context) {
	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication_context_missing", "message": "Unable to read authenticated user"})
		return
	}

	var request ConfirmManyBookingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "showtime_id and 1-10 locked seats are required"})
		return
	}
	showtimeID, err := primitive.ObjectIDFromHex(strings.TrimSpace(request.ShowtimeID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_showtime_id", "message": "Showtime ID is invalid"})
		return
	}

	seats := make([]services.ConfirmSeatInput, 0, len(request.Seats))
	for _, seat := range request.Seats {
		seats = append(seats, services.ConfirmSeatInput{SeatCode: seat.SeatCode, LockID: seat.LockID})
	}
	bookings, err := h.bookingService.ConfirmBookings(c.Request.Context(), services.ConfirmBookingsInput{
		ShowtimeID: showtimeID, UserID: userID, Seats: seats,
	})
	if err != nil {
		h.handleBookingError(c, err)
		return
	}

	responses := make([]models.BookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		responses = append(responses, models.NewBookingResponse(booking))
	}
	c.JSON(http.StatusCreated, gin.H{"data": responses})
}

type BookingListResponse struct {
	Data []models.BookingResponse `json:"data"`

	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewBookingHandler(
	bookingService BookingService,
) *BookingHandler {
	if bookingService == nil {
		panic("booking handler: booking service is nil")
	}

	return &BookingHandler{
		bookingService: bookingService,
	}
}

func (h *BookingHandler) Confirm(
	c *gin.Context,
) {
	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	var request ConfirmBookingRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "showtime_id, seat_code, and lock_id are required",
		})
		return
	}

	showtimeID, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(request.ShowtimeID),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_showtime_id",
			"message": "Showtime ID is invalid",
		})
		return
	}

	booking, err := h.bookingService.ConfirmBooking(
		c.Request.Context(),
		services.ConfirmBookingInput{
			ShowtimeID: showtimeID,
			UserID:     userID,
			SeatCode:   request.SeatCode,
			LockID:     request.LockID,
		},
	)
	if err != nil {
		h.handleBookingError(c, err)
		return
	}

	c.JSON(
		http.StatusCreated,
		models.NewBookingResponse(booking),
	)
}

func (h *BookingHandler) ListMine(
	c *gin.Context,
) {
	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	page := parsePositiveInt(
		c.Query("page"),
		1,
	)

	limit := parsePositiveInt(
		c.Query("limit"),
		10,
	)

	result, err := h.bookingService.ListMyBookings(
		c.Request.Context(),
		userID,
		page,
		limit,
	)
	if err != nil {
		h.handleBookingError(c, err)
		return
	}

	c.JSON(http.StatusOK, BookingListResponse{
		Data: models.NewBookingResponses(
			result.Bookings,
		),

		Page:       result.Page,
		Limit:      result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

func (h *BookingHandler) GetMine(
	c *gin.Context,
) {
	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	bookingID, err := primitive.ObjectIDFromHex(
		c.Param("bookingID"),
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_booking_id",
			"message": "Booking ID is invalid",
		})
		return
	}

	booking, err := h.bookingService.GetMyBooking(
		c.Request.Context(),
		bookingID,
		userID,
	)
	if err != nil {
		h.handleBookingError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewBookingResponse(booking),
	)
}

func (h *BookingHandler) handleBookingError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, repository.ErrBookingNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "booking_not_found",
			"message": "Booking not found",
		})

	case errors.Is(err, repository.ErrShowtimeNotFound),
		errors.Is(err, services.ErrShowtimeUnavailable):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "showtime_not_found",
			"message": "Showtime not found or unavailable",
		})

	case errors.Is(err, services.ErrSeatNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "seat_not_found",
			"message": "Seat not found",
		})

	case errors.Is(err, services.ErrSeatAlreadyBooked),
		errors.Is(err, repository.ErrSeatUnavailable),
		errors.Is(err, repository.ErrBookingConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "seat_unavailable",
			"message": "Seat is no longer available",
		})

	case errors.Is(err, redislock.ErrSeatLockNotFound),
		errors.Is(err, services.ErrBookingLockExpired):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "seat_lock_expired",
			"message": "Seat lock has expired",
		})

	case errors.Is(err, redislock.ErrSeatLockNotOwned):
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "seat_lock_not_owned",
			"message": "You do not own this seat lock",
		})

	case errors.Is(err, services.ErrInvalidBookingData),
		errors.Is(err, repository.ErrInvalidBooking),
		errors.Is(err, redislock.ErrInvalidSeatLock):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_booking_data",
			"message": "Booking request is invalid",
		})

	case errors.Is(err, redislock.ErrSeatLockUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "seat_lock_unavailable",
			"message": "Seat lock service is temporarily unavailable",
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to complete booking",
		})
	}
}
