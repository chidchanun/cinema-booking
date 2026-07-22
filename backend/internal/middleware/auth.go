package middleware

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	authpkg "cinema-booking/internal/auth"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AccessTokenCookieManager interface {
	ReadAccessToken(
		r *http.Request,
	) (string, error)

	ClearAccessToken(
		w http.ResponseWriter,
	)

	ReadCSRFToken(
		r *http.Request,
	) (string, error)
}

type AccessTokenParser interface {
	ParseAccessToken(
		rawToken string,
	) (*authpkg.AccessTokenClaims, error)
}

// RequireCSRF implements the double-submit cookie pattern. Authentication must
// run first so unauthenticated requests still receive a 401 response.
func (m *AuthMiddleware) RequireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		cookieToken, err := m.cookieService.ReadCSRFToken(c.Request)
		headerToken := strings.TrimSpace(
			c.GetHeader("X-CSRF-Token"),
		)

		if err != nil || headerToken == "" ||
			subtle.ConstantTimeCompare(
				[]byte(cookieToken),
				[]byte(headerToken),
			) != 1 {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"error":   "invalid_csrf_token",
					"message": "CSRF token is missing or invalid",
				},
			)
			return
		}

		c.Next()
	}
}

type AuthMiddleware struct {
	cookieService AccessTokenCookieManager
	jwtService    AccessTokenParser
}

func NewAuthMiddleware(
	cookieService AccessTokenCookieManager,
	jwtService AccessTokenParser,
) *AuthMiddleware {
	if cookieService == nil {
		panic("auth middleware: cookie service is nil")
	}

	if jwtService == nil {
		panic("auth middleware: JWT service is nil")
	}

	return &AuthMiddleware{
		cookieService: cookieService,
		jwtService:    jwtService,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken, err := m.cookieService.ReadAccessToken(
			c.Request,
		)

		if err != nil {
			m.handleCookieError(c, err)
			return
		}

		claims, err := m.jwtService.ParseAccessToken(
			rawToken,
		)

		if err != nil {
			// ลบ Cookie ที่หมดอายุหรือไม่ถูกต้อง
			m.cookieService.ClearAccessToken(c.Writer)

			m.handleTokenError(c, err)
			return
		}

		userID, err := primitive.ObjectIDFromHex(
			claims.UserID,
		)
		if err != nil {
			m.cookieService.ClearAccessToken(c.Writer)

			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error":   "invalid_session",
					"message": "Authentication session is invalid",
				},
			)
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Set(ContextUserRoleKey, claims.Role)

		c.Next()
	}
}

func (m *AuthMiddleware) handleCookieError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		authpkg.ErrAccessTokenCookieNotFound,
	):
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{
				"error":   "authentication_required",
				"message": "Authentication is required",
			},
		)

	default:
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{
				"error":   "invalid_session",
				"message": "Authentication session is invalid",
			},
		)
	}
}

func (m *AuthMiddleware) handleTokenError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		authpkg.ErrAccessTokenExpired,
	):
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{
				"error":   "session_expired",
				"message": "Authentication session has expired",
			},
		)

	default:
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{
				"error":   "invalid_session",
				"message": "Authentication session is invalid",
			},
		)
	}
}
