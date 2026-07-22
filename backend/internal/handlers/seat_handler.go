package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"
	"cinema-booking/internal/redislock"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SeatService interface {
	LockSeat(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
	) (*redislock.AcquireResult, error)

	ReleaseSeat(
		ctx context.Context,
		showtimeID primitive.ObjectID,
		seatCode string,
		userID primitive.ObjectID,
		lockID string,
	) error

	GetSeatMap(
		ctx context.Context,
		showtimeID primitive.ObjectID,
	) (*models.SeatMapResponse, error)
}

type SeatHandler struct {
	seatService SeatService
}

func NewSeatHandler(
	seatService SeatService,
) *SeatHandler {
	if seatService == nil {
		panic("seat handler: seat service is nil")
	}

	return &SeatHandler{
		seatService: seatService,
	}
}

type SeatLockResponse struct {
	ShowtimeID primitive.ObjectID `json:"showtime_id"`
	SeatCode   string             `json:"seat_code"`

	LockID       string    `json:"lock_id"`
	ExpiresAt    time.Time `json:"expires_at"`
	ExpiresIn    int64     `json:"expires_in"`
	AlreadyOwned bool      `json:"already_owned"`
}

func (h *SeatHandler) SeatMap(
	c *gin.Context,
) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	response, err := h.seatService.GetSeatMap(
		c.Request.Context(),
		showtimeID,
	)
	if err != nil {
		h.handleSeatError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		response,
	)
}

func (h *SeatHandler) Lock(
	c *gin.Context,
) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	seatCode := strings.ToUpper(
		strings.TrimSpace(
			c.Param("seatCode"),
		),
	)

	if seatCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_seat_code",
			"message": "Seat code is required",
		})
		return
	}

	result, err := h.seatService.LockSeat(
		c.Request.Context(),
		showtimeID,
		seatCode,
		userID,
	)
	if err != nil {
		h.handleSeatError(c, err)
		return
	}

	c.JSON(http.StatusOK, SeatLockResponse{
		ShowtimeID: showtimeID,
		SeatCode:   seatCode,

		LockID:       result.LockID,
		ExpiresAt:    result.ExpiresAt,
		ExpiresIn:    result.ExpiresIn,
		AlreadyOwned: result.AlreadyOwned,
	})
}

func (h *SeatHandler) Release(
	c *gin.Context,
) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	seatCode := strings.ToUpper(
		strings.TrimSpace(
			c.Param("seatCode"),
		),
	)

	lockID := strings.TrimSpace(
		c.GetHeader("X-Seat-Lock-Token"),
	)

	if seatCode == "" || lockID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_lock_request",
			"message": "Seat code and X-Seat-Lock-Token are required",
		})
		return
	}

	if err := h.seatService.ReleaseSeat(
		c.Request.Context(),
		showtimeID,
		seatCode,
		userID,
		lockID,
	); err != nil {
		h.handleSeatError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Seat lock released successfully",
		"seat_code": seatCode,
	})
}

func (h *SeatHandler) handleSeatError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		repository.ErrShowtimeNotFound,
	),
		errors.Is(
			err,
			services.ErrShowtimeUnavailable,
		):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "showtime_not_found",
			"message": "Showtime not found or unavailable",
		})

	case errors.Is(
		err,
		services.ErrSeatNotFound,
	):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "seat_not_found",
			"message": "Seat not found",
		})

	case errors.Is(
		err,
		services.ErrSeatAlreadyBooked,
	):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "seat_already_booked",
			"message": "Seat is already booked",
		})

	case errors.Is(
		err,
		redislock.ErrSeatAlreadyLocked,
	):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "seat_already_locked",
			"message": "Seat is currently locked by another user",
		})

	case errors.Is(
		err,
		redislock.ErrSeatLockNotFound,
	):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "seat_lock_not_found",
			"message": "Seat lock was not found or has expired",
		})

	case errors.Is(
		err,
		redislock.ErrSeatLockNotOwned,
	):
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "seat_lock_not_owned",
			"message": "You do not own this seat lock",
		})

	case errors.Is(
		err,
		services.ErrInvalidSeatData,
	),
		errors.Is(
			err,
			redislock.ErrInvalidSeatLock,
		):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_seat_request",
			"message": "Seat lock request is invalid",
		})

	case errors.Is(
		err,
		redislock.ErrSeatLockUnavailable,
	):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "seat_lock_unavailable",
			"message": "Seat lock service is temporarily unavailable",
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to process seat lock",
		})
	}
}
