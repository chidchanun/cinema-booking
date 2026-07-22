package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrInvalidAuditLog       = errors.New("invalid audit log")
	ErrAuditLogAlreadyExists = errors.New("audit log already exists")
	ErrAuditLogNotFound      = errors.New("audit log not found")
)

type AuditLogFilter struct {
	EventType string

	ActorUserID *primitive.ObjectID

	EntityType string
	EntityID   *primitive.ObjectID

	Action string

	From *time.Time
	To   *time.Time
}

const auditLogsCollection = "audit_logs"

type AuditLogRepository struct {
	collection *mongo.Collection
}

func NewAuditLogRepository(
	db *mongo.Database,
) *AuditLogRepository {
	if db == nil {
		panic(
			"audit log repository: MongoDB database is nil",
		)
	}

	return &AuditLogRepository{
		collection: db.Collection(
			auditLogsCollection,
		),
	}
}

func (r *AuditLogRepository) Create(
	ctx context.Context,
	auditLog *models.AuditLog,
) error {
	if auditLog == nil ||
		strings.TrimSpace(auditLog.EventID) == "" ||
		strings.TrimSpace(auditLog.EventType) == "" ||
		strings.TrimSpace(auditLog.EntityType) == "" ||
		strings.TrimSpace(auditLog.Action) == "" {
		return ErrInvalidAuditLog
	}

	if !auditLog.ActorType.IsValid() {
		return ErrInvalidAuditLog
	}

	if auditLog.ActorType == models.AuditActorUser {
		if auditLog.ActorUserID == nil ||
			auditLog.ActorUserID.IsZero() {
			return ErrInvalidAuditLog
		}
	}

	if !auditLog.Severity.IsValid() {
		return ErrInvalidAuditLog
	}

	if auditLog.ID.IsZero() {
		auditLog.ID = primitive.NewObjectID()
	}

	if auditLog.OccurredAt.IsZero() {
		auditLog.OccurredAt = time.Now().UTC()
	}

	if auditLog.Metadata == nil {
		auditLog.Metadata = bson.M{}
	}

	auditLog.CreatedAt = time.Now().UTC()

	_, err := r.collection.InsertOne(
		ctx,
		auditLog,
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrAuditLogAlreadyExists
		}

		return fmt.Errorf(
			"create audit log: %w",
			err,
		)
	}

	return nil
}

func (r *AuditLogRepository) FindAll(
	ctx context.Context,
	filter AuditLogFilter,
	skip int64,
	limit int64,
) ([]models.AuditLog, int64, error) {
	if skip < 0 || limit < 1 {
		return nil, 0, ErrInvalidAuditLog
	}

	mongoFilter := buildAuditLogFilter(filter)

	total, err := r.collection.CountDocuments(
		ctx,
		mongoFilter,
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"count audit logs: %w",
			err,
		)
	}

	cursor, err := r.collection.Find(
		ctx,
		mongoFilter,
		options.Find().
			SetSort(bson.D{
				{Key: "occurred_at", Value: -1},
				{Key: "_id", Value: -1},
			}).
			SetSkip(skip).
			SetLimit(limit),
	)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"find audit logs: %w",
			err,
		)
	}
	defer cursor.Close(ctx)

	auditLogs := make(
		[]models.AuditLog,
		0,
	)

	if err := cursor.All(ctx, &auditLogs); err != nil {
		return nil, 0, fmt.Errorf(
			"decode audit logs: %w",
			err,
		)
	}

	return auditLogs, total, nil
}

func (r *AuditLogRepository) FindByID(
	ctx context.Context,
	auditLogID primitive.ObjectID,
) (*models.AuditLog, error) {
	if auditLogID.IsZero() {
		return nil, ErrInvalidAuditLog
	}

	var auditLog models.AuditLog

	err := r.collection.FindOne(
		ctx,
		bson.M{
			"_id": auditLogID,
		},
	).Decode(&auditLog)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrAuditLogNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"find audit log by id: %w",
			err,
		)
	}

	return &auditLog, nil
}

func buildAuditLogFilter(
	filter AuditLogFilter,
) bson.M {
	result := bson.M{}

	if filter.EventType != "" {
		result["event_type"] = filter.EventType
	}

	if filter.ActorUserID != nil {
		result["actor_user_id"] = *filter.ActorUserID
	}

	if filter.EntityType != "" {
		result["entity_type"] = filter.EntityType
	}

	if filter.EntityID != nil {
		result["entity_id"] = *filter.EntityID
	}

	if filter.Action != "" {
		result["action"] = filter.Action
	}

	if filter.From != nil || filter.To != nil {
		occurredAtFilter := bson.M{}

		if filter.From != nil {
			occurredAtFilter["$gte"] =
				filter.From.UTC()
		}

		if filter.To != nil {
			occurredAtFilter["$lt"] =
				filter.To.UTC()
		}

		result["occurred_at"] = occurredAtFilter
	}

	return result
}
