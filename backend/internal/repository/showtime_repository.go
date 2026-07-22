package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrShowtimeNotFound  = errors.New("showtime not found")
	ErrInvalidShowtimeID = errors.New("invalid showtime id")
)

const showtimeCollecion = "showtimes"

type ShowtimeRepository struct {
	collection *mongo.Collection
}

func NewShowtimeRepository(
	db *mongo.Database,
) *ShowtimeRepository {
	if db == nil {
		panic("showtime repository: MongoDB database is nil")
	}

	return &ShowtimeRepository{
		collection: db.Collection(showtimeCollecion),
	}
}

func (r *ShowtimeRepository) Create(
	ctx context.Context,
	showtime *models.Showtime,
) error {
	if showtime == nil {
		return errors.New("showtime is nil")
	}

	now := time.Now().UTC()

	if showtime.ID.IsZero() {
		showtime.ID = primitive.NewObjectID()
	}

	showtime.CreatedAt = now
	showtime.UpdatedAt = now
	showtime.DeletedAt = nil

	_, err := r.collection.InsertOne(
		ctx,
		showtime,
	)
	if err != nil {
		return fmt.Errorf(
			"create showtime: %w",
			err,
		)
	}

	return nil
}

func (r *ShowtimeRepository) FindByID(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Showtime, error) {
	if id.IsZero() {
		return nil, ErrInvalidShowtimeID
	}

	var showtime models.Showtime
	err := r.collection.FindOne(
		ctx,
		bson.M{
			"_id":        id,
			"deleted_at": nil,
		},
	).Decode(&showtime)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrShowtimeNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"find showtime by id: %w",
			err,
		)
	}

	return &showtime, nil
}

func (r *ShowtimeRepository) FindByMovieID(
	ctx context.Context,
	movieID primitive.ObjectID,
	from time.Time,
	to time.Time,
) ([]models.Showtime, error) {
	if movieID.IsZero() {
		return nil, ErrInvalidMovieID
	}

	filter := bson.M{
		"movie_id":   movieID,
		"status":     models.ShowtimeStatusActive,
		"deleted_at": nil,
		"start_time": bson.M{
			"$gte": from.UTC(),
			"$lt":  to.UTC(),
		},
	}

	findOptions := options.Find().
		SetSort(bson.D{
			{Key: "start_time", Value: 1},
		})

	cursor, err := r.collection.Find(
		ctx,
		filter,
		findOptions,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"find movie showtimes: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	showtimes := make([]models.Showtime, 0)

	if err := cursor.All(ctx, &showtimes); err != nil {
		return nil, fmt.Errorf(
			"decode movie showtimes: %w",
			err,
		)
	}

	return showtimes, nil
}

func (r *ShowtimeRepository) HasHallConflict(
	ctx context.Context,
	hallName string,
	startTime time.Time,
	endTime time.Time,
) (bool, error) {
	hallName = strings.TrimSpace(hallName)

	filter := bson.M{
		"hall_name":  hallName,
		"status":     models.ShowtimeStatusActive,
		"deleted_at": nil,

		// รอบเดิมเริ่มก่อนรอบใหม่จบ
		"start_time": bson.M{
			"$lt": endTime.UTC(),
		},

		// รอบเดิมจบหลังรอบใหม่เริ่ม
		"end_time": bson.M{
			"$gt": startTime.UTC(),
		},
	}

	err := r.collection.FindOne(
		ctx,
		filter,
		options.FindOne().SetProjection(
			bson.M{
				"_id": 1,
			},
		),
	).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf(
			"check hall schedule conflict: %w",
			err,
		)
	}

	return true, nil
}

func (r *ShowtimeRepository) ListHalls(ctx context.Context) ([]models.HallSummary, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"deleted_at": nil},
		options.Find().
			SetProjection(bson.M{
				"hall_name":     1,
				"seat_rows":     1,
				"seats_per_row": 1,
				"total_seats":   1,
			}).
			SetSort(bson.D{
				{Key: "hall_name", Value: 1},
				{Key: "updated_at", Value: -1},
			}),
	)
	if err != nil {
		return nil, fmt.Errorf("list halls: %w", err)
	}
	defer cursor.Close(ctx)

	halls := make([]models.HallSummary, 0)
	seen := make(map[string]struct{})
	for cursor.Next(ctx) {
		var hall models.HallSummary
		if err := cursor.Decode(&hall); err != nil {
			return nil, fmt.Errorf("decode hall: %w", err)
		}
		hall.Name = strings.TrimSpace(hall.Name)
		if hall.Name == "" {
			continue
		}
		key := strings.ToLower(hall.Name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		halls = append(halls, hall)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterate halls: %w", err)
	}
	return halls, nil
}

func (r *ShowtimeRepository) Cancel(
	ctx context.Context,
	id primitive.ObjectID,
) error {
	if id.IsZero() {
		return ErrInvalidShowtimeID
	}

	now := time.Now().UTC()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"_id":        id,
			"status":     models.ShowtimeStatusActive,
			"deleted_at": nil,
		},
		bson.M{
			"$set": bson.M{
				"status":     models.ShowtimeStatusCancelled,
				"updated_at": now,
			},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"cancel showtime: %w",
			err,
		)
	}

	if result.MatchedCount == 0 {
		return ErrShowtimeNotFound
	}

	return nil
}
