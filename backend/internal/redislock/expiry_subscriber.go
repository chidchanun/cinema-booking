package redislock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const seatLockKeyPrefix = "cinema:seat-lock:{"

type ExpiredSeatLockHandler func(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
) error

type ExpirySubscriber struct {
	client *redis.Client
	db     int
}

func NewExpirySubscriber(
	client *redis.Client,
	db int,
) *ExpirySubscriber {
	if client == nil {
		panic(
			"Redis expiry subscriber: client is nil",
		)
	}

	return &ExpirySubscriber{
		client: client,
		db:     db,
	}
}

func (s *ExpirySubscriber) Run(
	ctx context.Context,
	handler ExpiredSeatLockHandler,
) error {
	if handler == nil {
		return fmt.Errorf(
			"expired seat lock handler is nil",
		)
	}

	channelName := fmt.Sprintf(
		"__keyevent@%d__:expired",
		s.db,
	)

	// Redis ต้องเปิด notify-keyspace-events=Ex จึงจะ publish key ที่หมดอายุ
	// ช่องทางนี้ใช้แจ้ง WebSocket เท่านั้น ส่วน TTL ใน Redis ยังเป็น source of truth
	pubSub := s.client.Subscribe(
		ctx,
		channelName,
	)
	defer pubSub.Close()

	if _, err := pubSub.Receive(ctx); err != nil {
		return fmt.Errorf(
			"subscribe Redis expiry events: %w",
			err,
		)
	}

	messages := pubSub.Channel()

	for {
		select {
		case <-ctx.Done():
			return nil

		case message, open := <-messages:
			if !open {
				return nil
			}

			showtimeID, seatCode, ok :=
				parseExpiredSeatLockKey(
					message.Payload,
				)
			if !ok {
				continue
			}

			// จำกัดเวลา callback เพื่อไม่ให้ event ที่มีปัญหาขวางการอ่าน expiry
			// event ถัดไปจาก subscription เป็นเวลานานเกินไป
			eventCtx, cancel :=
				context.WithTimeout(
					context.Background(),
					3*time.Second,
				)

			err := handler(
				eventCtx,
				showtimeID,
				seatCode,
			)

			cancel()

			if err != nil {
				continue
			}
		}
	}
}

func parseExpiredSeatLockKey(
	key string,
) (primitive.ObjectID, string, bool) {
	if !strings.HasPrefix(
		key,
		seatLockKeyPrefix,
	) {
		return primitive.NilObjectID, "", false
	}

	remainder := strings.TrimPrefix(
		key,
		seatLockKeyPrefix,
	)

	parts := strings.SplitN(
		remainder,
		"}:",
		2,
	)

	if len(parts) != 2 {
		return primitive.NilObjectID, "", false
	}

	showtimeID, err :=
		primitive.ObjectIDFromHex(parts[0])
	if err != nil {
		return primitive.NilObjectID, "", false
	}

	seatCode := normalizeSeatCode(parts[1])
	if seatCode == "" {
		return primitive.NilObjectID, "", false
	}

	return showtimeID, seatCode, true
}
