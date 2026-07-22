package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidMovieData = errors.New("invalid movie data")
var ErrMovieUnavailable = errors.New("movie unavailable")

type MovieRepository interface {
	Create(
		ctx context.Context,
		movie *models.Movie,
	) error

	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Movie, error)

	FindAll(
		ctx context.Context,
		activeOnly bool,
		search string,
		skip int64,
		limit int64,
	) ([]models.Movie, int64, error)

	Update(
		ctx context.Context,
		movie *models.Movie,
	) error

	SoftDelete(
		ctx context.Context,
		id primitive.ObjectID,
	) error
}

type MovieService struct {
	movieRepository MovieRepository
}

type CreateMovieInput struct {
	Title           string
	Description     string
	DurationMinutes int
	PosterURL       string
	IsActive        *bool
}

type UpdateMovieInput struct {
	Title           *string
	Description     *string
	DurationMinutes *int
	PosterURL       *string
	IsActive        *bool
}

type MovieListResult struct {
	Movies []models.Movie

	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

func NewMovieService(
	movieRepository MovieRepository,
) *MovieService {
	if movieRepository == nil {
		panic("movie service: movie repository is nil")
	}

	return &MovieService{
		movieRepository: movieRepository,
	}
}

func (s *MovieService) CreateMovie(
	ctx context.Context,
	input CreateMovieInput,
) (*models.Movie, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.PosterURL = strings.TrimSpace(input.PosterURL)

	if err := validateMovieData(
		input.Title,
		input.Description,
		input.DurationMinutes,
		input.PosterURL,
	); err != nil {
		return nil, err
	}

	isActive := true

	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	movie := &models.Movie{
		Title:           input.Title,
		Description:     input.Description,
		DurationMinutes: input.DurationMinutes,
		PosterURL:       input.PosterURL,
		IsActive:        isActive,
	}

	if err := s.movieRepository.Create(
		ctx,
		movie,
	); err != nil {
		return nil, fmt.Errorf(
			"create movie: %w",
			err,
		)
	}

	return movie, nil
}

func (s *MovieService) GetPublicMovie(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Movie, error) {
	movie, err := s.movieRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return movie, nil

}

func (s *MovieService) GetAdminMovie(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Movie, error) {
	return s.movieRepository.FindByID(ctx, id)
}

func (s *MovieService) ListMovies(
	ctx context.Context,
	page int,
	limit int,
	search string,
	activeOnly bool,
) (*MovieListResult, error) {
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

	movies, total, err := s.movieRepository.FindAll(
		ctx,
		activeOnly,
		search,
		skip,
		int64(limit),
	)

	if err != nil {
		return nil, fmt.Errorf(
			"list movies: %w",
			err,
		)
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(limit) - 1) / int64(limit),
		)
	}

	return &MovieListResult{
		Movies:     movies,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *MovieService) UpdateMovie(
	ctx context.Context,
	id primitive.ObjectID,
	input UpdateMovieInput,
) (*models.Movie, error) {
	movie, err := s.movieRepository.FindByID(
		ctx,
		id,
	)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		movie.Title = strings.TrimSpace(*input.Title)
	}

	if input.Description != nil {
		movie.Description = strings.TrimSpace(
			*input.Description,
		)
	}

	if input.DurationMinutes != nil {
		movie.DurationMinutes = *input.DurationMinutes
	}

	if input.PosterURL != nil {
		movie.PosterURL = strings.TrimSpace(
			*input.PosterURL,
		)
	}

	if input.IsActive != nil {
		movie.IsActive = *input.IsActive
	}

	if err := validateMovieData(
		movie.Title,
		movie.Description,
		movie.DurationMinutes,
		movie.PosterURL,
	); err != nil {
		return nil, err
	}

	if err := s.movieRepository.Update(
		ctx,
		movie,
	); err != nil {
		return nil, fmt.Errorf(
			"update movie: %w",
			err,
		)
	}

	return movie, nil
}

func (s *MovieService) DeleteMovie(
	ctx context.Context,
	id primitive.ObjectID,
) error {
	if id.IsZero() {
		return ErrInvalidMovieData
	}

	if err := s.movieRepository.SoftDelete(
		ctx,
		id,
	); err != nil {
		return fmt.Errorf(
			"delete movie: %w",
			err,
		)
	}

	return nil
}

func validateMovieData(
	title string,
	description string,
	durationMinutes int,
	posterURL string,
) error {
	if title == "" {
		return fmt.Errorf(
			"%w: title is required",
			ErrInvalidMovieData,
		)
	}

	if len(title) > 200 {
		return fmt.Errorf(
			"%w: title must not exceed 200 characters",
			ErrInvalidMovieData,
		)
	}

	if len(description) > 5000 {
		return fmt.Errorf(
			"%w: description must not exceed 5000 characters",
			ErrInvalidMovieData,
		)
	}

	if durationMinutes < 1 || durationMinutes > 600 {
		return fmt.Errorf(
			"%w: duration_minutes must be between 1 and 600",
			ErrInvalidMovieData,
		)
	}

	if posterURL != "" {
		parsedURL, err := url.ParseRequestURI(posterURL)
		if err != nil {
			return fmt.Errorf(
				"%w: poster_url is invalid",
				ErrInvalidMovieData,
			)
		}

		if parsedURL.Scheme != "http" &&
			parsedURL.Scheme != "https" {
			return fmt.Errorf(
				"%w: poster_url must use http or https",
				ErrInvalidMovieData,
			)
		}
	}

	return nil
}
