package realtime

import (
	"context"
	"errors"
	"time"

	"cinema-booking/internal/models"
)

var (
	// ErrRealtimeClosed หมายถึง Hub หยุดรับ event เพราะ application กำลังปิด
	ErrRealtimeClosed = errors.New("realtime hub is closed")
	// ErrRealtimeQueueFull หมายถึง event เข้ามาเร็วกว่าที่ Hub กระจายได้
	ErrRealtimeQueueFull = errors.New("realtime event queue is full")
)

// SeatEventType ระบุชนิด event เพื่อให้ frontend เลือกวิธีอัปเดต state
type SeatEventType string

const (
	// EventSeatStatusChanged แจ้งว่าที่นั่งเปลี่ยนเป็น AVAILABLE, LOCKED หรือ BOOKED
	EventSeatStatusChanged SeatEventType = "seat.status_changed"
)

// SeatEvent คือ payload JSON ที่ส่งให้ทุก client ใน showtime room
// ExpiresAt ใช้แสดงเวลาหมดอายุของ lock และ BookingID มีค่าเมื่อจองสำเร็จ
type SeatEvent struct {
	Type SeatEventType `json:"type"`

	ShowtimeID string            `json:"showtime_id"`
	SeatCode   string            `json:"seat_code"`
	Status     models.SeatStatus `json:"status"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	BookingID string     `json:"booking_id,omitempty"`

	OccurredAt time.Time `json:"occurred_at"`
}

// Publisher แยก service ออกจาก Hub implementation และช่วยให้ทดสอบด้วย fake publisher ได้
type Publisher interface {
	Publish(
		ctx context.Context,
		event SeatEvent,
	) error
}
