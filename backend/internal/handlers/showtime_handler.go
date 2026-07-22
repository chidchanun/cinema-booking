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

type ShowtimeService interface {
	CreateShowtime(
		ctx context.Context,
		input services.CreateShowtimeInput,
	) (*models.Showtime, error)

	GetPublicShowtime(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)

	GetAdminShowtime(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)

	ListMovieShowtimes(
		ctx context.Context,
		movieID primitive.ObjectID,
		from time.Time,
		to time.Time,
	) ([]models.Showtime, error)

	CancelShowtime(
		ctx context.Context,
		id primitive.ObjectID,
	) error
	ListHallNames(ctx context.Context) ([]string, error)
	CheckHallAvailability(
		ctx context.Context,
		movieID primitive.ObjectID,
		hallName string,
		startTime time.Time,
	) (*services.ShowtimeAvailability, error)
}

func (h *ShowtimeHandler) ListHalls(c *gin.Context) {
	halls, err := h.showtimeService.ListHallNames(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_server_error", "message": "Unable to retrieve halls"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": halls})
}

func (h *ShowtimeHandler) CheckHallAvailability(c *gin.Context) {
	movieID, err := primitive.ObjectIDFromHex(strings.TrimSpace(c.Query("movie_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_movie_id", "message": "Movie ID is invalid"})
		return
	}
	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(c.Query("start_time")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_start_time", "message": "Start time must use RFC3339 format"})
		return
	}

	availability, err := h.showtimeService.CheckHallAvailability(
		c.Request.Context(), movieID, c.Query("hall_name"), startTime.UTC(),
	)
	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"available": availability.Available,
		"end_time":  availability.EndTime,
	})
}

type ShowtimeHandler struct {
	showtimeService ShowtimeService
}

type CreateShowtimeRequest struct {
	MovieID string `json:"movie_id" binding:"required"`

	HallName  string    `json:"hall_name" binding:"required"`
	StartTime time.Time `json:"start_time" binding:"required"`

	Price    int64  `json:"price"`
	Currency string `json:"currency"`

	SeatRows    int `json:"seat_rows" binding:"required"`
	SeatsPerRow int `json:"seats_per_row" binding:"required"`
}

func NewShowtimeHandler(
	showtimeService ShowtimeService,
) *ShowtimeHandler {
	if showtimeService == nil {
		panic("showtime handler: showtime service is nil")
	}

	return &ShowtimeHandler{
		showtimeService: showtimeService,
	}
}

func (h *ShowtimeHandler) Create(c *gin.Context) {
	var request CreateShowtimeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid showtime data",
		})
		return
	}

	movieID, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(request.MovieID),
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_movie_id",
			"message": "Movie ID is invalid",
		})
		return
	}

	currency := strings.TrimSpace(request.Currency)
	if currency == "" {
		currency = "THB"
	}

	showtime, err := h.showtimeService.CreateShowtime(
		c.Request.Context(),
		services.CreateShowtimeInput{
			MovieID: movieID,

			HallName:  request.HallName,
			StartTime: request.StartTime,

			Price:    request.Price,
			Currency: currency,

			SeatRows:    request.SeatRows,
			SeatsPerRow: request.SeatsPerRow,
		},
	)

	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	c.JSON(
		http.StatusCreated,
		models.NewShowtimeResponse(showtime),
	)
}

func (h *ShowtimeHandler) ListByMovie(c *gin.Context) {
	movieID, err := primitive.ObjectIDFromHex(
		c.Param("movieID"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_movie_id",
			"message": "Movie ID is invalid",
		})
		return
	}

	from := time.Now().UTC()
	to := from.AddDate(0, 1, 0)

	if rawFrom := strings.TrimSpace(c.Query("from")); rawFrom != "" {
		parsedFrom, err := time.Parse(
			time.RFC3339,
			rawFrom,
		)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_from_time",
				"message": "from must use RFC3339 format",
			})
			return
		}

		from = parsedFrom.UTC()
	}

	if rawTo := strings.TrimSpace(c.Query("to")); rawTo != "" {
		parsedTo, err := time.Parse(
			time.RFC3339,
			rawTo,
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_to_time",
				"message": "to must use RFC3339 format",
			})
			return
		}

		to = parsedTo.UTC()
	}

	showtimes, err := h.showtimeService.ListMovieShowtimes(
		c.Request.Context(),
		movieID,
		from,
		to,
	)
	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": models.NewShowtimeResponses(showtimes),
	})
}

func (h *ShowtimeHandler) GetPublic(c *gin.Context) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	showtime, err := h.showtimeService.GetPublicShowtime(
		c.Request.Context(),
		showtimeID,
	)
	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewShowtimeResponse(showtime),
	)
}

func (h *ShowtimeHandler) GetAdmin(c *gin.Context) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	showtime, err := h.showtimeService.GetAdminShowtime(
		c.Request.Context(),
		showtimeID,
	)
	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewShowtimeResponse(showtime),
	)
}

func (h *ShowtimeHandler) SeatMap(c *gin.Context) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	showtime, err := h.showtimeService.GetPublicShowtime(
		c.Request.Context(),
		showtimeID,
	)
	if err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	/*
		ตอนนี้จะคืน AVAILABLE และ BOOKED จาก MongoDB

		Phase Redis Lock จะนำข้อมูล Lock จาก Redis
		มาเปลี่ยน AVAILABLE เป็น LOCKED ก่อนส่ง Response
	*/
	c.JSON(
		http.StatusOK,
		models.NewSeatMapResponse(showtime),
	)
}

func (h *ShowtimeHandler) Cancel(c *gin.Context) {
	showtimeID, ok := parseShowtimeID(c)
	if !ok {
		return
	}

	if err := h.showtimeService.CancelShowtime(
		c.Request.Context(),
		showtimeID,
	); err != nil {
		h.handleShowtimeError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"message": "Showtime cancelled successfully",
		},
	)
}

func (h *ShowtimeHandler) handleShowtimeError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, repository.ErrShowtimeNotFound),
		errors.Is(err, services.ErrShowtimeUnavailable):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "showtime_not_found",
			"message": "Showtime not found",
		})

	case errors.Is(err, repository.ErrMovieNotFound),
		errors.Is(err, services.ErrMovieUnavailable):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "movie_not_found",
			"message": "Movie not found",
		})

	case errors.Is(err, services.ErrShowtimeConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "showtime_conflict",
			"message": "The hall already has another showtime during this period",
		})

	case errors.Is(err, repository.ErrInvalidShowtimeID),
		errors.Is(err, repository.ErrInvalidMovieID),
		errors.Is(err, services.ErrInvalidShowtimeData):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_showtime_data",
			"message": err.Error(),
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to process showtime",
		})
	}
}

func parseShowtimeID(
	c *gin.Context,
) (primitive.ObjectID, bool) {
	showtimeID, err := primitive.ObjectIDFromHex(
		c.Param("showtimeID"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_showtime_id",
			"message": "Showtime ID is invalid",
		})
		return primitive.NilObjectID, false
	}

	return showtimeID, true
}
