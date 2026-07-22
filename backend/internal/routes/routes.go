package routes

import (
	"cinema-booking/internal/handlers"
	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"
	"cinema-booking/internal/realtime"

	"github.com/gin-gonic/gin"
)

func Register(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	adminHandler *handlers.AdminHandler,
	movieHandler *handlers.MovieHandler,
	showtimeHandler *handlers.ShowtimeHandler,
	seatHandler *handlers.SeatHandler,
	bookingHandler *handlers.BookingHandler,
	realtimeHandler *realtime.Handler,
	adminBookingHandler *handlers.AdminBookingHandler,
	auditLogHandler *handlers.AuditLogHandler,
	tmdbHandler *handlers.TMDBHandler,
	paymentNotificationHandler *handlers.PaymentNotificationHandler,
	authMiddleware *authmiddleware.AuthMiddleware,
	rateLimiter *authmiddleware.RateLimiter,
	authRateLimit int,
	mutationRateLimit int,
	webSocketRateLimit int,
) {

	api := router.Group("/api/v1")

	realtimeRoutes := api.Group("/ws")

	realtimeRoutes.Use(
		authMiddleware.RequireAuth(),
		rateLimiter.Limit(
			"websocket",
			webSocketRateLimit,
		),
	)

	{
		realtimeRoutes.GET(
			"/showtimes/:showtimeID/seats",
			realtimeHandler.Connect,
		)
	}

	authRoutes := api.Group("/auth")
	{
		authRoutes.POST(
			"/google",
			rateLimiter.Limit(
				"auth",
				authRateLimit,
			),
			authHandler.GoogleLogin,
		)

		authRoutes.POST(
			"/logout",
			authMiddleware.RequireAuth(),
			authMiddleware.RequireCSRF(),
			authHandler.Logout,
		)

		authRoutes.GET(
			"/me",
			authMiddleware.RequireAuth(),
			authHandler.Me,
		)
	}

	movieRoutes := api.Group("/movies")
	{
		movieRoutes.GET(
			"",
			movieHandler.ListPublic,
		)

		movieRoutes.GET(
			"/:movieID",
			movieHandler.GetPublic,
		)

		movieRoutes.GET(
			"/:movieID/showtimes",
			showtimeHandler.ListByMovie,
		)
	}

	showtimeRoutes := api.Group("/showtimes")
	{
		showtimeRoutes.GET(
			"/:showtimeID",
			showtimeHandler.GetPublic,
		)

		showtimeRoutes.GET(
			"/:showtimeID/seats",
			seatHandler.SeatMap,
		)
	}

	seatLockRoutes := api.Group(
		"/showtimes/:showtimeID/seats/:seatCode",
	)

	seatLockRoutes.Use(
		authMiddleware.RequireAuth(),
		authMiddleware.RequireCSRF(),
		rateLimiter.LimitMutations(
			"seat",
			mutationRateLimit,
		),
	)

	{
		seatLockRoutes.POST(
			"/lock",
			seatHandler.Lock,
		)

		seatLockRoutes.DELETE(
			"/lock",
			seatHandler.Release,
		)
	}

	bookingRoutes := api.Group("/bookings")

	bookingRoutes.Use(
		authMiddleware.RequireAuth(),
		authMiddleware.RequireCSRF(),
		rateLimiter.LimitMutations(
			"booking",
			mutationRateLimit,
		),
	)

	{
		bookingRoutes.POST(
			"/confirm",
			bookingHandler.Confirm,
		)
		bookingRoutes.POST(
			"/confirm-many",
			bookingHandler.ConfirmMany,
		)
		bookingRoutes.POST(
			"/payment-reminder",
			paymentNotificationHandler.SendPending,
		)

		bookingRoutes.GET(
			"",
			bookingHandler.ListMine,
		)

		bookingRoutes.GET(
			"/:bookingID",
			bookingHandler.GetMine,
		)
	}

	adminRoutes := api.Group("/admin")

	adminRoutes.Use(
		authMiddleware.RequireAuth(),
		authMiddleware.RequireCSRF(),
		rateLimiter.LimitMutations(
			"admin",
			mutationRateLimit,
		),
		authmiddleware.RequireRoles(
			models.RoleAdmin,
		),
	)

	{
		adminRoutes.GET(
			"/ping",
			adminHandler.Ping,
		)

		adminMovieRoutes := adminRoutes.Group("/movies")
		{
			adminMovieRoutes.GET(
				"",
				movieHandler.ListAdmin,
			)

			adminMovieRoutes.GET(
				"/:movieID",
				movieHandler.GetAdmin,
			)

			adminMovieRoutes.POST(
				"",
				movieHandler.Create,
			)

			adminMovieRoutes.PATCH(
				"/:movieID",
				movieHandler.Update,
			)

			adminMovieRoutes.DELETE(
				"/:movieID",
				movieHandler.Delete,
			)
		}

		adminRoutes.GET(
			"/tmdb/movies",
			tmdbHandler.SearchMovies,
		)
		adminRoutes.GET(
			"/tmdb/movies/:tmdbMovieID",
			tmdbHandler.GetMovie,
		)

		adminShowtimeRoutes := adminRoutes.Group("/showtimes")
		{
			adminShowtimeRoutes.POST(
				"",
				showtimeHandler.Create,
			)

			adminShowtimeRoutes.GET(
				"/:showtimeID",
				showtimeHandler.GetAdmin,
			)

			adminShowtimeRoutes.DELETE(
				"/:showtimeID",
				showtimeHandler.Cancel,
			)
		}

		adminRoutes.GET("/halls", showtimeHandler.ListHalls)
		adminRoutes.GET("/halls/availability", showtimeHandler.CheckHallAvailability)

		adminBookingRoutes := adminRoutes.Group("/bookings")
		{
			adminBookingRoutes.GET(
				"",
				adminBookingHandler.List,
			)

			adminBookingRoutes.GET(
				"/:bookingID",
				adminBookingHandler.Get,
			)
		}

		adminAuditLogRoutes := adminRoutes.Group("/audit-logs")
		{
			adminAuditLogRoutes.GET(
				"",
				auditLogHandler.List,
			)

			adminAuditLogRoutes.GET(
				"/:auditLogID",
				auditLogHandler.Get,
			)
		}
	}
}
