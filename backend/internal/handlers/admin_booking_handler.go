package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminBookingService interface {
	ListBookings(
		ctx context.Context,
		filter services.AdminBookingFilter,
		page int,
		limit int,
	) (*services.AdminBookingListResult, error)

	GetBooking(
		ctx context.Context,
		bookingID primitive.ObjectID,
	) (*models.AdminBookingView, error)
}

type AdminBookingHandler struct {
	adminBookingService AdminBookingService
}

type AdminBookingListResponse struct {
	Data []models.AdminBookingView `json:"data"`

	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewAdminBookingHandler(
	adminBookingService AdminBookingService,
) *AdminBookingHandler {
	if adminBookingService == nil {
		panic(
			"admin booking handler: service is nil",
		)
	}

	return &AdminBookingHandler{
		adminBookingService: adminBookingService,
	}
}

func (h *AdminBookingHandler) List(
	c *gin.Context,
) {
	filter, err := parseAdminBookingFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_booking_filter",
			"message": err.Error(),
		})
		return
	}

	page := parsePositiveInt(
		c.Query("page"),
		1,
	)

	limit := parsePositiveInt(
		c.Query("limit"),
		20,
	)

	result, err := h.adminBookingService.ListBookings(
		c.Request.Context(),
		filter,
		page,
		limit,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, AdminBookingListResponse{
		Data: result.Bookings,

		Page:       result.Page,
		Limit:      result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

func (h *AdminBookingHandler) Get(
	c *gin.Context,
) {
	bookingID, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(
			c.Param("bookingID"),
		),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_booking_id",
			"message": "Booking ID is invalid",
		})
		return
	}

	booking, err :=
		h.adminBookingService.GetBooking(
			c.Request.Context(),
			bookingID,
		)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, booking)
}

func (h *AdminBookingHandler) handleError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		repository.ErrBookingNotFound,
	):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "booking_not_found",
			"message": "Booking not found",
		})

	case errors.Is(
		err,
		services.ErrInvalidAdminBookingFilter,
	),
		errors.Is(
			err,
			repository.ErrInvalidBooking,
		):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_booking_filter",
			"message": err.Error(),
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to retrieve bookings",
		})
	}
}

func parseAdminBookingFilter(
	c *gin.Context,
) (services.AdminBookingFilter, error) {
	userID, err := parseOptionalObjectID(
		c.Query("user_id"),
		"user_id",
	)
	if err != nil {
		return services.AdminBookingFilter{}, err
	}

	movieID, err := parseOptionalObjectID(
		c.Query("movie_id"),
		"movie_id",
	)
	if err != nil {
		return services.AdminBookingFilter{}, err
	}

	showtimeID, err := parseOptionalObjectID(
		c.Query("showtime_id"),
		"showtime_id",
	)
	if err != nil {
		return services.AdminBookingFilter{}, err
	}

	var status *models.BookingStatus

	rawStatus := strings.ToUpper(
		strings.TrimSpace(
			c.Query("status"),
		),
	)

	if rawStatus != "" {
		parsedStatus := models.BookingStatus(rawStatus)

		if !parsedStatus.IsValid() {
			return services.AdminBookingFilter{},
				errors.New(
					"status must be BOOKED or CANCELLED",
				)
		}

		status = &parsedStatus
	}

	from, err := parseOptionalRFC3339(
		c.Query("from"),
		"from",
	)
	if err != nil {
		return services.AdminBookingFilter{}, err
	}

	to, err := parseOptionalRFC3339(
		c.Query("to"),
		"to",
	)
	if err != nil {
		return services.AdminBookingFilter{}, err
	}

	return services.AdminBookingFilter{
		UserID:     userID,
		MovieID:    movieID,
		ShowtimeID: showtimeID,
		Status:     status,
		From:       from,
		To:         to,
	}, nil
}

func parseOptionalObjectID(
	rawValue string,
	fieldName string,
) (*primitive.ObjectID, error) {
	rawValue = strings.TrimSpace(rawValue)

	if rawValue == "" {
		return nil, nil
	}

	value, err := primitive.ObjectIDFromHex(rawValue)
	if err != nil {
		return nil, errors.New(
			fieldName + " is invalid",
		)
	}

	return &value, nil
}

func parseOptionalRFC3339(
	rawValue string,
	fieldName string,
) (*time.Time, error) {
	rawValue = strings.TrimSpace(rawValue)

	if rawValue == "" {
		return nil, nil
	}

	value, err := time.Parse(
		time.RFC3339,
		rawValue,
	)
	if err != nil {
		return nil, errors.New(
			fieldName + " must use RFC3339 format",
		)
	}

	value = value.UTC()

	return &value, nil
}
