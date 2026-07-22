package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		provided  string
		wantExact string
	}{
		{
			name:      "preserves valid client request ID",
			provided:  "client-request-123",
			wantExact: "client-request-123",
		},
		{
			name:     "generates missing request ID",
			provided: "",
		},
		{
			name:     "replaces invalid request ID",
			provided: "bad id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestID())
			router.GET("/", func(c *gin.Context) {
				if GetRequestID(c) == "" {
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.provided != "" {
				request.Header.Set(RequestIDHeader, test.provided)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			requestID := response.Header().Get(RequestIDHeader)
			if requestID == "" {
				t.Fatal("expected response request ID")
			}
			if test.wantExact != "" && requestID != test.wantExact {
				t.Fatalf("expected request ID %q, got %q", test.wantExact, requestID)
			}
			if test.wantExact == "" && requestID == test.provided {
				t.Fatalf("expected generated request ID, got %q", requestID)
			}
		})
	}
}
