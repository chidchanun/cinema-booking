package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var (
	ErrBookingNotFound = errors.New("booking not found")
	ErrInvalidBooking  = errors.New("invalid booking")
	ErrSeatUnavailable = errors.New("seat is no longer available")
	ErrBookingConflict = errors.New("booking conflict")
)

const bookingsCollection = "bookings"
const showtimesCollection = "showtimes"

type BookingRepository struct {
	client *mongo.Client

	bookingCollection  *mongo.Collection
	showtimeCollection *mongo.Collection
}

type AdminBookingFilter struct {
	UserID     *primitive.ObjectID
	MovieID    *primitive.ObjectID
	ShowtimeID *primitive.ObjectID

	Status *models.BookingStatus

	From *time.Time
	To   *time.Time
}

func NewBookingRepository(
	client *mongo.Client,
	db *mongo.Database,
) *BookingRepository {
	if client == nil {
		panic("booking repository: MongoDB client is nil")
	}

	if db == nil {
		panic("booking repository: MongoDB database is nil")
	}

	return &BookingRepository{
		client: client,

		bookingCollection: db.Collection(
			bookingsCollection,
		),
		showtimeCollection: db.Collection(
			showtimesCollection,
		),
	}
}

func (r *BookingRepository) Confirm(
	ctx context.Context,
	booking *models.Booking,
) error {
	return r.ConfirmMany(ctx, []*models.Booking{booking})
}

func (r *BookingRepository) ConfirmMany(
	ctx context.Context,
	bookings []*models.Booking,
) error {
	if len(bookings) == 0 {
		return ErrInvalidBooking
	}
	for _, booking := range bookings {
		if booking == nil || booking.ID.IsZero() || booking.UserID.IsZero() || booking.MovieID.IsZero() || booking.SeatCode == "" {
			return ErrInvalidBooking
		}
	}

	now := time.Now().UTC()

	for _, booking := range bookings {
		booking.CreatedAt = now
		booking.UpdatedAt = now
		booking.ConfirmedAt = now
		booking.Status = models.BookingStatusBooked
	}

	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf(
			"start booking transaction session: %w",
			err,
		)
	}
	defer session.EndSession(ctx)

	transactionOptions := options.Transaction().
		SetReadConcern(
			readconcern.Snapshot(),
		).
		SetWriteConcern(
			writeconcern.Majority(),
		).
		SetReadPreference(
			readpref.Primary(),
		)

	_, err = session.WithTransaction(
		ctx,
		func(sessionContext mongo.SessionContext) (interface{}, error) {
			for _, booking := range bookings {
				/*
					Conditional update:

					มีเพียง Request แรกที่พบ Seat เป็น AVAILABLE
					เท่านั้นที่จะเปลี่ยนเป็น BOOKED สำเร็จ
				*/

				updateResult, err := r.showtimeCollection.UpdateOne(
					sessionContext,
					bson.M{
						"_id":        booking.ShowtimeID,
						"status":     models.ShowtimeStatusActive,
						"deleted_at": nil,
						"seats": bson.M{
							"$elemMatch": bson.M{
								"code":   booking.SeatCode,
								"status": models.SeatStatusAvailable,
							},
						},
					},
					bson.M{
						"$set": bson.M{
							"seats.$.status": models.SeatStatusBooked,
							"updated_at":     now,
						},
					},
				)
				if err != nil {
					return nil, fmt.Errorf(
						"mark seat as booked: %w",
						err,
					)
				}

				if updateResult.MatchedCount == 0 {
					return nil, ErrSeatUnavailable
				}

				_, err = r.bookingCollection.InsertOne(
					sessionContext,
					booking,
				)

				if err != nil {
					if mongo.IsDuplicateKeyError(err) {
						return nil, ErrBookingConflict
					}

					return nil, fmt.Errorf(
						"insert booking: %w",
						err,
					)
				}

			}
			return bookings, nil
		},
		transactionOptions,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrSeatUnavailable):
			return ErrSeatUnavailable

		case errors.Is(err, ErrBookingConflict):
			return ErrBookingConflict

		default:
			return fmt.Errorf(
				"confirm booking transaction: %w",
				err,
			)
		}
	}

	return nil

}

func (r *BookingRepository) FindByIDAndUserID(
	ctx context.Context,
	bookingID primitive.ObjectID,
	userID primitive.ObjectID,
) (*models.Booking, error) {
	if bookingID.IsZero() || userID.IsZero() {
		return nil, ErrInvalidBooking
	}

	var booking models.Booking

	err := r.bookingCollection.FindOne(
		ctx,
		bson.M{
			"_id":     bookingID,
			"user_id": userID,
		},
	).Decode(&booking)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrBookingNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"find user booking: %w",
			err,
		)
	}

	return &booking, nil
}

type UserBookingFilter struct {
	MovieID *primitive.ObjectID
	From    *time.Time
	To      *time.Time
}

func (r *BookingRepository) FindByUserID(
	ctx context.Context,
	userID primitive.ObjectID,
	filter UserBookingFilter,
	skip int64,
	limit int64,
) ([]models.Booking, int64, error) {
	if userID.IsZero() {
		return nil, 0, ErrInvalidUserID
	}

	query := bson.M{
		"user_id": userID,
	}
	if filter.MovieID != nil {
		query["movie_id"] = *filter.MovieID
	}
	if filter.From != nil || filter.To != nil {
		showtimeFilter := bson.M{}
		if filter.From != nil {
			showtimeFilter["$gte"] = filter.From.UTC()
		}
		if filter.To != nil {
			showtimeFilter["$lt"] = filter.To.UTC()
		}
		query["showtime_start"] = showtimeFilter
	}

	total, err := r.bookingCollection.CountDocuments(
		ctx,
		query,
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"count user bookings: %w",
			err,
		)
	}

	cursor, err := r.bookingCollection.Find(
		ctx,
		query,
		options.Find().
			SetSort(bson.D{
				{Key: "created_at", Value: -1},
			}).
			SetSkip(skip).
			SetLimit(limit),
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"find user bookings: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	bookings := make([]models.Booking, 0)

	if err := cursor.All(ctx, &bookings); err != nil {
		return nil, 0, fmt.Errorf(
			"decode user bookings: %w",
			err,
		)
	}

	return bookings, total, nil
}

func (r *BookingRepository) FindAllAdmin(
	ctx context.Context,
	filter AdminBookingFilter,
	skip int64,
	limit int64,
) ([]models.AdminBookingView, int64, error) {
	if skip < 0 || limit < 1 {
		return nil, 0, ErrInvalidBooking
	}

	mongoFilter := buildAdminBookingFilter(filter)

	total, err := r.bookingCollection.CountDocuments(
		ctx,
		mongoFilter,
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"count admin bookings: %w",
			err,
		)
	}

	pipeline := buildAdminBookingPipeline(
		mongoFilter,
		&skip,
		&limit,
	)

	cursor, err := r.bookingCollection.Aggregate(
		ctx,
		pipeline,
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"aggregate admin bookings: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	bookings := make(
		[]models.AdminBookingView,
		0,
	)

	if err := cursor.All(ctx, &bookings); err != nil {
		return nil, 0, fmt.Errorf(
			"decode admin bookings: %w",
			err,
		)
	}

	return bookings, total, nil
}

func (r *BookingRepository) FindAdminByID(
	ctx context.Context,
	bookingID primitive.ObjectID,
) (*models.AdminBookingView, error) {
	if bookingID.IsZero() {
		return nil, ErrInvalidBooking
	}

	pipeline := buildAdminBookingPipeline(
		bson.M{
			"_id": bookingID,
		},
		nil,
		nil,
	)

	cursor, err := r.bookingCollection.Aggregate(
		ctx,
		pipeline,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"aggregate admin booking by id: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		if err := cursor.Err(); err != nil {
			return nil, fmt.Errorf(
				"read admin booking cursor: %w",
				err,
			)
		}

		return nil, ErrBookingNotFound
	}

	var booking models.AdminBookingView

	if err := cursor.Decode(&booking); err != nil {
		return nil, fmt.Errorf(
			"decode admin booking: %w",
			err,
		)
	}

	return &booking, nil
}

func buildAdminBookingFilter(
	filter AdminBookingFilter,
) bson.M {
	result := bson.M{}

	if filter.UserID != nil {
		result["user_id"] = *filter.UserID
	}

	if filter.MovieID != nil {
		result["movie_id"] = *filter.MovieID
	}

	if filter.ShowtimeID != nil {
		result["showtime_id"] = *filter.ShowtimeID
	}

	if filter.Status != nil {
		result["status"] = *filter.Status
	}

	if filter.From != nil || filter.To != nil {
		createdAtFilter := bson.M{}

		if filter.From != nil {
			createdAtFilter["$gte"] = filter.From.UTC()
		}

		if filter.To != nil {
			createdAtFilter["$lt"] = filter.To.UTC()
		}

		result["created_at"] = createdAtFilter
	}

	return result
}

func buildAdminBookingPipeline(
	filter bson.M,
	skip *int64,
	limit *int64,
) mongo.Pipeline {
	pipeline := mongo.Pipeline{
		{
			{
				Key:   "$match",
				Value: filter,
			},
		},
		{
			{
				Key: "$sort",
				Value: bson.D{
					{Key: "created_at", Value: -1},
					{Key: "_id", Value: -1},
				},
			},
		},
	}

	if skip != nil {
		pipeline = append(
			pipeline,
			bson.D{
				{
					Key:   "$skip",
					Value: *skip,
				},
			},
		)
	}

	if limit != nil {
		pipeline = append(
			pipeline,
			bson.D{
				{
					Key:   "$limit",
					Value: *limit,
				},
			},
		)
	}

	pipeline = append(
		pipeline,
		bson.D{
			{
				Key: "$lookup",
				Value: bson.D{
					{Key: "from", Value: "users"},
					{Key: "localField", Value: "user_id"},
					{Key: "foreignField", Value: "_id"},
					{Key: "as", Value: "user_document"},
				},
			},
		},
		bson.D{
			{
				Key: "$unwind",
				Value: bson.D{
					{
						Key:   "path",
						Value: "$user_document",
					},
					{
						Key:   "preserveNullAndEmptyArrays",
						Value: true,
					},
				},
			},
		},
		bson.D{
			{
				Key: "$lookup",
				Value: bson.D{
					{Key: "from", Value: "movies"},
					{Key: "localField", Value: "movie_id"},
					{Key: "foreignField", Value: "_id"},
					{Key: "as", Value: "movie_document"},
				},
			},
		},
		bson.D{
			{
				Key: "$unwind",
				Value: bson.D{
					{
						Key:   "path",
						Value: "$movie_document",
					},
					{
						Key:   "preserveNullAndEmptyArrays",
						Value: true,
					},
				},
			},
		},
		bson.D{
			{
				Key: "$project",
				Value: bson.D{
					{Key: "_id", Value: 1},
					{Key: "booking_code", Value: 1},

					{
						Key: "user",
						Value: bson.D{
							{
								Key:   "id",
								Value: "$user_document._id",
							},
							{
								Key:   "name",
								Value: "$user_document.name",
							},
							{
								Key:   "email",
								Value: "$user_document.email",
							},
						},
					},
					{
						Key: "movie",
						Value: bson.D{
							{
								Key:   "id",
								Value: "$movie_document._id",
							},
							{
								Key:   "title",
								Value: "$movie_document.title",
							},
						},
					},

					{Key: "showtime_id", Value: 1},
					{Key: "seat_code", Value: 1},
					{Key: "hall_name", Value: 1},
					{Key: "showtime_start", Value: 1},
					{Key: "price", Value: 1},
					{Key: "currency", Value: 1},
					{Key: "status", Value: 1},
					{Key: "confirmed_at", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "updated_at", Value: 1},
				},
			},
		},
	)

	return pipeline
}
