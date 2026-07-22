package auth

import (
	"net/http/httptest"
	"testing"
)

func TestCookieServiceCSRFTokenRoundTrip(t *testing.T) {
	service, err := NewCookieService(
		"cinema_access_token",
		"",
		false,
		"lax",
	)
	if err != nil {
		t.Fatalf("create cookie service: %v", err)
	}

	response := httptest.NewRecorder()
	token, err := service.SetCSRFToken(response)
	if err != nil {
		t.Fatalf("set CSRF token: %v", err)
	}
	if token == "" {
		t.Fatal("expected a generated CSRF token")
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].HttpOnly {
		t.Fatal("CSRF cookie must be readable by the browser client")
	}

	request := httptest.NewRequest("POST", "/", nil)
	request.AddCookie(cookies[0])

	storedToken, err := service.ReadCSRFToken(request)
	if err != nil {
		t.Fatalf("read CSRF token: %v", err)
	}
	if storedToken != token {
		t.Fatalf("expected token %q, got %q", token, storedToken)
	}
}
