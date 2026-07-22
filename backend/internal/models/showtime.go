package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SeatStatus string

const (
	SeatStatusAvailable SeatStatus = "AVAILABLE"
	SeatStatusLocked    SeatStatus = "LOCKED"
	SeatStatusBooked    SeatStatus = "BOOKED"
)

// IsStoredStatus ใช้ตรวจสอบสถานะที่อนุญาตให้เก็บใน MongoDB
// LOCKED จะถูกจัดเก็บใน Redis เท่านั้น
func (s SeatStatus) IsStoredStatus() bool {
	switch s {
	case SeatStatusAvailable, SeatStatusBooked:
		return true
	default:
		return false
	}
}

type ShowtimeStatus string

const (
	ShowtimeStatusActive    ShowtimeStatus = "ACTIVE"
	ShowtimeStatusCancelled ShowtimeStatus = "CANCELLED"
)

func (s ShowtimeStatus) IsValid() bool {
	switch s {
	case ShowtimeStatusActive, ShowtimeStatusCancelled:
		return true
	default:
		return false
	}
}

type Seat struct {
	Code   string     `bson:"code" json:"code"`
	Row    string     `bson:"row" json:"row"`
	Number int        `bson:"number" json:"number"`
	Status SeatStatus `bson:"status" json:"status"`
}

type Showtime struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	MovieID primitive.ObjectID `bson:"movie_id" json:"movie_id"`

	HallName string `bson:"hall_name" json:"hall_name"`

	StartTime time.Time `bson:"start_time" json:"start_time"`
	EndTime   time.Time `bson:"end_time" json:"end_time"`

	Price    int64  `bson:"price" json:"price"`
	Currency string `bson:"currency" json:"currency"`

	SeatRows       int            `bson:"seat_rows" json:"seat_rows"`
	SeatsPerRow    int            `bson:"seats_per_row" json:"seats_per_row"`
	Seats          []Seat         `bson:"seats" json:"seats"`
	TotalSeats     int            `bson:"total_seats" json:"total_seats"`
	ShowtimeStatus ShowtimeStatus `bson:"status" json:"status"`

	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `bson:"deleted_at,omitempty" json:"-"`
}

type ShowtimeResponse struct {
	ID      primitive.ObjectID `json:"id"`
	MovieID primitive.ObjectID `json:"movie_id"`

	HallName string `json:"hall_name"`

	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	Price    int64  `json:"price"`
	Currency string `json:"currency"`

	SeatRows    int `json:"seat_rows"`
	SeatsPerRow int `json:"seats_per_row"`
	TotalSeats  int `json:"total_seats"`

	Status ShowtimeStatus `json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SeatMapSummary struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Locked    int `json:"locked"`
	Booked    int `json:"booked"`
}

type SeatMapResponse struct {
	ShowtimeID primitive.ObjectID `json:"showtime_id"`
	MovieID    primitive.ObjectID `json:"movie_id"`

	HallName  string    `json:"hall_name"`
	StartTime time.Time `json:"start_time"`

	Price    int64  `json:"price"`
	Currency string `json:"currency"`

	Status  ShowtimeStatus `json:"status"`
	Summary SeatMapSummary `json:"summary"`
	Seats   []Seat         `json:"seats"`
}

func NewShowtimeResponse(
	showtime *Showtime,
) ShowtimeResponse {
	if showtime == nil {
		return ShowtimeResponse{}
	}

	return ShowtimeResponse{
		ID:      showtime.ID,
		MovieID: showtime.MovieID,

		HallName: showtime.HallName,

		StartTime: showtime.StartTime,
		EndTime:   showtime.EndTime,

		Price:    showtime.Price,
		Currency: showtime.Currency,

		SeatRows:    showtime.SeatRows,
		SeatsPerRow: showtime.SeatsPerRow,
		TotalSeats:  showtime.TotalSeats,

		Status: showtime.ShowtimeStatus,

		CreatedAt: showtime.CreatedAt,
		UpdatedAt: showtime.UpdatedAt,
	}
}

func NewShowtimeResponses(
	showtimes []Showtime,
) []ShowtimeResponse {
	responses := make(
		[]ShowtimeResponse,
		0,
		len(showtimes),
	)

	for index := range showtimes {
		responses = append(
			responses,
			NewShowtimeResponse(&showtimes[index]),
		)
	}

	return responses
}

func NewSeatMapResponse(
	showtime *Showtime,
) SeatMapResponse {
	if showtime == nil {
		return SeatMapResponse{}
	}

	summary := SeatMapSummary{
		Total: len(showtime.Seats),
	}

	for _, seat := range showtime.Seats {
		switch seat.Status {
		case SeatStatusAvailable:
			summary.Available++

		case SeatStatusLocked:
			summary.Locked++

		case SeatStatusBooked:
			summary.Booked++
		}
	}

	return SeatMapResponse{
		ShowtimeID: showtime.ID,
		MovieID:    showtime.MovieID,

		HallName:  showtime.HallName,
		StartTime: showtime.StartTime,

		Price:    showtime.Price,
		Currency: showtime.Currency,

		Status:  showtime.ShowtimeStatus,
		Summary: summary,
		Seats:   showtime.Seats,
	}
}
