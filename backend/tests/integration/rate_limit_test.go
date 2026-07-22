//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	authmiddleware "cinema-booking/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterEnforcesLimit(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("set RUN_INTEGRATION_TESTS=1 to run integration tests")
	}

	address := strings.TrimSpace(os.Getenv("REDIS_TEST_ADDR"))
	if address == "" {
		address = "localhost:6380"
	}

	client := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("prepare Redis rate-limit database: %v", err)
	}

	limiter, err := authmiddleware.NewRateLimiter(client, time.Minute, nil)
	if err != nil {
		t.Fatalf("create Redis rate limiter: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(
		"/limited",
		limiter.Limit("integration", 1),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/limited", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected first request status 204, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/limited", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d", second.Code)
	}
}
