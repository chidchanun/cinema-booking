package handlers

import (
	"net/http"

	authmiddleware "cinema-booking/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

func (h *AdminHandler) Ping(c *gin.Context) {
	userID, exists := authmiddleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "authentication_context_missing",
			"message": "Unable to read authenticated user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Admin access granted",
		"user_id": userID.Hex(),
	})
}
