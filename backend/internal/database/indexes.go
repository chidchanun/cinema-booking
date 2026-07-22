package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateIndexes(ctx context.Context, db *mongo.Database) error {
	if db == nil {
		return fmt.Errorf("MongoDB database is nil")
	}

	if err := createUserIndexes(ctx, db); err != nil {
		return err
	}

	if err := createMovieIndexes(ctx, db); err != nil {
		return err
	}

	if err := createShowtimeIndexes(ctx, db); err != nil {
		return err
	}

	if err := createBookingIndexes(ctx, db); err != nil {
		return err
	}

	if err := createAuditLogIndexes(ctx, db); err != nil {
		return err
	}

	return nil
}

func createUserIndexes(ctx context.Context, db *mongo.Database) error {
	collection := db.Collection("users")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "email", Value: 1},
			},
			Options: options.Index().
				SetName("uq_users_email").
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "google_id", Value: 1},
			},
			Options: options.Index().
				SetName("uq_users_google_id").
				SetUnique(true).
				SetSparse(true),
		},
		{
			Keys: bson.D{
				{Key: "role", Value: 1},
			},
			Options: options.Index().
				SetName("idx_users_role"),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create users indexes: %w", err)
	}

	return nil
}

func createMovieIndexes(ctx context.Context, db *mongo.Database) error {
	collection := db.Collection("movies")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "title", Value: 1},
			},
			Options: options.Index().
				SetName("idx_movies_title"),
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName("idx_movies_created_at"),
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create movies indexes: %w", err)
	}

	return nil
}

func createShowtimeIndexes(
	ctx context.Context,
	db *mongo.Database,
) error {
	collection := db.Collection("showtimes")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "movie_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "start_time", Value: 1},
			},
			Options: options.Index().
				SetName(
					"idx_showtimes_movie_status_start",
				),
		},
		{
			Keys: bson.D{
				{Key: "hall_name", Value: 1},
				{Key: "status", Value: 1},
				{Key: "start_time", Value: 1},
				{Key: "end_time", Value: 1},
			},
			Options: options.Index().
				SetName(
					"idx_showtimes_hall_status_time",
				),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "start_time", Value: 1},
			},
			Options: options.Index().
				SetName(
					"idx_showtimes_status_start",
				),
		},
	}

	if _, err := collection.Indexes().
		CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf(
			"create showtimes indexes: %w",
			err,
		)
	}

	return nil
}

func createBookingIndexes(
	ctx context.Context,
	db *mongo.Database,
) error {
	collection := db.Collection("bookings")

	// Migrate bookings created before BOOKED became the only paid status.
	if _, err := collection.UpdateMany(
		ctx,
		bson.M{"status": "CONFIRMED"},
		bson.M{"$set": bson.M{"status": "BOOKED", "updated_at": time.Now().UTC()}},
	); err != nil {
		return fmt.Errorf("migrate confirmed bookings to booked: %w", err)
	}

	if _, err := collection.Indexes().DropOne(
		ctx,
		"uq_bookings_showtime_seat_confirmed",
	); err != nil {
		var commandError mongo.CommandError
		if !errors.As(err, &commandError) || commandError.Code != 27 {
			return fmt.Errorf("drop legacy confirmed booking index: %w", err)
		}
	}

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "booking_code", Value: 1},
			},
			Options: options.Index().
				SetName("uq_bookings_booking_code").
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_bookings_user_created_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "showtime_id", Value: 1},
				{Key: "seat_code", Value: 1},
			},
			Options: options.Index().
				SetName("uq_bookings_showtime_seat_booked").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"status": "BOOKED"}),
		},
		{
			Keys: bson.D{
				{Key: "movie_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_bookings_movie_created_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "showtime_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_bookings_showtime_created_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_bookings_status_created_id",
				),
		},
	}

	if _, err := collection.Indexes().
		CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf(
			"create bookings indexes: %w",
			err,
		)
	}

	return nil
}

func createAuditLogIndexes(
	ctx context.Context,
	db *mongo.Database,
) error {
	collection := db.Collection("audit_logs")

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "event_id", Value: 1},
			},
			Options: options.Index().
				SetName("uq_audit_logs_event_id").
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "actor_user_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_actor_created_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "entity_type", Value: 1},
				{Key: "entity_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_entity_created_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "event_type", Value: 1},
				{Key: "occurred_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_event_occurred_at",
				),
		},
		{
			Keys: bson.D{
				{Key: "actor_user_id", Value: 1},
				{Key: "occurred_at", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_actor_occurred",
				),
		},
		{
			Keys: bson.D{
				{Key: "entity_type", Value: 1},
				{Key: "entity_id", Value: 1},
				{Key: "occurred_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_entity_occurred",
				),
		},
		{
			Keys: bson.D{
				{Key: "action", Value: 1},
				{Key: "occurred_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_action_occurred",
				),
		},
		{
			Keys: bson.D{
				{Key: "severity", Value: 1},
				{Key: "occurred_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_severity_occurred",
				),
		},
		{
			Keys: bson.D{
				{Key: "actor_type", Value: 1},
				{Key: "occurred_at", Value: -1},
			},
			Options: options.Index().
				SetName(
					"idx_audit_logs_actor_type_occurred",
				),
		},
	}

	if _, err := collection.Indexes().
		CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf(
			"create audit log indexes: %w",
			err,
		)
	}

	return nil
}
