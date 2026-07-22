package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MovieService interface {
	CreateMovie(
		ctx context.Context,
		input services.CreateMovieInput,
	) (*models.Movie, error)

	GetPublicMovie(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Movie, error)

	GetAdminMovie(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Movie, error)

	ListMovies(
		ctx context.Context,
		page int,
		limit int,
		search string,
		activeOnly bool,
	) (*services.MovieListResult, error)

	UpdateMovie(
		ctx context.Context,
		id primitive.ObjectID,
		input services.UpdateMovieInput,
	) (*models.Movie, error)

	DeleteMovie(
		ctx context.Context,
		id primitive.ObjectID,
	) error
}

type MovieHandler struct {
	movieService MovieService
}

type CreateMovieRequest struct {
	Title           string `json:"title" binding:"required"`
	Description     string `json:"description"`
	DurationMinutes int    `json:"duration_minutes" binding:"required"`
	PosterURL       string `json:"poster_url"`
	IsActive        *bool  `json:"is_active"`
}

type UpdateMovieRequest struct {
	Title           *string `json:"title"`
	Description     *string `json:"description"`
	DurationMinutes *int    `json:"duration_minutes"`
	PosterURL       *string `json:"poster_url"`
	IsActive        *bool   `json:"is_active"`
}

type MovieListResponse struct {
	Data []models.MovieResponse `json:"data"`

	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewMovieHandler(
	movieService MovieService,
) *MovieHandler {
	if movieService == nil {
		panic("movie handler: movie service is nil")
	}

	return &MovieHandler{
		movieService: movieService,
	}
}

func (h *MovieHandler) ListPublic(c *gin.Context) {
	h.list(c, true)
}

func (h *MovieHandler) ListAdmin(c *gin.Context) {
	h.list(c, false)
}

func (h *MovieHandler) list(
	c *gin.Context,
	activeOnly bool,
) {
	page := parsePositiveInt(
		c.Query("page"),
		1,
	)

	limit := parsePositiveInt(
		c.Query("limit"),
		10,
	)

	search := strings.TrimSpace(
		c.Query("search"),
	)

	result, err := h.movieService.ListMovies(
		c.Request.Context(),
		page,
		limit,
		search,
		activeOnly,
	)
	if err != nil {
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to retrieve movies",
		})
		return
	}

	c.JSON(http.StatusOK, MovieListResponse{
		Data: models.NewMovieResponses(result.Movies),

		Page:       result.Page,
		Limit:      result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

func (h *MovieHandler) GetPublic(c *gin.Context) {

	id, ok := parseMovieID(c)

	if !ok {
		return
	}

	movie, err := h.movieService.GetPublicMovie(
		c.Request.Context(),
		id,
	)
	if err != nil {
		h.handleMovieError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewMovieResponse(movie),
	)
}

func (h *MovieHandler) GetAdmin(c *gin.Context) {
	id, ok := parseMovieID(c)
	if !ok {
		return
	}

	movie, err := h.movieService.GetAdminMovie(
		c.Request.Context(),
		id,
	)
	if err != nil {
		h.handleMovieError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewMovieResponse(movie),
	)
}

func (h *MovieHandler) Create(c *gin.Context) {
	var request CreateMovieRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid movie data",
		})
		return
	}

	movie, err := h.movieService.CreateMovie(
		c.Request.Context(),
		services.CreateMovieInput{
			Title:           request.Title,
			Description:     request.Description,
			DurationMinutes: request.DurationMinutes,
			PosterURL:       request.PosterURL,
			IsActive:        request.IsActive,
		},
	)

	if err != nil {
		h.handleMovieError(c, err)
		return
	}

	c.JSON(
		http.StatusCreated,
		models.NewMovieResponse(movie),
	)
}

func (h *MovieHandler) Update(c *gin.Context) {

	id, ok := parseMovieID(c)
	if !ok {
		return
	}

	var request UpdateMovieRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid movie data",
		})
		return
	}

	if request.Title == nil &&
		request.Description == nil &&
		request.DurationMinutes == nil &&
		request.PosterURL == nil &&
		request.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "At least one field is required",
		})
		return
	}

	movie, err := h.movieService.UpdateMovie(
		c.Request.Context(),
		id,
		services.UpdateMovieInput{
			Title:           request.Title,
			Description:     request.Description,
			DurationMinutes: request.DurationMinutes,
			PosterURL:       request.PosterURL,
			IsActive:        request.IsActive,
		},
	)
	if err != nil {
		h.handleMovieError(c, err)
		return
	}

	c.JSON(
		http.StatusOK,
		models.NewMovieResponse(movie),
	)
}

func (h *MovieHandler) Delete(c *gin.Context) {
	id, ok := parseMovieID(c)
	if !ok {
		return
	}

	if err := h.movieService.DeleteMovie(
		c.Request.Context(),
		id,
	); err != nil {
		h.handleMovieError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Movie deleted successfully",
	})
}

func (h *MovieHandler) handleMovieError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, repository.ErrMovieNotFound),
		errors.Is(err, services.ErrMovieUnavailable):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "movie_not_found",
			"message": "Movie not found",
		})

	case errors.Is(err, repository.ErrInvalidMovieID),
		errors.Is(err, services.ErrInvalidMovieData):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_movie_data",
			"message": err.Error(),
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to process movie",
		})
	}
}

func parseMovieID(
	c *gin.Context,
) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(
		c.Param("movieID"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_movie_id",
			"message": "Movie ID is invalid",
		})
		return primitive.NilObjectID, false
	}

	return id, true
}

func parsePositiveInt(
	value string,
	fallback int,
) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}

	return parsed
}
