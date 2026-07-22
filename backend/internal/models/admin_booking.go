package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminBookingUser struct {
	ID    primitive.ObjectID `bson:"id" json:"id"`
	Name  string             `bson:"name" json:"name"`
	Email string             `bson:"email" json:"email"`
}

type AdminBookingMovie struct {
	ID    primitive.ObjectID `bson:"id" json:"id"`
	Title string             `bson:"title" json:"title"`
}

// AdminBookingView เป็น Read Model สำหรับหน้า Admin
// ไม่ได้ถูกบันทึกเป็น Collection แยกใน MongoDB
type AdminBookingView struct {
	ID primitive.ObjectID `bson:"_id" json:"id"`

	BookingCode string `bson:"booking_code" json:"booking_code"`

	User  AdminBookingUser  `bson:"user" json:"user"`
	Movie AdminBookingMovie `bson:"movie" json:"movie"`

	ShowtimeID primitive.ObjectID `bson:"showtime_id" json:"showtime_id"`

	SeatCode string `bson:"seat_code" json:"seat_code"`
	HallName string `bson:"hall_name" json:"hall_name"`

	ShowtimeStart time.Time `bson:"showtime_start" json:"showtime_start"`

	Price    int64  `bson:"price" json:"price"`
	Currency string `bson:"currency" json:"currency"`

	Status BookingStatus `bson:"status" json:"status"`

	ConfirmedAt time.Time `bson:"confirmed_at" json:"confirmed_at"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}