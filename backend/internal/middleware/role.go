package middleware

import (
	"net/http"

	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
)

func RequireRoles(
	allowedRoles ...models.UserRole,
) gin.HandlerFunc {
	if len(allowedRoles) == 0 {
		panic("role middleware: allowed roles cannot be empty")
	}

	roleSet := make(map[models.UserRole]struct{}, len(allowedRoles))

	for _, role := range allowedRoles {
		if !role.IsValid() {
			panic("role middleware: invalid role")
		}

		roleSet[role] = struct{}{}
	}

	return func(c *gin.Context) {
		currentRole, exists := GetUserRole(c)
		if !exists {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error":   "authentication_required",
					"message": "Authentication is required",
				},
			)
			return
		}

		if _, allowed := roleSet[currentRole]; !allowed {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{
					"error":   "permission_denied",
					"message": "You do not have permission to access this resource",
				},
			)
			return
		}
		c.Next()
	}
}
