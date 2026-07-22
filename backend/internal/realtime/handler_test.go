package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type originTestShowtimeReader struct{}

func (*originTestShowtimeReader) GetPublicShowtime(
	context.Context,
	primitive.ObjectID,
) (*models.Showtime, error) {
	return &models.Showtime{}, nil
}

func TestWebSocketReceivesLockedAndAvailableEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	showtimeID := primitive.NewObjectID()
	hub := NewHub(8)
	hubContext, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubContext)

	handler, err := NewHandler(
		hub,
		&originTestShowtimeReader{},
		[]string{"http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("create realtime handler: %v", err)
	}

	router := gin.New()
	router.GET("/ws/:showtimeID", handler.Connect)
	server := httptest.NewServer(router)
	defer server.Close()

	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/" + showtimeID.Hex()
	connection, _, err := websocket.DefaultDialer.Dial(websocketURL, nil)
	if err != nil {
		t.Fatalf("connect websocket: %v", err)
	}
	defer connection.Close()
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set websocket read deadline: %v", err)
	}
	// The client-side dial returns after the HTTP upgrade; allow Connect to
	// finish registering the socket before publishing the first business event.
	time.Sleep(10 * time.Millisecond)

	events := []SeatEvent{
		{
			Type: EventSeatStatusChanged, ShowtimeID: showtimeID.Hex(),
			SeatCode: "A1", Status: models.SeatStatusLocked, OccurredAt: time.Now().UTC(),
		},
		{
			Type: EventSeatStatusChanged, ShowtimeID: showtimeID.Hex(),
			SeatCode: "A1", Status: models.SeatStatusAvailable, OccurredAt: time.Now().UTC(),
		},
	}

	for _, expected := range events {
		if err := hub.Publish(context.Background(), expected); err != nil {
			t.Fatalf("publish %s event: %v", expected.Status, err)
		}

		_, payload, err := connection.ReadMessage()
		if err != nil {
			t.Fatalf("read %s event: %v", expected.Status, err)
		}
		var received SeatEvent
		if err := json.Unmarshal(payload, &received); err != nil {
			t.Fatalf("decode websocket event: %v", err)
		}
		if received.ShowtimeID != showtimeID.Hex() || received.SeatCode != "A1" || received.Status != expected.Status {
			t.Fatalf("unexpected websocket event: %+v", received)
		}
	}
}

func TestWebSocketOriginAllowlist(t *testing.T) {
	handler, err := NewHandler(
		NewHub(1),
		&originTestShowtimeReader{},
		[]string{"http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("create realtime handler: %v", err)
	}

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{
			name:    "configured browser origin is accepted",
			origin:  "http://localhost:5173",
			allowed: true,
		},
		{
			name:    "unknown browser origin is rejected",
			origin:  "https://attacker.example",
			allowed: false,
		},
		{
			name:    "non-browser client without origin is accepted",
			origin:  "",
			allowed: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(
				http.MethodGet,
				"http://localhost/ws",
				nil,
			)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}

			if got := handler.upgrader.CheckOrigin(request); got != test.allowed {
				t.Fatalf("expected allowed=%t, got %t", test.allowed, got)
			}
		})
	}
}

func TestNewRealtimeHandlerRequiresOrigins(t *testing.T) {
	_, err := NewHandler(
		NewHub(1),
		&originTestShowtimeReader{},
		nil,
	)
	if err == nil {
		t.Fatal("expected an error for an empty origin allowlist")
	}
}
