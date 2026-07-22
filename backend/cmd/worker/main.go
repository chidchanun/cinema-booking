package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cinema-booking/internal/config"
	"cinema-booking/internal/database"
	"cinema-booking/internal/messaging"
	"cinema-booking/internal/repository"
)

func main() {
	if err := config.Load(); err != nil {
		log.Fatalf(
			"load configuration: %v",
			err,
		)
	}

	workerContext, stopWorker :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
	defer stopWorker()

	mongoDB, err := database.ConnectMongoDB()
	if err != nil {
		log.Fatalf(
			"connect MongoDB: %v",
			err,
		)
	}

	defer func() {
		disconnectContext, cancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
		defer cancel()

		if err := mongoDB.Disconnect(
			disconnectContext,
		); err != nil {
			log.Printf(
				"disconnect MongoDB: %v",
				err,
			)
		}
	}()

	rabbitMQ, err := database.ConnectRabbitMQ()
	if err != nil {
		log.Fatalf(
			"connect RabbitMQ: %v",
			err,
		)
	}

	defer func() {
		if err := rabbitMQ.Close(); err != nil {
			log.Printf(
				"close RabbitMQ: %v",
				err,
			)
		}
	}()

	auditLogRepository :=
		repository.NewAuditLogRepository(
			mongoDB.Database,
		)

	topologyConfig := messaging.TopologyConfig{
		Exchange: config.App.RabbitMQExchange,

		AuditQueue: config.App.RabbitMQAuditQueue,

		DeadLetterExchange: config.App.
			RabbitMQDeadLetterExchange,

		AuditDeadLetterQueue: config.App.RabbitMQAuditDLQ,
	}

	auditWorker, err := messaging.NewAuditWorker(
		rabbitMQ.Connection,
		topologyConfig,
		config.App.RabbitMQPrefetch,
		auditLogRepository,
	)
	if err != nil {
		log.Fatalf(
			"create audit worker: %v",
			err,
		)
	}

	log.Println(
		"Cinema Booking audit worker started",
	)

	err = auditWorker.Run(workerContext)
	if err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatalf(
			"audit worker stopped: %v",
			err,
		)
	}

	log.Println(
		"Cinema Booking audit worker stopped",
	)
}
