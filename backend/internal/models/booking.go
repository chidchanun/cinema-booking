package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BookingStatus string

const (
	BookingStatusBooked    BookingStatus = "BOOKED"
	BookingStatusCancelled BookingStatus = "CANCELLED"
)

func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusCancelled, BookingStatusBooked:
		return true
	default:
		return false
	}
}

type Booking struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	BookingCode string `bson:"booking_code" json:"booking_code"`

	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	MovieID    primitive.ObjectID `bson:"movie_id" json:"movie_id"`
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

type BookingResponse struct {
	ID primitive.ObjectID `json:"id"`

	BookingCode string `json:"booking_code"`

	MovieID    primitive.ObjectID `json:"movie_id"`
	ShowtimeID primitive.ObjectID `json:"showtime_id"`

	SeatCode string `json:"seat_code"`
	HallName string `json:"hall_name"`

	ShowtimeStart time.Time `json:"showtime_start"`

	Price    int64  `json:"price"`
	Currency string `json:"currency"`

	Status BookingStatus `json:"status"`

	ConfirmedAt time.Time `json:"confirmed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewBookingResponse(
	booking *Booking,
) BookingResponse {
	if booking == nil {
		return BookingResponse{}
	}

	return BookingResponse{
		ID: booking.ID,

		BookingCode: booking.BookingCode,

		MovieID:    booking.MovieID,
		ShowtimeID: booking.ShowtimeID,

		SeatCode: booking.SeatCode,
		HallName: booking.HallName,

		ShowtimeStart: booking.ShowtimeStart,

		Price:    booking.Price,
		Currency: booking.Currency,

		Status: booking.Status,

		ConfirmedAt: booking.ConfirmedAt,
		CreatedAt:   booking.CreatedAt,
	}
}

func NewBookingResponses(
	bookings []Booking,
) []BookingResponse {
	responses := make(
		[]BookingResponse,
		0,
		len(bookings),
	)

	for index := range bookings {
		responses = append(
			responses,
			NewBookingResponse(&bookings[index]),
		)
	}

	return responses
}
