package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
)

func TestRequireRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminRole := models.RoleAdmin
	userRole := models.RoleUser

	tests := []struct {
		name       string
		role       *models.UserRole
		wantStatus int
	}{
		{
			name:       "admin can access admin route",
			role:       &adminRole,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "user cannot access admin route",
			role:       &userRole,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing role returns unauthorized",
			role:       nil,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()

			router.Use(func(c *gin.Context) {
				if test.role != nil {
					c.Set(
						ContextUserRoleKey,
						*test.role,
					)
				}

				c.Next()
			})

			router.GET(
				"/admin",
				RequireRoles(models.RoleAdmin),
				func(c *gin.Context) {
					c.Status(http.StatusNoContent)
				},
			)

			request := httptest.NewRequest(
				http.MethodGet,
				"/admin",
				nil,
			)

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