//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"cinema-booking/internal/redislock"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRedisSeatLockConcurrentAcquire(
	t *testing.T,
) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip(
			"set RUN_INTEGRATION_TESTS=1 to run integration tests",
		)
	}

	redisAddress := os.Getenv("REDIS_TEST_ADDR")
	if redisAddress == "" {
		// Docker Compose exposes this project's Redis on host port 6380.
		redisAddress = "localhost:6380"
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddress,
		DB:   15,
	})

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		_ = client.FlushDB(ctx).Err()
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf(
			"connect Redis test server: %v",
			err,
		)
	}

	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush Redis test DB: %v", err)
	}

	lockManager, err := redislock.NewSeatLockManager(
		client,
		5*time.Second,
	)
	if err != nil {
		t.Fatalf(
			"create seat lock manager: %v",
			err,
		)
	}

	showtimeID := primitive.NewObjectID()

	const workerCount = 32

	type acquireResult struct {
		userID primitive.ObjectID
		lockID string
		err    error
	}

	results := make(
		chan acquireResult,
		workerCount,
	)

	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	for index := 0; index < workerCount; index++ {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			userID := primitive.NewObjectID()

			<-start

			result, err := lockManager.Acquire(
				ctx,
				showtimeID,
				"A1",
				userID,
			)

			lockID := ""

			if result != nil {
				lockID = result.LockID
			}

			results <- acquireResult{
				userID: userID,
				lockID: lockID,
				err:    err,
			}
		}()
	}

	close(start)

	waitGroup.Wait()
	close(results)

	successCount := 0
	lockedCount := 0

	var winner acquireResult

	for result := range results {
		if result.err == nil {
			successCount++
			winner = result
			continue
		}

		if errors.Is(
			result.err,
			redislock.ErrSeatAlreadyLocked,
		) {
			lockedCount++
			continue
		}

		t.Fatalf(
			"unexpected Redis lock error: %v",
			result.err,
		)
	}

	if successCount != 1 {
		t.Fatalf(
			"expected exactly 1 lock winner, got %d",
			successCount,
		)
	}

	if lockedCount != workerCount-1 {
		t.Fatalf(
			"expected %d locked results, got %d",
			workerCount-1,
			lockedCount,
		)
	}

	wrongOwner := primitive.NewObjectID()

	err = lockManager.Release(
		ctx,
		showtimeID,
		"A1",
		wrongOwner,
		winner.lockID,
	)

	if !errors.Is(
		err,
		redislock.ErrSeatLockNotOwned,
	) {
		t.Fatalf(
			"expected ErrSeatLockNotOwned, got %v",
			err,
		)
	}

	err = lockManager.Release(
		ctx,
		showtimeID,
		"A1",
		winner.userID,
		winner.lockID,
	)
	if err != nil {
		t.Fatalf(
			"winner could not release lock: %v",
			err,
		)
	}

	nextUser := primitive.NewObjectID()

	nextResult, err := lockManager.Acquire(
		ctx,
		showtimeID,
		"A1",
		nextUser,
	)
	if err != nil {
		t.Fatalf(
			"seat could not be locked after release: %v",
			err,
		)
	}

	if nextResult == nil ||
		nextResult.LockID == "" {
		t.Fatal("new lock result is invalid")
	}
}
