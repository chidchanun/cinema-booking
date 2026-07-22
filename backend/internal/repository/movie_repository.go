package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrMovieNotFound  = errors.New("movie not found")
	ErrInvalidMovieID = errors.New("invalid movie id")
)

const moviesCollection = "movies"

type MovieRepository struct {
	collection *mongo.Collection
}

func NewMovieRepository(
	db *mongo.Database,
) *MovieRepository {
	if db == nil {
		panic("movie repository: MongoDB database is nil")
	}

	return &MovieRepository{
		collection: db.Collection(moviesCollection),
	}
}

func (r *MovieRepository) Create(
	ctx context.Context,
	movie *models.Movie,
) error {
	if movie == nil {
		return errors.New("movie is nil")
	}

	now := time.Now().UTC()

	if movie.ID.IsZero() {
		movie.ID = primitive.NewObjectID()
	}

	movie.CreatedAt = now
	movie.UpdatedAt = now
	movie.DeletedAt = nil

	_, err := r.collection.InsertOne(ctx, movie)

	if err != nil {
		return fmt.Errorf("create movie: %w", err)
	}

	return nil
}

func (r *MovieRepository) FindByID(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Movie, error) {

	if id.IsZero() {
		return nil, ErrInvalidMovieID
	}

	var movie models.Movie

	err := r.collection.FindOne(
		ctx,
		bson.M{
			"_id":        id,
			"deleted_at": nil,
		},
	).Decode(&movie)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrMovieNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find movie by id: %w", err)
	}

	return &movie, nil
}

func (r *MovieRepository) FindAll(
	ctx context.Context,
	activeOnly bool,
	search string,
	skip int64,
	limit int64,
) ([]models.Movie, int64, error) {
	filter := bson.M{
		"deleted_at": nil,
	}

	if activeOnly {
		filter["is_active"] = true
	}

	search = strings.TrimSpace(search)

	if search != "" {
		filter["title"] = primitive.Regex{
			Pattern: regexp.QuoteMeta(search),
			Options: "i",
		}
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"count movies: %w",
			err,
		)
	}

	findOptions := options.Find().
		SetSort(bson.D{
			{Key: "created_at", Value: -1},
		}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := r.collection.Find(
		ctx,
		filter,
		findOptions,
	)

	if err != nil {
		return nil, 0, fmt.Errorf(
			"find movies: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	movies := make([]models.Movie, 0)

	if err := cursor.All(ctx, &movies); err != nil {
		return nil, 0, fmt.Errorf(
			"decode movies: %w",
			err,
		)
	}

	return movies, total, nil
}

func (r *MovieRepository) Update(
	ctx context.Context,
	movie *models.Movie,
) error {
	if movie == nil || movie.ID.IsZero() {
		return ErrInvalidMovieID
	}

	movie.UpdatedAt = time.Now().UTC()

	update := bson.M{
		"$set": bson.M{
			"title":            movie.Title,
			"description":      movie.Description,
			"duration_minutes": movie.DurationMinutes,
			"poster_url":       movie.PosterURL,
			"is_active":        movie.IsActive,
			"updated_at":       movie.UpdatedAt,
		},
	}

	result := r.collection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id":        movie.ID,
			"deleted_at": nil,
		},
		update,
		options.FindOneAndUpdate().
			SetReturnDocument(options.After),
	)

	if errors.Is(result.Err(), mongo.ErrNoDocuments) {
		return ErrMovieNotFound
	}

	if result.Err() != nil {
		return fmt.Errorf(
			"update movie: %w",
			result.Err(),
		)
	}

	if err := result.Decode(movie); err != nil {
		return fmt.Errorf(
			"decode updated movie: %w",
			err,
		)
	}

	return nil
}

func (r *MovieRepository) SoftDelete(
	ctx context.Context,
	id primitive.ObjectID,
) error {
	if id.IsZero() {
		return ErrInvalidMovieID
	}

	now := time.Now().UTC()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"_id":        id,
			"deleted_at": nil,
		},
		bson.M{
			"$set": bson.M{
				"is_active":  false,
				"deleted_at": now,
				"updated_at": now,
			},
		},
	)

	if err != nil {
		return fmt.Errorf(
			"soft delete movie: %w",
			err,
		)
	}

	if result.MatchedCount == 0 {
		return ErrMovieNotFound
	}

	return nil
}
