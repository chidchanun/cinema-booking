package middleware

import (
	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ContextUserIDKey   = "authenticated_user_id"
	ContextUserRoleKey = "authenticated_user_role"
)

func GetUserID(
	c *gin.Context,
) (primitive.ObjectID, bool) {
	if c == nil {
		return primitive.NilObjectID, false
	}

	value, exists := c.Get(ContextUserIDKey)
	if !exists {
		return primitive.NilObjectID, false
	}

	userID, ok := value.(primitive.ObjectID)
	if !ok || userID.IsZero() {
		return primitive.NilObjectID, false
	}

	return userID, true
}

func GetUserRole(
	c *gin.Context,
) (models.UserRole, bool) {
	if c == nil {
		return "", false
	}

	value, exists := c.Get(ContextUserRoleKey)
	if !exists {
		return "", false
	}

	role, ok := value.(models.UserRole)
	if !ok || !role.IsValid() {
		return "", false
	}

	return role, true
}
