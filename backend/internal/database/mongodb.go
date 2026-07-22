package database

import (
	"context"
	"fmt"
	"time"

	"cinema-booking/internal/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func ConnectMongoDB() (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(config.App.MongoURI).
		SetMaxPoolSize(100).
		SetMinPoolSize(5).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer disconnectCancel()

		_ = client.Disconnect(disconnectCtx)

		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	db := client.Database(config.App.MongoDatabase)

	return &MongoDB{
		Client:   client,
		Database: db,
	}, nil
}

func (m *MongoDB) Disconnect(ctx context.Context) error {
	if m == nil || m.Client == nil {
		return nil
	}

	if err := m.Client.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnect MongoDB: %w", err)
	}

	return nil
}

func (m *MongoDB) Ping(ctx context.Context) error {
	if m == nil || m.Client == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}

	if err := m.Client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("ping MongoDB: %w", err)
	}

	return nil
}
