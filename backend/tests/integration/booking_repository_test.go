//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	dbsetup "cinema-booking/internal/database"
	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func TestBookingRepositoryPreventsDoubleBooking(
	t *testing.T,
) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip(
			"set RUN_INTEGRATION_TESTS=1 to run integration tests",
		)
	}

	mongoURI := os.Getenv("MONGO_TEST_URI")
	if mongoURI == "" {
		t.Skip(
			"MONGO_TEST_URI is required for MongoDB integration test",
		)
	}

	connectContext, connectCancel :=
		context.WithTimeout(
			context.Background(),
			15*time.Second,
		)
	defer connectCancel()

	client, err := mongo.Connect(
		connectContext,
		options.Client().ApplyURI(mongoURI),
	)
	if err != nil {
		t.Fatalf("connect MongoDB: %v", err)
	}

	t.Cleanup(func() {
		disconnectContext, cancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
		defer cancel()

		_ = client.Disconnect(disconnectContext)
	})

	if err := client.Ping(
		connectContext,
		readpref.Primary(),
	); err != nil {
		t.Fatalf("ping MongoDB: %v", err)
	}

	/*
		ตรวจว่า MongoDB ทำงานเป็น Replica Set

		Multi-document Transaction ใช้กับ Standalone
		MongoDB ไม่ได้
	*/
	var helloResult bson.M

	err = client.Database("admin").
		RunCommand(
			connectContext,
			bson.D{
				{
					Key:   "hello",
					Value: 1,
				},
			},
		).
		Decode(&helloResult)

	if err != nil {
		t.Fatalf(
			"read MongoDB topology: %v",
			err,
		)
	}

	if _, replicaSetEnabled :=
		helloResult["setName"]; !replicaSetEnabled {
		t.Skip(
			"MongoDB integration test requires a replica set",
		)
	}

	databaseName :=
		"cinema_booking_test_" +
			primitive.NewObjectID().Hex()

	database := client.Database(databaseName)
	if err := dbsetup.CreateIndexes(connectContext, database); err != nil {
		t.Fatalf("create integration test indexes: %v", err)
	}

	t.Cleanup(func() {
		dropContext, cancel :=
			context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
		defer cancel()

		_ = database.Drop(dropContext)
	})

	seededAdminEmail := "seeded-admin@example.com"
	if _, err := database.Collection("users").InsertOne(
		context.Background(),
		bson.M{
			"email":          seededAdminEmail,
			"email_verified": false,
			"name":           seededAdminEmail,
			"picture":        "",
			"role":           models.RoleAdmin,
			"last_login_at":  time.Unix(0, 0).UTC(),
			"created_at":     time.Now().UTC(),
			"updated_at":     time.Now().UTC(),
		},
	); err != nil {
		t.Fatalf("insert seeded admin fixture: %v", err)
	}

	seededAdmin, err := repository.NewUserRepository(database).UpsertGoogleUser(
		context.Background(),
		models.GoogleUserProfile{
			GoogleID:      "google-seeded-admin-id",
			Email:         seededAdminEmail,
			EmailVerified: true,
			Name:          "Seeded Admin",
		},
	)
	if err != nil {
		t.Fatalf("claim seeded admin with Google identity: %v", err)
	}
	if seededAdmin.Role != models.RoleAdmin {
		t.Fatalf("expected seeded role ADMIN, got %s", seededAdmin.Role)
	}
	if seededAdmin.GoogleID != "google-seeded-admin-id" {
		t.Fatalf("expected seeded admin Google identity to be attached")
	}

	showtimeID := primitive.NewObjectID()
	movieID := primitive.NewObjectID()

	showtime := &models.Showtime{
		ID:      showtimeID,
		MovieID: movieID,

		HallName: "Integration Hall",

		StartTime: time.Now().
			UTC().
			Add(3 * time.Hour),

		EndTime: time.Now().
			UTC().
			Add(5 * time.Hour),

		Price:    250,
		Currency: "THB",

		SeatRows:    1,
		SeatsPerRow: 1,
		TotalSeats:  1,

		Seats: []models.Seat{
			{
				Code:   "A1",
				Row:    "A",
				Number: 1,
				Status: models.SeatStatusAvailable,
			},
		},

		ShowtimeStatus: models.ShowtimeStatusActive,

		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err = database.
		Collection("showtimes").
		InsertOne(
			connectContext,
			showtime,
		)
	if err != nil {
		t.Fatalf(
			"insert test showtime: %v",
			err,
		)
	}

	bookingRepository :=
		repository.NewBookingRepository(
			client,
			database,
		)

	const requestCount = 16

	type bookingResult struct {
		err error
	}

	results := make(
		chan bookingResult,
		requestCount,
	)

	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	for index := 0; index < requestCount; index++ {
		index := index

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			<-start

			bookingID := primitive.NewObjectID()

			booking := &models.Booking{
				ID: bookingID,

				BookingCode: fmt.Sprintf(
					"BK-INTEGRATION-%02d-%s",
					index,
					bookingID.Hex(),
				),

				UserID:     primitive.NewObjectID(),
				MovieID:    movieID,
				ShowtimeID: showtimeID,

				SeatCode: "A1",
				HallName: "Integration Hall",

				ShowtimeStart: showtime.StartTime,

				Price:    250,
				Currency: "THB",
			}

			err := bookingRepository.Confirm(
				context.Background(),
				booking,
			)

			results <- bookingResult{
				err: err,
			}
		}()
	}

	close(start)

	waitGroup.Wait()
	close(results)

	successCount := 0
	conflictCount := 0

	for result := range results {
		if result.err == nil {
			successCount++
			continue
		}

		if errors.Is(
			result.err,
			repository.ErrSeatUnavailable,
		) ||
			errors.Is(
				result.err,
				repository.ErrBookingConflict,
			) {
			conflictCount++
			continue
		}

		t.Fatalf(
			"unexpected booking error: %v",
			result.err,
		)
	}

	if successCount != 1 {
		t.Fatalf(
			"expected exactly 1 successful booking, got %d",
			successCount,
		)
	}

	if conflictCount != requestCount-1 {
		t.Fatalf(
			"expected %d conflicts, got %d",
			requestCount-1,
			conflictCount,
		)
	}

	bookingCount, err := database.
		Collection("bookings").
		CountDocuments(
			context.Background(),
			bson.M{
				"showtime_id": showtimeID,
				"seat_code":   "A1",
			},
		)

	if err != nil {
		t.Fatalf(
			"count stored bookings: %v",
			err,
		)
	}

	if bookingCount != 1 {
		t.Fatalf(
			"expected 1 MongoDB booking, got %d",
			bookingCount,
		)
	}

	var storedShowtime models.Showtime

	err = database.
		Collection("showtimes").
		FindOne(
			context.Background(),
			bson.M{
				"_id": showtimeID,
			},
		).
		Decode(&storedShowtime)

	if err != nil {
		t.Fatalf(
			"read stored showtime: %v",
			err,
		)
	}

	if len(storedShowtime.Seats) != 1 {
		t.Fatalf(
			"expected 1 seat, got %d",
			len(storedShowtime.Seats),
		)
	}

	if storedShowtime.Seats[0].Status !=
		models.SeatStatusBooked {
		t.Fatalf(
			"expected seat BOOKED, got %s",
			storedShowtime.Seats[0].Status,
		)
	}

	// A duplicate booking code fails after the conditional seat update. The
	// transaction must roll that update back so the second seat stays available.
	rollbackShowtime := *showtime
	rollbackShowtime.ID = primitive.NewObjectID()
	rollbackShowtime.Seats = []models.Seat{
		{
			Code:   "B1",
			Row:    "B",
			Number: 1,
			Status: models.SeatStatusAvailable,
		},
	}

	if _, err := database.Collection("showtimes").InsertOne(
		context.Background(),
		&rollbackShowtime,
	); err != nil {
		t.Fatalf("insert rollback test showtime: %v", err)
	}

	duplicateCode := "BK-ROLLBACK-" + primitive.NewObjectID().Hex()
	if _, err := database.Collection("bookings").InsertOne(
		context.Background(),
		&models.Booking{
			ID:          primitive.NewObjectID(),
			BookingCode: duplicateCode,
			UserID:      primitive.NewObjectID(),
			MovieID:     movieID,
			ShowtimeID:  primitive.NewObjectID(),
			SeatCode:    "Z1",
			Status:      models.BookingStatusBooked,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
			ConfirmedAt: time.Now().UTC(),
		},
	); err != nil {
		t.Fatalf("insert duplicate-code fixture: %v", err)
	}

	err = bookingRepository.Confirm(
		context.Background(),
		&models.Booking{
			ID:            primitive.NewObjectID(),
			BookingCode:   duplicateCode,
			UserID:        primitive.NewObjectID(),
			MovieID:       movieID,
			ShowtimeID:    rollbackShowtime.ID,
			SeatCode:      "B1",
			HallName:      rollbackShowtime.HallName,
			ShowtimeStart: rollbackShowtime.StartTime,
			Price:         rollbackShowtime.Price,
			Currency:      rollbackShowtime.Currency,
		},
	)
	if !errors.Is(err, repository.ErrBookingConflict) {
		t.Fatalf("expected booking conflict, got %v", err)
	}

	var afterRollback models.Showtime
	if err := database.Collection("showtimes").FindOne(
		context.Background(),
		bson.M{"_id": rollbackShowtime.ID},
	).Decode(&afterRollback); err != nil {
		t.Fatalf("read rollback test showtime: %v", err)
	}
	if afterRollback.Seats[0].Status != models.SeatStatusAvailable {
		t.Fatalf(
			"expected rollback seat AVAILABLE, got %s",
			afterRollback.Seats[0].Status,
		)
	}
}
