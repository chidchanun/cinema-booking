package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authpkg "cinema-booking/internal/auth"
	authmiddleware "cinema-booking/internal/middleware"
	"cinema-booking/internal/models"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type authHandlerTestService struct {
	result  *services.GoogleLoginResult
	err     error
	user    *models.User
	userErr error
}

func (s *authHandlerTestService) GetUser(
	context.Context,
	primitive.ObjectID,
) (*models.User, error) {
	return s.user, s.userErr
}

func (s *authHandlerTestService) LoginWithGoogle(
	context.Context,
	string,
) (*services.GoogleLoginResult, error) {
	return s.result, s.err
}

type authHandlerTestCookies struct {
	accessSet   bool
	accessClear bool
	csrfClear   bool
	csrfToken   string
	csrfErr     error
}

func (c *authHandlerTestCookies) SetAccessToken(
	http.ResponseWriter,
	*authpkg.AccessToken,
) error {
	c.accessSet = true
	return nil
}

func (c *authHandlerTestCookies) ClearAccessToken(http.ResponseWriter) {
	c.accessClear = true
}

func (c *authHandlerTestCookies) SetCSRFToken(http.ResponseWriter) (string, error) {
	return c.csrfToken, c.csrfErr
}

func (c *authHandlerTestCookies) ClearCSRFToken(http.ResponseWriter) {
	c.csrfClear = true
}

func TestGoogleLoginReturnsCSRFToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	cookies := &authHandlerTestCookies{csrfToken: "csrf-token"}
	handler := NewAuthHandler(
		&authHandlerTestService{result: &services.GoogleLoginResult{
			User: &models.User{
				ID:    primitive.NewObjectID(),
				Email: "user@example.com",
				Name:  "Test User",
				Role:  models.RoleUser,
			},
			AccessToken: &authpkg.AccessToken{
				Token:     "access-token",
				ExpiresAt: now.Add(time.Hour),
				ExpiresIn: 3600,
			},
		}},
		cookies,
	)

	router := gin.New()
	router.POST("/login", handler.GoogleLogin)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(`{"id_token":"google-token"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	var body GoogleLoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if body.CSRFToken != "csrf-token" {
		t.Fatalf("expected CSRF token, got %q", body.CSRFToken)
	}
	if !cookies.accessSet {
		t.Fatal("expected access token cookie to be set")
	}
}

func TestGoogleLoginRejectsInvalidPayload(t *testing.T) {
	handler := NewAuthHandler(
		&authHandlerTestService{},
		&authHandlerTestCookies{},
	)
	router := gin.New()
	router.POST("/login", handler.GoogleLogin)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestGoogleLoginClearsPartialSessionWhenCSRFCreationFails(t *testing.T) {
	cookies := &authHandlerTestCookies{
		csrfErr: errors.New("random source unavailable"),
	}
	handler := NewAuthHandler(
		&authHandlerTestService{result: &services.GoogleLoginResult{
			User: &models.User{ID: primitive.NewObjectID(), Role: models.RoleUser},
			AccessToken: &authpkg.AccessToken{
				Token: "access-token", ExpiresAt: time.Now().Add(time.Hour), ExpiresIn: 3600,
			},
		}},
		cookies,
	)
	router := gin.New()
	router.POST("/login", handler.GoogleLogin)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(`{"id_token":"google-token"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", response.Code)
	}
	if !cookies.accessClear {
		t.Fatal("expected partial access-token session to be cleared")
	}
}

func TestLogoutClearsAuthenticationCookies(t *testing.T) {
	cookies := &authHandlerTestCookies{}
	handler := NewAuthHandler(&authHandlerTestService{}, cookies)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	handler.Logout(context)

	if !cookies.accessClear || !cookies.csrfClear {
		t.Fatal("expected access-token and CSRF cookies to be cleared")
	}
}

func TestMeReturnsAuthenticatedUserProfile(t *testing.T) {
	userID := primitive.NewObjectID()
	handler := NewAuthHandler(
		&authHandlerTestService{user: &models.User{
			ID:      userID,
			Email:   "user@example.com",
			Name:    "Test User",
			Picture: "https://example.com/profile.jpg",
			Role:    models.RoleUser,
		}},
		&authHandlerTestCookies{},
	)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/me", nil)
	context.Set(authmiddleware.ContextUserIDKey, userID)
	context.Set(authmiddleware.ContextUserRoleKey, models.RoleUser)
	handler.Me(context)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body struct {
		User models.UserResponse `json:"user"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if body.User.ID != userID || body.User.Name != "Test User" {
		t.Fatalf("unexpected user profile: %+v", body.User)
	}
}
