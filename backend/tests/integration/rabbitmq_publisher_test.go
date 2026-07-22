//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"cinema-booking/internal/events"
	"cinema-booking/internal/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRabbitMQPublisherRoutesAuditEvent(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("set RUN_INTEGRATION_TESTS=1 to run integration tests")
	}

	rabbitMQURL := strings.TrimSpace(os.Getenv("RABBITMQ_TEST_URL"))
	if rabbitMQURL == "" {
		rabbitMQURL = strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	}
	if rabbitMQURL == "" {
		t.Skip("RABBITMQ_TEST_URL or RABBITMQ_URL is required")
	}

	connection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		t.Fatalf("connect RabbitMQ test server: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	suffix := primitive.NewObjectID().Hex()
	topology := messaging.TopologyConfig{
		Exchange:             "cinema.test.events." + suffix,
		AuditQueue:           "cinema.test.audit." + suffix,
		DeadLetterExchange:   "cinema.test.dlx." + suffix,
		AuditDeadLetterQueue: "cinema.test.audit.dlq." + suffix,
	}

	publisher, err := messaging.NewRecoveringPublisher(
		connection,
		rabbitMQURL,
		topology,
	)
	if err != nil {
		t.Fatalf("create RabbitMQ publisher: %v", err)
	}
	t.Cleanup(func() { _ = publisher.Close() })

	consumerConnection, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		t.Fatalf("connect RabbitMQ consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumerConnection.Close() })

	consumerChannel, err := consumerConnection.Channel()
	if err != nil {
		t.Fatalf("open RabbitMQ consumer channel: %v", err)
	}
	t.Cleanup(func() { _ = consumerChannel.Close() })

	t.Cleanup(func() {
		cleanupChannel, cleanupErr := consumerConnection.Channel()
		if cleanupErr != nil {
			return
		}
		defer cleanupChannel.Close()
		_, _ = cleanupChannel.QueueDelete(topology.AuditQueue, false, false, false)
		_, _ = cleanupChannel.QueueDelete(topology.AuditDeadLetterQueue, false, false, false)
		_ = cleanupChannel.ExchangeDelete(topology.Exchange, false, false)
		_ = cleanupChannel.ExchangeDelete(topology.DeadLetterExchange, false, false)
	})

	deliveries, err := consumerChannel.Consume(
		topology.AuditQueue,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("consume RabbitMQ audit queue: %v", err)
	}

	event, err := events.NewSystemError(events.SystemErrorData{
		Component:    "integration-test",
		Operation:    "publish",
		ErrorCode:    "test_event",
		ErrorMessage: "RabbitMQ integration test event",
	})
	if err != nil {
		t.Fatalf("create system error event: %v", err)
	}

	publishContext, publishCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer publishCancel()

	if err := publisher.Publish(publishContext, event); err != nil {
		t.Fatalf("publish RabbitMQ event: %v", err)
	}

	select {
	case delivery := <-deliveries:
		var received events.SystemError
		if err := json.Unmarshal(delivery.Body, &received); err != nil {
			t.Fatalf("decode delivered event: %v", err)
		}
		if received.EventID != event.EventID {
			t.Fatalf("expected event ID %s, got %s", event.EventID, received.EventID)
		}
		if delivery.RoutingKey != events.SystemErrorEventType {
			t.Fatalf(
				"expected routing key %s, got %s",
				events.SystemErrorEventType,
				delivery.RoutingKey,
			)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for RabbitMQ audit event")
	}

	// Force the original connection closed. The next publish must establish a
	// new connection and redeclare the topology before sending the event.
	if err := connection.Close(); err != nil {
		t.Fatalf("close original RabbitMQ connection: %v", err)
	}

	recoveredEvent, err := events.NewSystemError(events.SystemErrorData{
		Component:    "integration-test",
		Operation:    "reconnect",
		ErrorCode:    "recovered_event",
		ErrorMessage: "RabbitMQ reconnect test event",
	})
	if err != nil {
		t.Fatalf("create reconnect event: %v", err)
	}
	if err := publisher.Publish(publishContext, recoveredEvent); err != nil {
		t.Fatalf("publish event after reconnect: %v", err)
	}

	select {
	case delivery := <-deliveries:
		var received events.SystemError
		if err := json.Unmarshal(delivery.Body, &received); err != nil {
			t.Fatalf("decode event after reconnect: %v", err)
		}
		if received.EventID != recoveredEvent.EventID {
			t.Fatalf(
				"expected recovered event ID %s, got %s",
				recoveredEvent.EventID,
				received.EventID,
			)
		}

	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for event after RabbitMQ reconnect")
	}
}
