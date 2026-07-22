package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentNotificationUserReader interface {
	FindByID(context.Context, primitive.ObjectID) (*models.User, error)
}
type PaymentNotificationShowtimeReader interface {
	FindByID(context.Context, primitive.ObjectID) (*models.Showtime, error)
}
type EmailSender interface {
	Send(context.Context, string, string, string) error
}

type PaymentNotificationHandler struct {
	users     PaymentNotificationUserReader
	showtimes PaymentNotificationShowtimeReader
	email     EmailSender
}

func NewPaymentNotificationHandler(users PaymentNotificationUserReader, showtimes PaymentNotificationShowtimeReader, email EmailSender) *PaymentNotificationHandler {
	return &PaymentNotificationHandler{users: users, showtimes: showtimes, email: email}
}

func (h *PaymentNotificationHandler) SendPending(c *gin.Context) {
	userID, ok := authmiddleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Authentication is required"})
		return
	}
	var request struct {
		ShowtimeID string    `json:"showtime_id" binding:"required"`
		SeatCodes  []string  `json:"seat_codes" binding:"required,min=1,max=10"`
		ExpiresAt  time.Time `json:"expires_at" binding:"required"`
		Status     string    `json:"status"`
	}
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Payment notification data is invalid"})
		return
	}
	showtimeID, err := primitive.ObjectIDFromHex(request.ShowtimeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_showtime_id", "message": "Showtime ID is invalid"})
		return
	}
	user, err := h.users.FindByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user_unavailable", "message": "Unable to retrieve user"})
		return
	}
	showtime, err := h.showtimes.FindByID(c.Request.Context(), showtimeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "showtime_not_found", "message": "Showtime not found"})
		return
	}
	bangkok, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		bangkok = time.FixedZone("ICT", 7*60*60)
	}
	formatBangkokTime := func(value time.Time) string {
		return value.In(bangkok).Format("Mon, 02 Jan 2006 15:04:05 MST")
	}
	subject := "Cinema Booking - payment pending"
	body := fmt.Sprintf(
		"Your seats are waiting for simulated payment.\n\nHall: %s\nShowtime: %s\nSeats: %s\nTotal: %d %s\nPay before: %s\n\nUnpaid seats are released automatically after 5 minutes.",
		showtime.HallName, formatBangkokTime(showtime.StartTime), strings.Join(request.SeatCodes, ", "),
		showtime.Price*int64(len(request.SeatCodes)), showtime.Currency, formatBangkokTime(request.ExpiresAt),
	)
	if strings.EqualFold(request.Status, "PAID") {
		subject = "Cinema Booking - payment successful"
		body = fmt.Sprintf(
			"Your simulated payment was successful and your seats are BOOKED.\n\nHall: %s\nShowtime: %s\nSeats: %s\nTotal: %d %s",
			showtime.HallName, formatBangkokTime(showtime.StartTime), strings.Join(request.SeatCodes, ", "),
			showtime.Price*int64(len(request.SeatCodes)), showtime.Currency,
		)
	}
	if err := h.email.Send(c.Request.Context(), user.Email, subject, body); err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "email_unavailable", "message": "Unable to send payment reminder email"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Payment reminder email sent"})
}
