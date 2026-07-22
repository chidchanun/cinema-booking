package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"cinema-booking/internal/events"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrPublisherClosed     = errors.New("RabbitMQ publisher is closed")
	ErrMessageNotConfirmed = errors.New("RabbitMQ message was not confirmed")
	ErrMessageNotRoutable  = errors.New("RabbitMQ message was not routable")
)

type Publisher struct {
	channel    *amqp.Channel
	returns    <-chan amqp.Return
	connection *amqp.Connection

	topology       TopologyConfig
	connectionURL  string
	ownsConnection bool

	mutex  sync.Mutex
	closed bool
}

func NewPublisher(
	connection *amqp.Connection,
	topologyConfig TopologyConfig,
) (*Publisher, error) {
	if connection == nil || connection.IsClosed() {
		return nil, errors.New(
			"RabbitMQ connection is unavailable",
		)
	}

	channel, returns, err := configurePublisherChannel(
		connection,
		topologyConfig,
	)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		channel:    channel,
		returns:    returns,
		connection: connection,
		topology:   topologyConfig,
	}, nil
}

// NewRecoveringPublisher reconnects lazily before a later publish when the
// original RabbitMQ connection or channel has been closed.
func NewRecoveringPublisher(
	connection *amqp.Connection,
	connectionURL string,
	topologyConfig TopologyConfig,
) (*Publisher, error) {
	publisher, err := NewPublisher(connection, topologyConfig)
	if err != nil {
		return nil, err
	}

	connectionURL = strings.TrimSpace(connectionURL)
	if connectionURL == "" {
		_ = publisher.Close()
		return nil, fmt.Errorf("RabbitMQ recovery URL is required")
	}

	publisher.connectionURL = connectionURL
	return publisher, nil
}

func (p *Publisher) Publish(
	ctx context.Context,
	event events.Message,
) error {
	if event == nil {
		return events.ErrInvalidEvent
	}

	if err := event.Validate(); err != nil {
		return err
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf(
			"marshal event %s: %w",
			event.Name(),
			err,
		)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return ErrPublisherClosed
	}
	if err := p.ensureChannel(); err != nil {
		return err
	}

	confirmation, err :=
		p.channel.PublishWithDeferredConfirmWithContext(
			ctx,
			p.topology.Exchange,
			event.Name(),
			true,
			false,
			amqp.Publishing{
				Headers: amqp.Table{
					"event_type":    event.Name(),
					"event_version": event.SchemaVersion(),
				},

				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,

				MessageId: event.ID(),
				Type:      event.Name(),
				Timestamp: event.HappenedAt(),

				Body: body,
			},
		)
	if err != nil {
		return fmt.Errorf(
			"publish event %s: %w",
			event.Name(),
			err,
		)
	}

	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf(
			"wait for event confirmation: %w",
			err,
		)
	}

	if !confirmed {
		return ErrMessageNotConfirmed
	}

	/*
		mandatory=true ทำให้ Message ที่ไม่มี Queue รองรับ
		ถูกส่งกลับมาทาง NotifyReturn
	*/
	select {
	case returned, open := <-p.returns:
		if !open {
			return ErrPublisherClosed
		}

		return fmt.Errorf(
			"%w: code=%d reason=%s routing_key=%s",
			ErrMessageNotRoutable,
			returned.ReplyCode,
			returned.ReplyText,
			returned.RoutingKey,
		)

	default:
		return nil
	}
}

func (p *Publisher) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	if p.channel == nil || p.channel.IsClosed() {
		if p.ownsConnection && p.connection != nil && !p.connection.IsClosed() {
			return p.connection.Close()
		}
		return nil
	}

	if err := p.channel.Close(); err != nil {
		return fmt.Errorf(
			"close RabbitMQ publisher channel: %w",
			err,
		)
	}

	if p.ownsConnection && p.connection != nil && !p.connection.IsClosed() {
		if err := p.connection.Close(); err != nil {
			return fmt.Errorf("close recovered RabbitMQ connection: %w", err)
		}
	}

	return nil
}

func (p *Publisher) IsHealthy() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return !p.closed &&
		p.connection != nil &&
		!p.connection.IsClosed() &&
		p.channel != nil &&
		!p.channel.IsClosed()
}

func (p *Publisher) ensureChannel() error {
	if p.connection != nil &&
		!p.connection.IsClosed() &&
		p.channel != nil &&
		!p.channel.IsClosed() {
		return nil
	}
	if p.connectionURL == "" {
		return ErrPublisherClosed
	}

	connection, err := amqp.Dial(p.connectionURL)
	if err != nil {
		return fmt.Errorf("reconnect RabbitMQ publisher: %w", err)
	}

	channel, returns, err := configurePublisherChannel(
		connection,
		p.topology,
	)
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("recover RabbitMQ publisher: %w", err)
	}

	if p.ownsConnection && p.connection != nil && !p.connection.IsClosed() {
		_ = p.connection.Close()
	}

	p.connection = connection
	p.channel = channel
	p.returns = returns
	p.ownsConnection = true
	return nil
}

func configurePublisherChannel(
	connection *amqp.Connection,
	topologyConfig TopologyConfig,
) (*amqp.Channel, <-chan amqp.Return, error) {
	channel, err := connection.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"open RabbitMQ publisher channel: %w",
			err,
		)
	}

	if err := DeclareTopology(channel, topologyConfig); err != nil {
		_ = channel.Close()
		return nil, nil, err
	}

	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		return nil, nil, fmt.Errorf(
			"enable RabbitMQ publisher confirms: %w",
			err,
		)
	}

	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	return channel, returns, nil
}
