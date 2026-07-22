package redislock

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrSeatAlreadyLocked   = errors.New("seat is already locked")
	ErrSeatLockNotFound    = errors.New("seat lock not found")
	ErrSeatLockNotOwned    = errors.New("seat lock is owned by another user")
	ErrInvalidSeatLock     = errors.New("invalid seat lock")
	ErrSeatLockUnavailable = errors.New("seat lock service unavailable")
)

type AcquireResult struct {
	LockID       string
	ExpiresAt    time.Time
	ExpiresIn    int64
	AlreadyOwned bool
}

type SeatLockManager struct {
	client redis.UniversalClient
	ttl    time.Duration
}

var releaseLockScript = redis.NewScript(`
	local currentValue = redis.call("GET", KEYS[1])

	if not currentValue then
		return 0
	end

	if currentValue ~= ARGV[1] then
		return -1
	end

	return redis.call("DEL", KEYS[1])
`)

var validateLockScript = redis.NewScript(`
	local currentValue = redis.call("GET", KEYS[1])

	if not currentValue then
		return 0
	end

	if currentValue ~= ARGV[1] then
		return -1
	end

	local remainingTTL = redis.call("PTTL", KEYS[1])

	if remainingTTL <= 0 then
		return 0
	end

	return remainingTTL
`)

func NewSeatLockManager(
	client redis.UniversalClient,
	ttl time.Duration,
) (*SeatLockManager, error) {
	if client == nil {
		return nil, fmt.Errorf(
			"%w: Redis client is nil",
			ErrInvalidSeatLock,
		)
	}

	if ttl < 0 {
		return nil, fmt.Errorf(
			"%w: TTL must be greater than zero",
			ErrInvalidSeatLock,
		)
	}

	return &SeatLockManager{
		client: client,
		ttl:    ttl,
	}, nil
}

func (m *SeatLockManager) Acquire(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
) (*AcquireResult, error) {
	if showtimeID.IsZero() {
		return nil, fmt.Errorf(
			"%w: showtime ID is required",
			ErrInvalidSeatLock,
		)
	}

	if userID.IsZero() {
		return nil, fmt.Errorf(
			"%w: user ID is required",
			ErrInvalidSeatLock,
		)
	}

	seatCode = normalizeSeatCode(seatCode)
	if seatCode == "" {
		return nil, fmt.Errorf(
			"%w: seat code is required",
			ErrInvalidSeatLock,
		)
	}

	key := buildSeatLockKey(
		showtimeID,
		seatCode,
	)

	/*
		ลองสูงสุด 2 ครั้ง เผื่อ Lock หมดอายุพอดีระหว่าง
		SET NX ไม่สำเร็จกับ GET ค่าเดิม
	*/

	for attempt := 0; attempt < 2; attempt++ {
		lockID := primitive.NewObjectID().Hex()

		lockValue := encodeLockValue(
			userID,
			lockID,
		)

		acquired, err := m.client.SetNX(
			ctx,
			key,
			lockValue,
			m.ttl,
		).Result()

		if err != nil {
			return nil, fmt.Errorf(
				"%w: acquire seat lock: %v",
				ErrSeatLockUnavailable,
				err,
			)

		}

		if acquired {
			return newAcquireResult(
				lockID,
				m.ttl,
				false,
			), nil
		}

		currentValue, err := m.client.Get(
			ctx,
			key,
		).Result()

		if errors.Is(err, redis.Nil) {
			// Lock หมดอายุพอดี ลองสร้างใหม่อีกครั้ง
			continue
		}

		if err != nil {
			return nil, fmt.Errorf(
				"%w: read existing seat lock: %v",
				ErrSeatLockUnavailable,
				err,
			)
		}

		ownerUserID, currentLockID, err :=
			decodeLockValue(currentValue)

		if err != nil {
			return nil, err
		}

		/*
			รองรับ Double Click จากผู้ใช้คนเดิมแบบ idempotent

			ไม่สร้าง Lock ใหม่ แต่คืน Lock เดิมพร้อม TTL ที่เหลือ
		*/

		if ownerUserID == userID.Hex() {
			remainingTTL, err := m.client.TTL(
				ctx,
				key,
			).Result()
			if err != nil {
				return nil, fmt.Errorf(
					"%w: read seat lock TTL: %v",
					ErrSeatLockUnavailable,
					err,
				)
			}

			if remainingTTL < 0 {
				continue
			}

			return newAcquireResult(
				currentLockID,
				remainingTTL,
				true,
			), nil
		}
		return nil, ErrSeatAlreadyLocked
	}

	return nil, ErrSeatAlreadyLocked
}

func (m *SeatLockManager) Release(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) error {
	if showtimeID.IsZero() || userID.IsZero() {
		return ErrInvalidSeatLock
	}

	seatCode = normalizeSeatCode(seatCode)
	lockID = strings.TrimSpace(lockID)

	if seatCode == "" || lockID == "" {
		return ErrInvalidSeatLock
	}

	key := buildSeatLockKey(
		showtimeID,
		seatCode,
	)

	expectedValue := encodeLockValue(
		userID,
		lockID,
	)

	result, err := releaseLockScript.Run(
		ctx,
		m.client,
		[]string{key},
		expectedValue,
	).Int()

	if err != nil {
		return fmt.Errorf(
			"%w: release seat lock: %v",
			ErrSeatLockUnavailable,
			err,
		)
	}

	switch result {
	case 1:
		return nil

	case 0:
		return ErrSeatLockNotFound

	case -1:
		return ErrSeatLockNotOwned

	default:
		return fmt.Errorf(
			"%w: unexpected release result",
			ErrSeatLockUnavailable,
		)
	}

}

func (m *SeatLockManager) GetLockedSeatCodes(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCodes []string,
) (map[string]bool, error) {
	if showtimeID.IsZero() {
		return nil, ErrInvalidSeatLock
	}

	lockedSeats := make(map[string]bool)

	if len(seatCodes) == 0 {
		return lockedSeats, nil
	}

	normalizedCodes := make(
		[]string,
		0,
		len(seatCodes),
	)

	keys := make(
		[]string,
		0,
		len(seatCodes),
	)

	for _, rawCode := range seatCodes {
		seatCode := normalizeSeatCode(rawCode)

		if seatCode == "" {
			continue
		}

		normalizedCodes = append(
			normalizedCodes,
			seatCode,
		)

		keys = append(
			keys,
			buildSeatLockKey(
				showtimeID,
				seatCode,
			),
		)
	}

	if len(keys) == 0 {
		return lockedSeats, nil
	}

	values, err := m.client.MGet(
		ctx,
		keys...,
	).Result()
	if err != nil {
		return nil, fmt.Errorf(
			"%w: read seat locks: %v",
			ErrSeatLockUnavailable,
			err,
		)
	}

	for index, value := range values {
		if value == nil {
			continue
		}

		lockedSeats[normalizedCodes[index]] = true
	}

	return lockedSeats, nil

}

func (m *SeatLockManager) ValidateOwnership(
	ctx context.Context,
	showtimeID primitive.ObjectID,
	seatCode string,
	userID primitive.ObjectID,
	lockID string,
) (time.Duration, error) {
	if showtimeID.IsZero() || userID.IsZero() {
		return 0, ErrInvalidSeatLock
	}

	seatCode = normalizeSeatCode(seatCode)
	lockID = strings.TrimSpace(lockID)

	if seatCode == "" || lockID == "" {
		return 0, ErrInvalidSeatLock
	}

	key := buildSeatLockKey(
		showtimeID,
		seatCode,
	)

	expectedValue := encodeLockValue(
		userID,
		lockID,
	)

	result, err := validateLockScript.Run(
		ctx,
		m.client,
		[]string{key},
		expectedValue,
	).Int64()

	if err != nil {
		return 0, fmt.Errorf(
			"%w: validate seat lock: %v",
			ErrSeatLockUnavailable,
			err,
		)
	}

	switch {
	case result > 0:
		return time.Duration(result) * time.Millisecond, nil

	case result == 0:
		return 0, ErrSeatLockNotFound

	case result == -1:
		return 0, ErrSeatLockNotOwned

	default:
		return 0, ErrSeatLockUnavailable
	}
}

func buildSeatLockKey(
	showtimeID primitive.ObjectID,
	seatCode string,
) string {
	/*
		{showtimeID} เป็น Redis Cluster hash tag

		ทำให้ Lock ของรอบฉายเดียวกันอยู่ใน hash slot เดียวกัน
	*/
	return fmt.Sprintf(
		"cinema:seat-lock:{%s}:%s",
		showtimeID.Hex(),
		normalizeSeatCode(seatCode),
	)
}

func encodeLockValue(
	userID primitive.ObjectID,
	lockID string,
) string {
	return userID.Hex() + "|" + lockID
}

func decodeLockValue(
	value string,
) (string, string, error) {
	parts := strings.SplitN(
		value,
		"|",
		2,
	)

	if len(parts) != 2 {
		return "", "", ErrInvalidSeatLock
	}

	userID := strings.TrimSpace(parts[0])
	lockID := strings.TrimSpace(parts[1])

	if userID == "" || lockID == "" {
		return "", "", ErrInvalidSeatLock
	}

	return userID, lockID, nil
}

func normalizeSeatCode(
	seatCode string,
) string {
	return strings.ToUpper(
		strings.TrimSpace(seatCode),
	)
}

func newAcquireResult(
	lockID string,
	remainingTTL time.Duration,
	alreadyOwned bool,
) *AcquireResult {
	expiresIn := int64(
		math.Ceil(remainingTTL.Seconds()),
	)

	return &AcquireResult{
		LockID:       lockID,
		ExpiresAt:    time.Now().UTC().Add(remainingTTL),
		ExpiresIn:    expiresIn,
		AlreadyOwned: alreadyOwned,
	}
}
