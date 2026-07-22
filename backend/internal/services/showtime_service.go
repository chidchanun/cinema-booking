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

var (
	ErrInvalidShowtimeData = errors.New("invalid showtime data")
	ErrShowtimeConflict    = errors.New("showtime schedule conflict")
	ErrShowtimeUnavailable = errors.New("showtime unavailable")
)

type ShowtimeRepository interface {
	Create(
		ctx context.Context,
		showtime *models.Showtime,
	) error

	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)

	FindByMovieID(
		ctx context.Context,
		movieID primitive.ObjectID,
		from time.Time,
		to time.Time,
	) ([]models.Showtime, error)

	HasHallConflict(
		ctx context.Context,
		hallName string,
		startTime time.Time,
		endTime time.Time,
	) (bool, error)
	ListHalls(ctx context.Context) ([]models.HallSummary, error)

	Cancel(
		ctx context.Context,
		id primitive.ObjectID,
	) error
}

type ShowtimeMovieRepository interface {
	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Movie, error)
}

type ShowtimeService struct {
	showtimeRepository ShowtimeRepository
	movieRepository    ShowtimeMovieRepository
}

type CreateShowtimeInput struct {
	MovieID primitive.ObjectID

	HallName  string
	StartTime time.Time

	Price    int64
	Currency string

	SeatRows    int
	SeatsPerRow int
}

type ShowtimeAvailability struct {
	Available bool
	EndTime   time.Time
}

func (s *ShowtimeService) ListHalls(ctx context.Context) ([]models.HallSummary, error) {
	return s.showtimeRepository.ListHalls(ctx)
}

func (s *ShowtimeService) CheckHallAvailability(
	ctx context.Context,
	movieID primitive.ObjectID,
	hallName string,
	startTime time.Time,
) (*ShowtimeAvailability, error) {
	hallName = strings.TrimSpace(hallName)
	if movieID.IsZero() || hallName == "" || !startTime.After(time.Now().UTC()) {
		return nil, ErrInvalidShowtimeData
	}

	movie, err := s.movieRepository.FindByID(ctx, movieID)
	if err != nil {
		return nil, err
	}
	if !movie.IsActive {
		return nil, ErrMovieUnavailable
	}
	halls, err := s.showtimeRepository.ListHalls(ctx)
	if err != nil {
		return nil, fmt.Errorf("read hall layout: %w", err)
	}
	for _, hall := range halls {
		if strings.EqualFold(hall.Name, hallName) {
			hallName = hall.Name
			break
		}
	}

	endTime := startTime.UTC().Add(time.Duration(movie.DurationMinutes+15) * time.Minute)
	conflict, err := s.showtimeRepository.HasHallConflict(ctx, hallName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("check hall availability: %w", err)
	}

	return &ShowtimeAvailability{Available: !conflict, EndTime: endTime}, nil
}

func NewShowtimeService(
	showtimeRepository ShowtimeRepository,
	movieRepository MovieRepository,
) *ShowtimeService {
	if showtimeRepository == nil {
		panic("showtime service: showtime repository is nil")
	}

	if movieRepository == nil {
		panic("showtime service: movie repository is nil")
	}

	return &ShowtimeService{
		showtimeRepository: showtimeRepository,
		movieRepository:    movieRepository,
	}
}

func (s *ShowtimeService) CreateShowtime(
	ctx context.Context,
	input CreateShowtimeInput,
) (*models.Showtime, error) {
	input.HallName = strings.TrimSpace(input.HallName)
	input.Currency = strings.ToUpper(
		strings.TrimSpace(input.Currency),
	)
	input.StartTime = input.StartTime.UTC()

	if err := validateCreateShowtimeInput(input); err != nil {
		return nil, err
	}

	movie, err := s.movieRepository.FindByID(
		ctx,
		input.MovieID,
	)
	if err != nil {
		return nil, err
	}

	if !movie.IsActive {
		return nil, ErrMovieUnavailable
	}

	halls, err := s.showtimeRepository.ListHalls(ctx)
	if err != nil {
		return nil, fmt.Errorf("read hall layout: %w", err)
	}
	for _, hall := range halls {
		if strings.EqualFold(hall.Name, input.HallName) &&
			(hall.SeatRows != input.SeatRows || hall.SeatsPerRow != input.SeatsPerRow) {
			return nil, fmt.Errorf(
				"%w: existing hall layout is %d rows by %d seats",
				ErrInvalidShowtimeData,
				hall.SeatRows,
				hall.SeatsPerRow,
			)
		}
		if strings.EqualFold(hall.Name, input.HallName) {
			input.HallName = hall.Name
		}
	}

	/*
		EndTime คำนวณจากเวลาฉายหนัง

		เพิ่มเวลาเตรียมโรง 15 นาที เพื่อป้องกัน
		การสร้างรอบใหม่ติดกับรอบก่อนหน้ามากเกินไป
	*/
	endTime := input.StartTime.Add(
		time.Duration(movie.DurationMinutes+15) * time.Minute,
	)

	hasConflict, err := s.showtimeRepository.HasHallConflict(
		ctx,
		input.HallName,
		input.StartTime,
		endTime,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"check showtime conflict: %w",
			err,
		)
	}

	if hasConflict {
		return nil, ErrShowtimeConflict
	}

	seats := generateSeats(
		input.SeatRows,
		input.SeatsPerRow,
	)

	showtime := &models.Showtime{
		MovieID: input.MovieID,

		HallName: input.HallName,

		StartTime: input.StartTime,
		EndTime:   endTime,

		Price:    input.Price,
		Currency: input.Currency,

		SeatRows:    input.SeatRows,
		SeatsPerRow: input.SeatsPerRow,
		Seats:       seats,
		TotalSeats:  len(seats),

		ShowtimeStatus: models.ShowtimeStatusActive,
	}

	if err := s.showtimeRepository.Create(
		ctx,
		showtime,
	); err != nil {
		return nil, fmt.Errorf(
			"create showtime: %w",
			err,
		)
	}

	return showtime, nil
}

func (s *ShowtimeService) GetPublicShowtime(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Showtime, error) {
	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		id,
	)
	if err != nil {
		return nil, err
	}

	if showtime.ShowtimeStatus != models.ShowtimeStatusActive {
		return nil, ErrShowtimeUnavailable
	}

	return showtime, nil
}

func (s *ShowtimeService) GetAdminShowtime(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.Showtime, error) {
	return s.showtimeRepository.FindByID(
		ctx,
		id,
	)
}

func (s *ShowtimeService) ListMovieShowtimes(
	ctx context.Context,
	movieID primitive.ObjectID,
	from time.Time,
	to time.Time,
) ([]models.Showtime, error) {
	if movieID.IsZero() {
		return nil, repository.ErrInvalidMovieID
	}

	from = from.UTC()
	to = to.UTC()

	if to.Before(from) || to.Equal(from) {
		return nil, fmt.Errorf(
			"%w: to must be after from",
			ErrInvalidShowtimeData,
		)
	}

	// ป้องกัน Query ช่วงเวลาที่กว้างเกินไป
	if to.Sub(from) > 90*24*time.Hour {
		return nil, fmt.Errorf(
			"%w: date range must not exceed 90 days",
			ErrInvalidShowtimeData,
		)
	}

	showtimes, err := s.showtimeRepository.FindByMovieID(
		ctx,
		movieID,
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list movie showtimes: %w",
			err,
		)
	}

	return showtimes, nil
}

func (s *ShowtimeService) CancelShowtime(
	ctx context.Context,
	id primitive.ObjectID,
) error {
	showtime, err := s.showtimeRepository.FindByID(
		ctx,
		id,
	)
	if err != nil {
		return err
	}

	if showtime.ShowtimeStatus != models.ShowtimeStatusActive {
		return ErrShowtimeUnavailable
	}

	if showtime.StartTime.Before(time.Now().UTC()) {
		return fmt.Errorf(
			"%w: past showtime cannot be cancelled",
			ErrInvalidShowtimeData,
		)
	}

	if err := s.showtimeRepository.Cancel(
		ctx,
		id,
	); err != nil {
		return fmt.Errorf(
			"cancel showtime: %w",
			err,
		)
	}

	return nil
}

func generateSeats(
	rows int,
	seatsPerRow int,
) []models.Seat {
	total := rows * seatsPerRow

	seats := make([]models.Seat, 0, total)

	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		row := string(rune('A' + rowIndex))

		for seatNumber := 1; seatNumber <= seatsPerRow; seatNumber++ {
			code := fmt.Sprintf(
				"%s%d",
				row,
				seatNumber,
			)

			seats = append(
				seats,
				models.Seat{
					Code:   code,
					Row:    row,
					Number: seatNumber,
					Status: models.SeatStatusAvailable,
				},
			)
		}
	}

	return seats
}

func validateCreateShowtimeInput(
	input CreateShowtimeInput,
) error {
	if input.MovieID.IsZero() {
		return fmt.Errorf(
			"%w: movie_id is required",
			ErrInvalidShowtimeData,
		)
	}

	if input.HallName == "" {
		return fmt.Errorf(
			"%w: hall_name is required",
			ErrInvalidShowtimeData,
		)
	}

	if len(input.HallName) > 100 {
		return fmt.Errorf(
			"%w: hall_name must not exceed 100 characters",
			ErrInvalidShowtimeData,
		)
	}

	if input.StartTime.IsZero() {
		return fmt.Errorf(
			"%w: start_time is required",
			ErrInvalidShowtimeData,
		)
	}

	if !input.StartTime.After(time.Now().UTC()) {
		return fmt.Errorf(
			"%w: start_time must be in the future",
			ErrInvalidShowtimeData,
		)
	}

	if input.Price < 0 {
		return fmt.Errorf(
			"%w: price cannot be negative",
			ErrInvalidShowtimeData,
		)
	}

	if input.Currency == "" {
		return fmt.Errorf(
			"%w: currency is required",
			ErrInvalidShowtimeData,
		)
	}

	if len(input.Currency) != 3 {
		return fmt.Errorf(
			"%w: currency must contain 3 characters",
			ErrInvalidShowtimeData,
		)
	}

	if input.SeatRows < 1 || input.SeatRows > 26 {
		return fmt.Errorf(
			"%w: seat_rows must be between 1 and 26",
			ErrInvalidShowtimeData,
		)
	}

	if input.SeatsPerRow < 1 || input.SeatsPerRow > 50 {
		return fmt.Errorf(
			"%w: seats_per_row must be between 1 and 50",
			ErrInvalidShowtimeData,
		)
	}

	if input.SeatRows*input.SeatsPerRow > 1000 {
		return fmt.Errorf(
			"%w: total seats must not exceed 1000",
			ErrInvalidShowtimeData,
		)
	}

	return nil
}
