package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "cinema-booking/internal/auth"
	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type authTestCookieManager struct {
	token       string
	err         error
	clearCalled bool
}

func (m *authTestCookieManager) ReadAccessToken(*http.Request) (string, error) {
	return m.token, m.err
}

func (m *authTestCookieManager) ClearAccessToken(http.ResponseWriter) {
	m.clearCalled = true
}

func (*authTestCookieManager) ReadCSRFToken(*http.Request) (string, error) {
	return "", authpkg.ErrCSRFCookieNotFound
}

type authTestTokenParser struct {
	claims *authpkg.AccessTokenClaims
	err    error
}

func (p *authTestTokenParser) ParseAccessToken(string) (*authpkg.AccessTokenClaims, error) {
	return p.claims, p.err
}

func TestRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := primitive.NewObjectID()
	tests := []struct {
		name            string
		cookieError     error
		claims          *authpkg.AccessTokenClaims
		parserError     error
		wantStatus      int
		wantCookieClear bool
	}{
		{
			name: "valid session populates request context",
			claims: &authpkg.AccessTokenClaims{
				UserID: userID.Hex(),
				Role:   models.RoleUser,
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:        "missing cookie returns unauthorized",
			cookieError: authpkg.ErrAccessTokenCookieNotFound,
			wantStatus:  http.StatusUnauthorized,
		},
		{
			name:            "expired token clears cookie",
			parserError:     authpkg.ErrAccessTokenExpired,
			wantStatus:      http.StatusUnauthorized,
			wantCookieClear: true,
		},
		{
			name: "invalid user ID clears cookie",
			claims: &authpkg.AccessTokenClaims{
				UserID: "invalid",
				Role:   models.RoleUser,
			},
			wantStatus:      http.StatusUnauthorized,
			wantCookieClear: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cookieManager := &authTestCookieManager{
				token: "access-token",
				err:   test.cookieError,
			}
			parser := &authTestTokenParser{
				claims: test.claims,
				err:    test.parserError,
			}
			middleware := NewAuthMiddleware(cookieManager, parser)

			router := gin.New()
			router.GET(
				"/protected",
				middleware.RequireAuth(),
				func(c *gin.Context) {
					contextUserID, ok := GetUserID(c)
					if !ok || contextUserID != userID {
						c.AbortWithStatus(http.StatusInternalServerError)
						return
					}
					c.Status(http.StatusNoContent)
				},
			)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d: %s",
					test.wantStatus,
					response.Code,
					response.Body.String(),
				)
			}
			if cookieManager.clearCalled != test.wantCookieClear {
				t.Fatalf(
					"expected clearCalled=%t, got %t",
					test.wantCookieClear,
					cookieManager.clearCalled,
				)
			}
		})
	}
}

func TestRequireAuthRejectsUnexpectedParserError(t *testing.T) {
	cookieManager := &authTestCookieManager{token: "access-token"}
	middleware := NewAuthMiddleware(
		cookieManager,
		&authTestTokenParser{err: errors.New("signature invalid")},
	)

	router := gin.New()
	router.GET("/protected", middleware.RequireAuth())

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/protected", nil),
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
	if !cookieManager.clearCalled {
		t.Fatal("expected invalid session cookie to be cleared")
	}
}
