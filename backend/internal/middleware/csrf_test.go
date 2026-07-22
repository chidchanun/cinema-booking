package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "cinema-booking/internal/auth"

	"github.com/gin-gonic/gin"
)

type csrfTestCookieManager struct {
	token string
	err   error
}

func (m *csrfTestCookieManager) ReadAccessToken(*http.Request) (string, error) {
	return "", errors.New("not implemented")
}

func (m *csrfTestCookieManager) ClearAccessToken(http.ResponseWriter) {}

func (m *csrfTestCookieManager) ReadCSRFToken(*http.Request) (string, error) {
	return m.token, m.err
}

type csrfTestTokenParser struct{}

func (*csrfTestTokenParser) ParseAccessToken(string) (*authpkg.AccessTokenClaims, error) {
	return nil, errors.New("not implemented")
}

func TestRequireCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		method      string
		cookieToken string
		headerToken string
		cookieErr   error
		wantStatus  int
	}{
		{
			name:       "safe method does not require a token",
			method:     http.MethodGet,
			wantStatus: http.StatusNoContent,
		},
		{
			name:        "matching token is accepted",
			method:      http.MethodPost,
			cookieToken: "csrf-token",
			headerToken: "csrf-token",
			wantStatus:  http.StatusNoContent,
		},
		{
			name:        "missing header is rejected",
			method:      http.MethodPost,
			cookieToken: "csrf-token",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "mismatched token is rejected",
			method:      http.MethodDelete,
			cookieToken: "csrf-token",
			headerToken: "different-token",
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "missing cookie is rejected",
			method:      http.MethodPatch,
			headerToken: "csrf-token",
			cookieErr:   authpkg.ErrCSRFCookieNotFound,
			wantStatus:  http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cookieManager := &csrfTestCookieManager{
				token: test.cookieToken,
				err:   test.cookieErr,
			}
			middleware := NewAuthMiddleware(
				cookieManager,
				&csrfTestTokenParser{},
			)

			router := gin.New()
			router.Handle(
				test.method,
				"/resource",
				middleware.RequireCSRF(),
				func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			)

			request := httptest.NewRequest(
				test.method,
				"/resource",
				nil,
			)
			if test.headerToken != "" {
				request.Header.Set("X-CSRF-Token", test.headerToken)
			}

			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d: %s",
					test.wantStatus,
					response.Code,
					response.Body.String(),
				)
			}
		})
	}
}
