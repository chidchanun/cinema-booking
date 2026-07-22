package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ContextRequestIDKey = "request_id"
	RequestIDHeader     = "X-Request-ID"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = newRequestID()
		}

		c.Set(ContextRequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		attributes := []any{
			"request_id", GetRequestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if userID, ok := GetUserID(c); ok {
			attributes = append(attributes, "user_id", userID.Hex())
		}

		level := slog.LevelInfo
		if c.Writer.Status() >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if c.Writer.Status() >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		logger.Log(c.Request.Context(), level, "HTTP request", attributes...)
	}
}

func GetRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}

	value, exists := c.Get(ContextRequestIDKey)
	if !exists {
		return ""
	}

	requestID, _ := value.(string)
	return requestID
}

func newRequestID() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(randomBytes)
}
