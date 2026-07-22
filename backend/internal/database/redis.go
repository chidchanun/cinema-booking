package database

import (
	"context"
	"fmt"
	"time"

	"cinema-booking/internal/config"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	Client *redis.Client
}

func ConnectRedis() (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.App.RedisAddr,
		Password: config.App.RedisPassword,
		DB:       config.App.RedisDB,

		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		MaxRetries: 3,
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, fmt.Errorf(
			"ping Redis: %w",
			err,
		)
	}

	return &RedisDB{
		Client: client,
	}, nil
}

func (r *RedisDB) Ping(
	ctx context.Context,
) error {
	if r == nil || r.Client == nil {
		return fmt.Errorf(
			"Redis client is not initialized",
		)
	}

	if err := r.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf(
			"ping Redis: %w",
			err,
		)
	}

	return nil
}

func (r *RedisDB) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}

	if err := r.Client.Close(); err != nil {
		return fmt.Errorf(
			"close Redis: %w",
			err,
		)
	}

	return nil
}
