package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"cinema-booking/internal/tmdb"

	"github.com/gin-gonic/gin"
)

type TMDBMovieSearcher interface {
	SearchMovies(ctx context.Context, query string) ([]tmdb.Movie, error)
	GetMovie(ctx context.Context, movieID int64) (*tmdb.Movie, error)
}

func (h *TMDBHandler) GetMovie(c *gin.Context) {
	movieID, err := strconv.ParseInt(c.Param("tmdbMovieID"), 10, 64)
	if err != nil || movieID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_tmdb_movie_id", "message": "TMDB movie ID is invalid"})
		return
	}

	movie, err := h.searcher.GetMovie(c.Request.Context(), movieID)
	if err != nil {
		if errors.Is(err, tmdb.ErrNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tmdb_not_configured", "message": "TMDB credentials are required"})
			return
		}
		_ = c.Error(err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "tmdb_unavailable", "message": "Unable to retrieve TMDB movie details"})
		return
	}

	c.JSON(http.StatusOK, movie)
}

type TMDBHandler struct {
	searcher TMDBMovieSearcher
}

func NewTMDBHandler(searcher TMDBMovieSearcher) *TMDBHandler {
	if searcher == nil {
		panic("TMDB handler: searcher is nil")
	}
	return &TMDBHandler{searcher: searcher}
}

func (h *TMDBHandler) SearchMovies(c *gin.Context) {
	query := strings.TrimSpace(c.Query("query"))
	movies, err := h.searcher.SearchMovies(c.Request.Context(), query)
	if err != nil {
		switch {
		case errors.Is(err, tmdb.ErrQueryRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "query_required", "message": "Movie search query is required"})
		case errors.Is(err, tmdb.ErrNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tmdb_not_configured", "message": "TMDB_ACCESS_TOKEN or TMDB_API_KEY is required"})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "tmdb_unavailable", "message": "Unable to search TMDB"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": movies})
}
