package messaging

import (
	"fmt"
	"strings"

	"cinema-booking/internal/events"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	RoutingKeyBookingConfirmed = "booking.confirmed"

	RoutingKeyBookingConfirmedDead = "booking.confirmed.dead"
)

const RoutingKeyAuditDead = "audit.dead"

type TopologyConfig struct {
	Exchange string

	AuditQueue string

	DeadLetterExchange   string
	AuditDeadLetterQueue string
}

func (c TopologyConfig) Validate() error {
	if strings.TrimSpace(c.Exchange) == "" {
		return fmt.Errorf(
			"RabbitMQ exchange is required",
		)
	}

	if strings.TrimSpace(c.AuditQueue) == "" {
		return fmt.Errorf(
			"RabbitMQ audit queue is required",
		)
	}

	if strings.TrimSpace(c.DeadLetterExchange) == "" {
		return fmt.Errorf(
			"RabbitMQ dead letter exchange is required",
		)
	}

	if strings.TrimSpace(c.AuditDeadLetterQueue) == "" {
		return fmt.Errorf(
			"RabbitMQ audit dead letter queue is required",
		)
	}

	return nil
}

func DeclareTopology(
	channel *amqp.Channel,
	config TopologyConfig,
) error {
	if channel == nil {
		return fmt.Errorf(
			"RabbitMQ channel is nil",
		)
	}

	if err := config.Validate(); err != nil {
		return err
	}

	err := channel.ExchangeDeclare(
		config.Exchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"Declare RabbitMQ event exchange: %w",
			err,
		)
	}

	err = channel.ExchangeDeclare(
		config.DeadLetterExchange,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare RabbitMQ dead letter exchange: %w",
			err,
		)
	}

	_, err = channel.QueueDeclare(
		config.AuditDeadLetterQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"declare RabbitMQ audit dead letter queue: %w",
			err,
		)
	}

	err = channel.QueueBind(
		config.AuditDeadLetterQueue,
		RoutingKeyAuditDead,
		config.DeadLetterExchange,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"bind RabbitMQ audit dead letter queue: %w",
			err,
		)
	}

	queueArguments := amqp.Table{
		"x-dead-letter-exchange": config.DeadLetterExchange,

		"x-dead-letter-routing-key": RoutingKeyAuditDead,
	}

	_, err = channel.QueueDeclare(
		config.AuditQueue,
		true,
		false,
		false,
		false,
		queueArguments,
	)
	if err != nil {
		return fmt.Errorf(
			"declare RabbitMQ audit queue: %w",
			err,
		)
	}

	routingKeys := []string{
		events.BookingConfirmedEventType,
		events.BookingTimedOutEventType,
		events.SeatReleasedEventType,
		events.SystemErrorEventType,
	}

	for _, routingKey := range routingKeys {
		if err := channel.QueueBind(
			config.AuditQueue,
			routingKey,
			config.Exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf(
				"bind audit queue with routing key %s: %w",
				routingKey,
				err,
			)
		}
	}

	return nil

}
