package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditActorType string

const (
	AuditActorUser   AuditActorType = "USER"
	AuditActorSystem AuditActorType = "SYSTEM"
)

func (a AuditActorType) IsValid() bool {
	return a == AuditActorUser ||
		a == AuditActorSystem
}

type AuditSeverity string

const (
	AuditSeverityInfo    AuditSeverity = "INFO"
	AuditSeverityWarning AuditSeverity = "WARNING"
	AuditSeverityError   AuditSeverity = "ERROR"
)

func (s AuditSeverity) IsValid() bool {
	switch s {
	case AuditSeverityInfo,
		AuditSeverityWarning,
		AuditSeverityError:
		return true
	default:
		return false
	}
}

type AuditLog struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	EventID   string `bson:"event_id" json:"event_id"`
	EventType string `bson:"event_type" json:"event_type"`

	ActorType   AuditActorType      `bson:"actor_type" json:"actor_type"`
	ActorUserID *primitive.ObjectID `bson:"actor_user_id,omitempty" json:"actor_user_id,omitempty"`

	EntityType string              `bson:"entity_type" json:"entity_type"`
	EntityID   *primitive.ObjectID `bson:"entity_id,omitempty" json:"entity_id,omitempty"`

	Action   string        `bson:"action" json:"action"`
	Severity AuditSeverity `bson:"severity" json:"severity"`

	Metadata bson.M `bson:"metadata" json:"metadata"`

	OccurredAt time.Time `bson:"occurred_at" json:"occurred_at"`
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
}