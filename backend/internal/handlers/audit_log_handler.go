package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"
	"cinema-booking/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditLogService interface {
	ListAuditLogs(
		ctx context.Context,
		filter services.AuditLogFilter,
		page int,
		limit int,
	) (*services.AuditLogListResult, error)

	GetAuditLog(
		ctx context.Context,
		auditLogID primitive.ObjectID,
	) (*models.AuditLog, error)
}

type AuditLogHandler struct {
	auditLogService AuditLogService
}

type AuditLogListResponse struct {
	Data []models.AuditLog `json:"data"`

	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewAuditLogHandler(
	auditLogService AuditLogService,
) *AuditLogHandler {
	if auditLogService == nil {
		panic("audit log handler: service is nil")
	}

	return &AuditLogHandler{
		auditLogService: auditLogService,
	}
}

func (h *AuditLogHandler) List(
	c *gin.Context,
) {
	actorUserID, err := parseOptionalObjectID(
		c.Query("actor_user_id"),
		"actor_user_id",
	)
	if err != nil {
		h.invalidFilter(c, err)
		return
	}

	entityID, err := parseOptionalObjectID(
		c.Query("entity_id"),
		"entity_id",
	)
	if err != nil {
		h.invalidFilter(c, err)
		return
	}

	from, err := parseOptionalRFC3339(
		c.Query("from"),
		"from",
	)
	if err != nil {
		h.invalidFilter(c, err)
		return
	}

	to, err := parseOptionalRFC3339(
		c.Query("to"),
		"to",
	)
	if err != nil {
		h.invalidFilter(c, err)
		return
	}

	result, err := h.auditLogService.ListAuditLogs(
		c.Request.Context(),
		services.AuditLogFilter{
			EventType: strings.TrimSpace(
				c.Query("event_type"),
			),

			ActorUserID: actorUserID,

			EntityType: strings.TrimSpace(
				c.Query("entity_type"),
			),

			EntityID: entityID,

			Action: strings.TrimSpace(
				c.Query("action"),
			),

			From: from,
			To:   to,
		},
		parsePositiveInt(c.Query("page"), 1),
		parsePositiveInt(c.Query("limit"), 20),
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, AuditLogListResponse{
		Data: result.AuditLogs,

		Page:       result.Page,
		Limit:      result.Limit,
		Total:      result.Total,
		TotalPages: result.TotalPages,
	})
}

func (h *AuditLogHandler) Get(
	c *gin.Context,
) {
	auditLogID, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(
			c.Param("auditLogID"),
		),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_audit_log_id",
			"message": "Audit log ID is invalid",
		})
		return
	}

	auditLog, err :=
		h.auditLogService.GetAuditLog(
			c.Request.Context(),
			auditLogID,
		)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, auditLog)
}

func (h *AuditLogHandler) invalidFilter(
	c *gin.Context,
	err error,
) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":   "invalid_audit_log_filter",
		"message": err.Error(),
	})
}

func (h *AuditLogHandler) handleError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		repository.ErrAuditLogNotFound,
	):
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "audit_log_not_found",
			"message": "Audit log not found",
		})

	case errors.Is(
		err,
		services.ErrInvalidAuditLogFilter,
	),
		errors.Is(
		err,
		repository.ErrInvalidAuditLog,
	):
		h.invalidFilter(c, err)

	default:
		_ = c.Error(err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_server_error",
			"message": "Unable to retrieve audit logs",
		})
	}
}