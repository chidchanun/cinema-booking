package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName string
	AppPort string

	MongoURI      string
	MongoDatabase string

	JWTSecret    string
	JWTIssuer    string
	JWTAccessTTL time.Duration

	CookieName     string
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite string

	CORSAllowedOrigins []string

	GoogleClientID string

	TMDBAccessToken  string
	TMDBAPIKey       string
	TMDBImageBaseURL string

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	SeatLockTTL time.Duration

	RateLimitWindow    time.Duration
	RateLimitAuth      int
	RateLimitMutation  int
	RateLimitWebSocket int

	RabbitMQURL                string
	RabbitMQExchange           string
	RabbitMQAuditQueue         string
	RabbitMQDeadLetterExchange string
	RabbitMQAuditDLQ           string
	RabbitMQPrefetch           int
	RabbitMQPublishTimeout     time.Duration
}

var App Config

func Load() error {
	_ = godotenv.Load()

	jwtAccessTTL, err := time.ParseDuration(
		getEnv("JWT_ACCESS_TTL", "15m"),
	)

	if err != nil {
		return fmt.Errorf(
			"invalid JWT_ACCESS_TTL: %w",
			err,
		)
	}

	cookieSecure, err := strconv.ParseBool(
		getEnv("COOKIE_SECURE", "false"),
	)
	if err != nil {
		return fmt.Errorf(
			"invalid COOKIE_SECURE: %w",
			err,
		)
	}

	corsAllowedOrigins := parseCommaSeparated(
		getEnv(
			"CORS_ALLOWED_ORIGINS",
			"http://localhost:5173",
		),
	)

	if len(corsAllowedOrigins) == 0 {
		return fmt.Errorf(
			"CORS_ALLOWED_ORIGINS must contain at least one origin",
		)
	}

	for _, origin := range corsAllowedOrigins {
		if origin == "*" {
			return fmt.Errorf(
				"CORS_ALLOWED_ORIGINS cannot contain * when credentials are enabled",
			)
		}
	}

	redisDB, err := strconv.Atoi(
		getEnv("REDIS_DB", "0"),
	)
	if err != nil {
		return fmt.Errorf(
			"invalid REDIS_DB: %w",
			err,
		)
	}

	if redisDB < 0 {
		return fmt.Errorf(
			"REDIS_DB must not be negative",
		)
	}

	seatLockTTL, err := time.ParseDuration(
		getEnv("SEAT_LOCK_TTL", "5m"),
	)
	if err != nil {
		return fmt.Errorf(
			"invalid SEAT_LOCK_TTL: %w",
			err,
		)
	}

	if seatLockTTL <= 0 {
		return fmt.Errorf(
			"SEAT_LOCK_TTL must be greater than zero",
		)
	}

	rabbitMQPrefetch, err := strconv.Atoi(
		getEnv("RABBITMQ_PREFETCH", "10"),
	)
	if err != nil {
		return fmt.Errorf(
			"invalid RABBITMQ_PREFETCH: %w",
			err,
		)
	}

	if rabbitMQPrefetch < 1 || rabbitMQPrefetch > 1000 {
		return fmt.Errorf(
			"RABBITMQ_PREFETCH must be between 1 and 1000",
		)
	}

	rabbitMQPublishTimeout, err := time.ParseDuration(
		getEnv("RABBITMQ_PUBLISH_TIMEOUT", "3s"),
	)
	if err != nil {
		return fmt.Errorf(
			"invalid RABBITMQ_PUBLISH_TIMEOUT: %w",
			err,
		)
	}

	if rabbitMQPublishTimeout <= 0 {
		return fmt.Errorf(
			"RABBITMQ_PUBLISH_TIMEOUT must be greater than zero",
		)
	}

	rateLimitWindow, err := time.ParseDuration(
		getEnv("RATE_LIMIT_WINDOW", "1m"),
	)
	if err != nil || rateLimitWindow <= 0 {
		return fmt.Errorf(
			"RATE_LIMIT_WINDOW must be a positive duration",
		)
	}

	rateLimitAuth, err := parsePositiveInt("RATE_LIMIT_AUTH", 10)
	if err != nil {
		return err
	}
	rateLimitMutation, err := parsePositiveInt("RATE_LIMIT_MUTATION", 60)
	if err != nil {
		return err
	}
	rateLimitWebSocket, err := parsePositiveInt("RATE_LIMIT_WEBSOCKET", 20)
	if err != nil {
		return err
	}

	App = Config{
		AppName:            getEnv("APP_NAME", "Cinema Booking"),
		AppPort:            getEnv("APP_PORT", "8080"),
		CORSAllowedOrigins: corsAllowedOrigins,
		MongoURI:           strings.TrimSpace(os.Getenv("MONGO_URI")),
		MongoDatabase:      strings.TrimSpace(os.Getenv("MONGO_DATABASE")),

		JWTSecret:    strings.TrimSpace(os.Getenv("JWT_SECRET")),
		JWTIssuer:    getEnv("JWT_ISSUER", "cinema-booking-api"),
		JWTAccessTTL: jwtAccessTTL,

		CookieName: getEnv(
			"COOKIE_NAME",
			"cinema_access_token",
		),
		CookieDomain: strings.TrimSpace(
			os.Getenv("COOKIE_DOMAIN"),
		),
		CookieSecure: cookieSecure,
		CookieSameSite: strings.ToLower(
			getEnv("COOKIE_SAME_SITE", "lax"),
		),

		GoogleClientID: strings.TrimSpace(
			os.Getenv("GOOGLE_CLIENT_ID"),
		),

		TMDBAccessToken: strings.TrimSpace(os.Getenv("TMDB_ACCESS_TOKEN")),
		TMDBAPIKey:      strings.TrimSpace(os.Getenv("TMDB_API_KEY")),
		TMDBImageBaseURL: strings.TrimRight(
			getEnv("TMDB_IMAGE_BASE_URL", "https://image.tmdb.org/t/p/w500"),
			"/",
		),

		SMTPHost:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     getEnv("SMTP_FROM", "Cinema Booking <no-reply@cinema.local>"),

		RedisAddr: getEnv(
			"REDIS_ADDR",
			"localhost:6379",
		),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       redisDB,

		SeatLockTTL: seatLockTTL,

		RateLimitWindow:    rateLimitWindow,
		RateLimitAuth:      rateLimitAuth,
		RateLimitMutation:  rateLimitMutation,
		RateLimitWebSocket: rateLimitWebSocket,

		RabbitMQURL: getEnv(
			"RABBITMQ_URL",
			"amqp://cinema:cinema_secret@localhost:5672/",
		),

		RabbitMQExchange: getEnv(
			"RABBITMQ_EXCHANGE",
			"cinema.events",
		),

		RabbitMQAuditQueue: getEnv(
			"RABBITMQ_AUDIT_QUEUE",
			"cinema.audit.events",
		),

		RabbitMQDeadLetterExchange: getEnv(
			"RABBITMQ_DEAD_LETTER_EXCHANGE",
			"cinema.events.dlx",
		),

		RabbitMQAuditDLQ: getEnv(
			"RABBITMQ_AUDIT_DLQ",
			"cinema.audit.events.dlq",
		),

		RabbitMQPrefetch: rabbitMQPrefetch,

		RabbitMQPublishTimeout: rabbitMQPublishTimeout,
	}

	if App.MongoURI == "" {
		return fmt.Errorf("MONGO_URI is required")
	}

	if App.MongoDatabase == "" {
		return fmt.Errorf("MONGO_DATABASE is required")
	}

	if App.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if len(App.JWTSecret) < 32 {
		return fmt.Errorf(
			"JWT_SECRET must contain at least 32 characters",
		)
	}

	if App.JWTIssuer == "" {
		return fmt.Errorf("JWT_ISSUER is required")
	}

	if App.JWTAccessTTL <= 0 {
		return fmt.Errorf(
			"JWT_ACCESS_TTL must be greater than zero",
		)
	}

	if App.CookieName == "" {
		return fmt.Errorf("COOKIE_NAME is required")
	}

	if App.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	return value
}

func parseCommaSeparated(value string) []string {
	rawValues := strings.Split(value, ",")

	result := make([]string, 0, len(rawValues))

	for _, rawValue := range rawValues {
		item := strings.TrimSpace(rawValue)

		if item == "" {
			continue
		}

		result = append(result, item)
	}

	return result
}

func parsePositiveInt(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(getEnv(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}
