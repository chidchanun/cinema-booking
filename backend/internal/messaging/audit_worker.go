package messaging

import (
	"context"
	"errors"
	"fmt"
	"log"

	"cinema-booking/internal/events"
	"cinema-booking/internal/models"
	"cinema-booking/internal/repository"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AuditLogStore interface {
	Create(
		ctx context.Context,
		auditLog *models.AuditLog,
	) error
}

type AuditWorker struct {
	connection *amqp.Connection

	topologyConfig TopologyConfig
	prefetch       int

	auditLogRepository AuditLogStore
}

func NewAuditWorker(
	connection *amqp.Connection,
	topologyConfig TopologyConfig,
	prefetch int,
	auditLogRepository AuditLogStore,
) (*AuditWorker, error) {
	if connection == nil || connection.IsClosed() {
		return nil, fmt.Errorf(
			"RabbitMQ connection is unavailable",
		)
	}

	if err := topologyConfig.Validate(); err != nil {
		return nil, err
	}

	if prefetch < 1 {
		return nil, fmt.Errorf(
			"RabbitMQ prefetch must be greater than zero",
		)
	}

	if auditLogRepository == nil {
		return nil, fmt.Errorf(
			"audit log repository is nil",
		)
	}

	return &AuditWorker{
		connection: connection,

		topologyConfig: topologyConfig,
		prefetch:       prefetch,

		auditLogRepository: auditLogRepository,
	}, nil
}

func (w *AuditWorker) Run(
	ctx context.Context,
) error {
	channel, err := w.connection.Channel()
	if err != nil {
		return fmt.Errorf(
			"open RabbitMQ consumer channel: %w",
			err,
		)
	}
	defer channel.Close()

	if err := DeclareTopology(
		channel,
		w.topologyConfig,
	); err != nil {
		return err
	}

	if err := channel.Qos(
		w.prefetch,
		0,
		false,
	); err != nil {
		return fmt.Errorf(
			"configure RabbitMQ consumer QoS: %w",
			err,
		)
	}

	deliveries, err := channel.ConsumeWithContext(
		ctx,
		w.topologyConfig.AuditQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"consume RabbitMQ audit queue: %w",
			err,
		)
	}

	log.Printf(
		"Audit worker consuming queue %s",
		w.topologyConfig.AuditQueue,
	)

	for {
		select {
		case <-ctx.Done():
			return nil

		case delivery, open := <-deliveries:
			if !open {
				if ctx.Err() != nil {
					return nil
				}

				return fmt.Errorf(
					"RabbitMQ audit delivery channel closed",
				)
			}

			w.handleDelivery(
				ctx,
				delivery,
			)
		}
	}
}

func (w *AuditWorker) handleDelivery(
	ctx context.Context,
	delivery amqp.Delivery,
) {
	err := w.processEvent(
		ctx,
		delivery.Body,
	)

	switch {
	case err == nil:
		if ackErr := delivery.Ack(false); ackErr != nil {
			log.Printf(
				"ack audit message %s: %v",
				delivery.MessageId,
				ackErr,
			)
		}

	case errors.Is(
		err,
		repository.ErrAuditLogAlreadyExists,
	):
		// Event เดิมถูกประมวลผลแล้ว ถือว่าสำเร็จ
		if ackErr := delivery.Ack(false); ackErr != nil {
			log.Printf(
				"ack duplicate audit message %s: %v",
				delivery.MessageId,
				ackErr,
			)
		}

	case errors.Is(
		err,
		events.ErrInvalidEvent,
	),
		errors.Is(
			err,
			repository.ErrInvalidAuditLog,
		):

	default:
		log.Printf(
			"process audit message %s failed: %v",
			delivery.MessageId,
			err,
		)

		/*
			ลองใหม่หนึ่งครั้ง

			หากเป็น Message ที่ถูก Redeliver มาแล้ว
			ให้ส่งเข้า Dead Letter Queue เพื่อไม่ให้วนไม่สิ้นสุด
		*/
		requeue := !delivery.Redelivered

		if nackErr := delivery.Nack(
			false,
			requeue,
		); nackErr != nil {
			log.Printf(
				"nack audit message: %v",
				nackErr,
			)
		}
	}
}

func (w *AuditWorker) processEvent(
	ctx context.Context,
	body []byte,
) error {
	auditLog, err := MapAuditEvent(body)
	if err != nil {
		return err
	}

	return w.auditLogRepository.Create(
		ctx,
		auditLog,
	)
}
