package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrInvalidAuditLogFilter = errors.New(
	"invalid audit log filter",
)

type AuditLogRepository interface {
	FindAll(
		ctx context.Context,
		filter repository.AuditLogFilter,
		skip int64,
		limit int64,
	) ([]models.AuditLog, int64, error)

	FindByID(
		ctx context.Context,
		auditLogID primitive.ObjectID,
	) (*models.AuditLog, error)
}

type AuditLogFilter struct {
	EventType string

	ActorUserID *primitive.ObjectID

	EntityType string
	EntityID   *primitive.ObjectID

	Action string

	From *time.Time
	To   *time.Time
}

type AuditLogListResult struct {
	AuditLogs []models.AuditLog

	Page       int
	Limit      int
	Total      int64
	TotalPages int
}

type AuditLogService struct {
	auditLogRepository AuditLogRepository
}

func NewAuditLogService(
	auditLogRepository AuditLogRepository,
) *AuditLogService {
	if auditLogRepository == nil {
		panic(
			"audit log service: repository is nil",
		)
	}

	return &AuditLogService{
		auditLogRepository: auditLogRepository,
	}
}

func (s *AuditLogService) ListAuditLogs(
	ctx context.Context,
	filter AuditLogFilter,
	page int,
	limit int,
) (*AuditLogListResult, error) {
	filter.EventType = strings.TrimSpace(
		filter.EventType,
	)

	filter.EntityType = strings.ToUpper(
		strings.TrimSpace(filter.EntityType),
	)

	filter.Action = strings.ToUpper(
		strings.TrimSpace(filter.Action),
	)

	if filter.From != nil &&
		filter.To != nil &&
		!filter.To.After(*filter.From) {
		return nil, fmt.Errorf(
			"%w: to must be after from",
			ErrInvalidAuditLogFilter,
		)
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	skip := int64((page - 1) * limit)

	auditLogs, total, err :=
		s.auditLogRepository.FindAll(
			ctx,
			repository.AuditLogFilter{
				EventType: filter.EventType,

				ActorUserID: filter.ActorUserID,

				EntityType: filter.EntityType,
				EntityID:   filter.EntityID,

				Action: filter.Action,

				From: filter.From,
				To:   filter.To,
			},
			skip,
			int64(limit),
		)
	if err != nil {
		return nil, fmt.Errorf(
			"list audit logs: %w",
			err,
		)
	}

	totalPages := 0

	if total > 0 {
		totalPages = int(
			(total + int64(limit) - 1) /
				int64(limit),
		)
	}

	return &AuditLogListResult{
		AuditLogs: auditLogs,

		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (s *AuditLogService) GetAuditLog(
	ctx context.Context,
	auditLogID primitive.ObjectID,
) (*models.AuditLog, error) {
	if auditLogID.IsZero() {
		return nil, ErrInvalidAuditLogFilter
	}

	return s.auditLogRepository.FindByID(
		ctx,
		auditLogID,
	)
}