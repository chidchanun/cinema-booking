package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	authpkg "cinema-booking/internal/auth"
	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthService interface {
	LoginWithGoogle(
		ctx context.Context,
		idToken string,
	) (*services.GoogleLoginResult, error)
	GetUser(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.User, error)
}

type AccessTokenCookieService interface {
	SetAccessToken(
		writer http.ResponseWriter,
		accessToken *authpkg.AccessToken,
	) error

	ClearAccessToken(
		writer http.ResponseWriter,
	)

	SetCSRFToken(
		writer http.ResponseWriter,
	) (string, error)

	ClearCSRFToken(
		writer http.ResponseWriter,
	)
}

type AuthHandler struct {
	authService   AuthService
	cookieService AccessTokenCookieService
}

type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type GoogleLoginResponse struct {
	Message   string `json:"message"`
	CSRFToken string `json:"csrf_token"`

	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int64     `json:"expires_in"`

	User models.UserResponse `json:"user"`
}

func NewAuthHandler(
	authService AuthService,
	cookieService AccessTokenCookieService,
) *AuthHandler {
	if authService == nil {
		panic("auth handler: auth service is nil")
	}

	if cookieService == nil {
		panic("auth handler: cookie service is nil")
	}

	return &AuthHandler{
		authService:   authService,
		cookieService: cookieService,
	}
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var request GoogleLoginRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "id_token is required",
		})
		return
	}

	result, err := h.authService.LoginWithGoogle(
		c.Request.Context(),
		request.IDToken,
	)
	if err != nil {
		h.handleGoogleLoginError(c, err)
		return
	}

	if err := h.cookieService.SetAccessToken(
		c.Writer,
		result.AccessToken,
	); err != nil {
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "cookie_creation_failed",
			"message": "Unable to create authentication session",
		})
		return
	}

	csrfToken, err := h.cookieService.SetCSRFToken(c.Writer)
	if err != nil {
		h.cookieService.ClearAccessToken(c.Writer)
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "cookie_creation_failed",
			"message": "Unable to create authentication session",
		})
		return
	}

	c.JSON(http.StatusOK, GoogleLoginResponse{
		Message:   "Google authentication successful",
		CSRFToken: csrfToken,

		ExpiresAt: result.AccessToken.ExpiresAt,
		ExpiresIn: result.AccessToken.ExpiresIn,

		User: models.NewUserResponse(result.User),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.cookieService.ClearAccessToken(
		c.Writer,
	)
	h.cookieService.ClearCSRFToken(c.Writer)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

func (h *AuthHandler) handleGoogleLoginError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, services.ErrIDTokenRequired):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "id_token_required",
			"message": "Google ID token is required",
		})

	case errors.Is(err, authpkg.ErrInvalidGoogleIDToken),
		errors.Is(err, authpkg.ErrInvalidGoogleClaims):
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_google_token",
			"message": "Google ID token is invalid or expired",
		})

	case errors.Is(err, repository.ErrUserIdentityConflict):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "user_identity_conflict",
			"message": "The email is already associated with another account",
		})

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to authenticate user",
		})
	}
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, userIDExists := authmiddleware.GetUserID(c)

	if !userIDExists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authentication session",
		})
		return
	}

	user, err := h.authService.GetUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_session",
				"message": "The authenticated user no longer exists",
			})
			return
		}

		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to retrieve user profile",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": user.ID.Hex(),
		"role":    user.Role,
		"user":    models.NewUserResponse(user),
	})
}
