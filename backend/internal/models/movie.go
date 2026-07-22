package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Movie struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	Title           string `bson:"title" json:"title"`
	Description     string `bson:"description" json:"description"`
	DurationMinutes int    `bson:"duration_minutes" json:"duration_minutes"`
	PosterURL       string `bson:"poster_url,omitempty" json:"poster_url,omitempty"`

	IsActive bool `bson:"is_active" json:"is_active"`

	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `bson:"deleted_at,omitempty" json:"-"`
}

type MovieResponse struct {
	ID primitive.ObjectID `json:"id"`

	Title           string `json:"title"`
	Description     string `json:"description"`
	DurationMinutes int    `json:"duration_minutes"`
	PosterURL       string `json:"poster_url,omitempty"`
	IsActive        bool   `json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewMovieResponse(movie *Movie) MovieResponse {
	if movie == nil {
		return MovieResponse{}
	}

	return MovieResponse{
		ID:              movie.ID,
		Title:           movie.Title,
		Description:     movie.Description,
		DurationMinutes: movie.DurationMinutes,
		PosterURL:       movie.PosterURL,
		IsActive:        movie.IsActive,
		CreatedAt:       movie.CreatedAt,
		UpdatedAt:       movie.UpdatedAt,
	}
}

func NewMovieResponses(movies []Movie) []MovieResponse {
	responses := make([]MovieResponse, 0, len(movies))

	for index := range movies {
		responses = append(
			responses,
			NewMovieResponse(&movies[index]),
		)
	}

	return responses
}
