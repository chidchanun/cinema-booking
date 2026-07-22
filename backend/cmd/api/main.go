package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	authpkg "cinema-booking/internal/auth"
	"cinema-booking/internal/config"
	"cinema-booking/internal/database"
	"cinema-booking/internal/handlers"
	"cinema-booking/internal/messaging"
	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/notification"
	"cinema-booking/internal/observability"
	"cinema-booking/internal/realtime"
	"cinema-booking/internal/redislock"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/routes"
	"cinema-booking/internal/services"
	"cinema-booking/internal/tmdb"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {

	appCtx, appCancel := context.WithCancel(
		context.Background(),
	)

	// โหลดค่าจาก Environment Variables
	if err := config.Load(); err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// เชื่อมต่อ MongoDB
	mongoDB, err := database.ConnectMongoDB()
	if err != nil {
		log.Fatalf("connect MongoDB: %v", err)
	}

	log.Printf(
		"Connected to MongoDB database: %s",
		config.App.MongoDatabase,
	)

	// สร้าง MongoDB indexes
	indexCtx, indexCancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)

	if err := database.CreateIndexes(
		indexCtx,
		mongoDB.Database,
	); err != nil {
		indexCancel()

		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		if disconnectErr := mongoDB.Disconnect(disconnectCtx); disconnectErr != nil {
			log.Printf(
				"MongoDB disconnect after index error: %v",
				disconnectErr,
			)
		}

		disconnectCancel()

		log.Fatalf("create MongoDB indexes: %v", err)
	}

	indexCancel()

	log.Println("MongoDB indexes initialized")

	redisDB, err := database.ConnectRedis()
	if err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		_ = mongoDB.Disconnect(disconnectCtx)
		disconnectCancel()

		log.Fatalf(
			"connect Redis: %v",
			err,
		)
	}

	log.Printf(
		"Connected to Redis: %s",
		config.App.RedisAddr,
	)

	rabbitMQ, err := database.ConnectRabbitMQ()
	if err != nil {
		appCancel()

		_ = redisDB.Close()

		mongoDisconnectCtx, mongoDisconnectCancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)

		_ = mongoDB.Disconnect(
			mongoDisconnectCtx,
		)

		mongoDisconnectCancel()

		log.Fatalf(
			"connect RabbitMQ: %v",
			err,
		)
	}

	log.Println("Connected to RabbitMQ")

	rabbitMQTopology := messaging.TopologyConfig{
		Exchange: config.App.RabbitMQExchange,

		AuditQueue: config.App.RabbitMQAuditQueue,

		DeadLetterExchange: config.App.RabbitMQDeadLetterExchange,

		AuditDeadLetterQueue: config.App.RabbitMQAuditDLQ,
	}

	bookingEventPublisher, err :=
		messaging.NewRecoveringPublisher(
			rabbitMQ.Connection,
			config.App.RabbitMQURL,
			rabbitMQTopology,
		)
	if err != nil {
		appCancel()

		_ = rabbitMQ.Close()
		_ = redisDB.Close()

		mongoDisconnectCtx, mongoDisconnectCancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)

		_ = mongoDB.Disconnect(
			mongoDisconnectCtx,
		)

		mongoDisconnectCancel()

		log.Fatalf(
			"create RabbitMQ publisher: %v",
			err,
		)
	}

	// สร้าง Gin Router
	router := gin.New()

	realtimeHub := realtime.NewHub(512)

	// Hub ทำงานตลอดอายุ application และปิด WebSocket clients ทั้งหมดเมื่อ appCtx ถูกยกเลิก
	go realtimeHub.Run(appCtx)

	router.Use(
		observability.Middleware(),
		authmiddleware.RequestID(),
		authmiddleware.RequestLogger(logger),
		gin.Recovery(),
	)

	router.Use(cors.New(cors.Config{
		AllowOrigins: config.App.CORSAllowedOrigins,

		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"X-CSRF-Token",
			"X-Seat-Lock-Token",
			"X-Request-ID",
		},

		ExposeHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"Retry-After",
		},

		AllowCredentials: true,

		MaxAge: 12 * time.Hour,
	}))

	// Repository
	userRepository := repository.NewUserRepository(
		mongoDB.Database,
	)

	movieRepository := repository.NewMovieRepository(
		mongoDB.Database,
	)

	showtimeRepository := repository.NewShowtimeRepository(
		mongoDB.Database,
	)

	bookingRepository := repository.NewBookingRepository(
		mongoDB.Client,
		mongoDB.Database,
	)

	auditLogRepository := repository.NewAuditLogRepository(
		mongoDB.Database,
	)

	seatLockManager, err :=
		redislock.NewSeatLockManager(
			redisDB.Client,
			config.App.SeatLockTTL,
		)
	if err != nil {
		_ = redisDB.Close()

		disconnectCtx, disconnectCancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)

		_ = mongoDB.Disconnect(disconnectCtx)
		disconnectCancel()

		log.Fatalf(
			"create seat lock manager: %v",
			err,
		)
	}

	// Google ID Token Verifier
	googleVerifier, err := authpkg.NewGoogleVerifier(
		config.App.GoogleClientID,
	)
	if err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		if disconnectErr := mongoDB.Disconnect(disconnectCtx); disconnectErr != nil {
			log.Printf(
				"MongoDB disconnect after Google verifier error: %v",
				disconnectErr,
			)
		}

		disconnectCancel()

		log.Fatalf("create Google verifier: %v", err)
	}

	// JWTService issues application access tokens after Google verifies the
	// user's identity. Google ID tokens must not be used as API access tokens.
	jwtService, err := authpkg.NewJWTService(
		config.App.JWTSecret,
		config.App.JWTIssuer,
		config.App.JWTAccessTTL,
	)
	if err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		if disconnectErr := mongoDB.Disconnect(disconnectCtx); disconnectErr != nil {
			log.Printf(
				"MongoDB disconnect after JWT service error: %v",
				disconnectErr,
			)
		}

		disconnectCancel()

		log.Fatalf("create JWT service: %v", err)
	}

	// CookieService writes the issued access token to an HTTP-only cookie.
	cookieService, err := authpkg.NewCookieService(
		config.App.CookieName,
		config.App.CookieDomain,
		config.App.CookieSecure,
		config.App.CookieSameSite,
	)
	if err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		if disconnectErr := mongoDB.Disconnect(
			disconnectCtx,
		); disconnectErr != nil {
			log.Printf(
				"MongoDB disconnect after cookie service error: %v",
				disconnectErr,
			)
		}

		disconnectCancel()

		log.Fatalf(
			"create cookie service: %v",
			err,
		)
	}

	authService := services.NewAuthService(
		googleVerifier,
		userRepository,
		jwtService,
	)

	movieService := services.NewMovieService(
		movieRepository,
	)

	showtimeService := services.NewShowtimeService(
		showtimeRepository,
		movieRepository,
	)

	seatService := services.NewSeatService(
		showtimeRepository,
		seatLockManager,
		realtimeHub,
		bookingEventPublisher,
		config.App.RabbitMQPublishTimeout,
	)

	bookingService := services.NewBookingService(
		bookingRepository,
		showtimeRepository,
		seatLockManager,
		realtimeHub,
		bookingEventPublisher,
		config.App.RabbitMQPublishTimeout,
	)

	adminBookingService := services.NewAdminBookingService(
		bookingRepository,
	)

	auditLogService := services.NewAuditLogService(
		auditLogRepository,
	)

	tmdbClient := tmdb.NewClient(
		&http.Client{Timeout: 8 * time.Second},
		config.App.TMDBAccessToken,
		config.App.TMDBAPIKey,
		config.App.TMDBImageBaseURL,
	)

	// Handlers

	authHandler := handlers.NewAuthHandler(
		authService,
		cookieService,
	)

	movieHandler := handlers.NewMovieHandler(
		movieService,
	)

	showtimeHandler := handlers.NewShowtimeHandler(
		showtimeService,
	)

	adminBookingHandler := handlers.NewAdminBookingHandler(
		adminBookingService,
	)

	auditLogHandler := handlers.NewAuditLogHandler(
		auditLogService,
	)

	emailSender := notification.NewSMTPEmailSender(
		config.App.SMTPHost,
		config.App.SMTPPort,
		config.App.SMTPUsername,
		config.App.SMTPPassword,
		config.App.SMTPFrom,
	)
	paymentNotificationHandler := handlers.NewPaymentNotificationHandler(
		userRepository,
		showtimeRepository,
		emailSender,
	)

	tmdbHandler := handlers.NewTMDBHandler(tmdbClient)

	realtimeHandler, err := realtime.NewHandler(
		realtimeHub,
		showtimeService,
		config.App.CORSAllowedOrigins,
	)

	if err != nil {
		appCancel()

		_ = redisDB.Close()

		disconnectCtx, disconnectCancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)

		_ = mongoDB.Disconnect(disconnectCtx)
		disconnectCancel()

		log.Fatalf(
			"create realtime handler: %v",
			err,
		)
	}

	expirySubscriber :=
		redislock.NewExpirySubscriber(
			redisDB.Client,
			config.App.RedisDB,
		)

	go func() {
		err := expirySubscriber.Run(
			appCtx,
			func(
				ctx context.Context,
				showtimeID primitive.ObjectID,
				seatCode string,
			) error {
				return seatService.HandleExpiredSeatLock(
					ctx,
					showtimeID,
					seatCode,
				)
			},
		)

		if err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Printf(
				"Redis expiry subscriber stopped: %v",
				err,
			)
		}
	}()

	seatHandler := handlers.NewSeatHandler(
		seatService,
	)

	bookingHandler := handlers.NewBookingHandler(
		bookingService,
	)

	adminHandler := handlers.NewAdminHandler()

	authenticationMiddleware := authmiddleware.NewAuthMiddleware(
		cookieService,
		jwtService,
	)

	rateLimiter, err := authmiddleware.NewRateLimiter(
		redisDB.Client,
		config.App.RateLimitWindow,
		logger,
	)
	if err != nil {
		log.Fatalf("create rate limiter: %v", err)
	}

	routes.Register(
		router,
		authHandler,
		adminHandler,
		movieHandler,
		showtimeHandler,
		seatHandler,
		bookingHandler,
		realtimeHandler,
		adminBookingHandler,
		auditLogHandler,
		tmdbHandler,
		paymentNotificationHandler,
		authenticationMiddleware,
		rateLimiter,
		config.App.RateLimitAuth,
		config.App.RateLimitMutation,
		config.App.RateLimitWebSocket,
	)

	// Liveness only reports whether the HTTP process is serving requests.
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"app":    config.App.AppName,
			"status": "alive",
		})
	})

	readinessHandler := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			3*time.Second,
		)
		defer cancel()

		if err := mongoDB.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"app":      config.App.AppName,
				"status":   "not_ready",
				"mongodb":  "disconnected",
				"redis":    "unknown",
				"rabbitmq": "unknown",
			})
			return
		}

		if err := redisDB.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"app":      config.App.AppName,
				"status":   "not_ready",
				"mongodb":  "connected",
				"redis":    "disconnected",
				"rabbitmq": "unknown",
			})
			return
		}

		if !bookingEventPublisher.IsHealthy() {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"app":      config.App.AppName,
					"status":   "not_ready",
					"mongodb":  "connected",
					"redis":    "connected",
					"rabbitmq": "disconnected",
				},
			)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"app":      config.App.AppName,
			"status":   "ready",
			"mongodb":  "connected",
			"redis":    "connected",
			"rabbitmq": "connected",
		})
	}

	// Keep /health as a backwards-compatible readiness endpoint.
	router.GET("/health", readinessHandler)
	router.GET("/health/ready", readinessHandler)
	router.GET("/metrics", observability.Handler)

	// HTTP Server
	server := &http.Server{
		Addr:              ":" + config.App.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// รับ Error จาก HTTP Server
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf(
			"%s started on http://localhost:%s",
			config.App.AppName,
			config.App.AppPort,
		)

		err := server.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// รับ Shutdown Signal
	shutdownSignal := make(chan os.Signal, 1)

	signal.Notify(
		shutdownSignal,
		os.Interrupt,
		syscall.SIGTERM,
	)

	select {
	case sig := <-shutdownSignal:
		log.Printf(
			"Shutdown signal received: %s",
			sig,
		)

	case err := <-serverErrors:
		log.Printf(
			"HTTP server error: %v",
			err,
		)
	}

	// หยุดรับ Signal เพิ่ม
	signal.Stop(shutdownSignal)

	// Graceful Shutdown HTTP Server
	httpShutdownCtx, httpShutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	if err := server.Shutdown(httpShutdownCtx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}

	httpShutdownCancel()

	// Stop the WebSocket hub and Redis expiry subscriber before closing their
	// underlying connections.
	appCancel()

	if err := bookingEventPublisher.Close(); err != nil {
		log.Printf(
			"RabbitMQ publisher close error: %v",
			err,
		)
	}

	if err := rabbitMQ.Close(); err != nil {
		log.Printf(
			"RabbitMQ close error: %v",
			err,
		)
	}

	if err := redisDB.Close(); err != nil {
		log.Printf(
			"Redis close error: %v",
			err,
		)
	}

	// ปิด MongoDB ด้วย Context แยกจาก HTTP Server
	mongoShutdownCtx, mongoShutdownCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	if err := mongoDB.Disconnect(mongoShutdownCtx); err != nil {
		log.Printf(
			"MongoDB disconnect error: %v",
			err,
		)
	}

	mongoShutdownCancel()

	log.Println("Application stopped")
}
