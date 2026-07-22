package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const rateLimitScript = `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("TTL", KEYS[1])
return {count, ttl}
`

type RateLimitStore interface {
	Eval(
		ctx context.Context,
		script string,
		keys []string,
		args ...interface{},
	) *redis.Cmd
}

type RateLimiter struct {
	store  RateLimitStore
	window time.Duration
	logger *slog.Logger
}

func NewRateLimiter(
	store RateLimitStore,
	window time.Duration,
	logger *slog.Logger,
) (*RateLimiter, error) {
	if store == nil {
		return nil, fmt.Errorf("rate limiter: Redis store is nil")
	}
	if window <= 0 {
		return nil, fmt.Errorf("rate limiter: window must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &RateLimiter{store: store, window: window, logger: logger}, nil
}

func (l *RateLimiter) Limit(scope string, maximum int) gin.HandlerFunc {
	return l.limit(scope, maximum, false)
}

func (l *RateLimiter) LimitMutations(scope string, maximum int) gin.HandlerFunc {
	return l.limit(scope, maximum, true)
}

func (l *RateLimiter) limit(
	scope string,
	maximum int,
	mutationsOnly bool,
) gin.HandlerFunc {
	scope = strings.TrimSpace(scope)
	if scope == "" || maximum < 1 {
		panic("rate limiter: scope and positive maximum are required")
	}

	return func(c *gin.Context) {
		if mutationsOnly && isSafeHTTPMethod(c.Request.Method) {
			c.Next()
			return
		}

		identity := c.ClientIP()
		if userID, ok := GetUserID(c); ok {
			identity = userID.Hex()
		}
		key := "rate_limit:" + scope + ":" + identity

		windowSeconds := int64(l.window / time.Second)
		if windowSeconds < 1 {
			windowSeconds = 1
		}

		result, err := l.store.Eval(
			c.Request.Context(),
			rateLimitScript,
			[]string{key},
			windowSeconds,
		).Slice()
		if err != nil {
			l.logger.ErrorContext(
				c.Request.Context(),
				"Rate limiter unavailable; allowing request",
				"request_id", GetRequestID(c),
				"scope", scope,
				"error", err,
			)
			c.Next()
			return
		}

		count, countOK := redisInt64(result, 0)
		ttl, ttlOK := redisInt64(result, 1)
		if !countOK || !ttlOK {
			l.logger.ErrorContext(
				c.Request.Context(),
				"Rate limiter returned an invalid result",
				"request_id", GetRequestID(c),
				"scope", scope,
			)
			c.Next()
			return
		}
		if ttl < 0 {
			ttl = windowSeconds
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(maximum))
		remaining := int64(maximum) - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if count > int64(maximum) {
			c.Header("Retry-After", strconv.FormatInt(ttl, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests",
				"retry_after": ttl,
			})
			return
		}

		c.Next()
	}
}

func redisInt64(values []interface{}, index int) (int64, bool) {
	if index < 0 || index >= len(values) {
		return 0, false
	}

	switch value := values[index].(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
