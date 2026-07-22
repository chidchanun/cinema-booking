package realtime

import (
	"context"
	"net/http"
	"testing"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type originTestShowtimeReader struct{}

func (*originTestShowtimeReader) GetPublicShowtime(
	context.Context,
	primitive.ObjectID,
) (*models.Showtime, error) {
	return &models.Showtime{}, nil
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
