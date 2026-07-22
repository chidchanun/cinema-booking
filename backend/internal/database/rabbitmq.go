package database

import (
	"fmt"
	"strings"

	"cinema-booking/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	Connection *amqp.Connection
}

func ConnectRabbitMQ() (*RabbitMQ, error) {
	rabbitMQURL := strings.TrimSpace(
		config.App.RabbitMQURL,
	)

	if rabbitMQURL == "" {
		return nil, fmt.Errorf(
			"RabbitMQ URL is required",
		)
	}

	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf(
			"connect RabbitMQ: %w",
			err,
		)
	}

	return &RabbitMQ{
		Connection: connection,
	}, nil
}

func (r *RabbitMQ) IsHealthy() bool {
	return r != nil &&
		r.Connection != nil &&
		!r.Connection.IsClosed()
}

func (r *RabbitMQ) Close() error {
	if r == nil || r.Connection == nil || r.Connection.IsClosed() {
		return nil
	}

	if err := r.Connection.Close(); err != nil {
		return fmt.Errorf(
			"close RabbitMQ connection: %w",
			err,
		)
	}

	return nil
}
