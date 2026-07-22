package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type rateLimitTestStore struct {
	count int64
	err   error
}

func (s *rateLimitTestStore) Eval(
	ctx context.Context,
	_ string,
	_ []string,
	_ ...interface{},
) *redis.Cmd {
	command := redis.NewCmd(ctx)
	if s.err != nil {
		command.SetErr(s.err)
		return command
	}

	s.count++
	command.SetVal([]interface{}{s.count, int64(60)})
	return command
}

func TestRateLimiterRejectsRequestsAboveLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &rateLimitTestStore{}
	limiter, err := NewRateLimiter(store, time.Minute, nil)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	router := gin.New()
	router.POST(
		"/login",
		limiter.Limit("auth", 1),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/login", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("expected first status 204, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/login", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second status 429, got %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimiterFailsOpenWhenRedisIsUnavailable(t *testing.T) {
	limiter, err := NewRateLimiter(
		&rateLimitTestStore{err: errors.New("Redis unavailable")},
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("create rate limiter: %v", err)
	}

	router := gin.New()
	router.POST(
		"/booking",
		limiter.Limit("booking", 1),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPost, "/booking", nil),
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open status 204, got %d", response.Code)
	}
}
