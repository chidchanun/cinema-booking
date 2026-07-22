package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrInvalidCookieConfig       = errors.New("Invalid cookie configuration")
	ErrAccessTokenCookieNotFound = errors.New("Access token cookie not found")
	ErrInvalidAccessTokenCookie  = errors.New("Invalid access token cookie")
	ErrCSRFCookieNotFound        = errors.New("CSRF cookie not found")
	ErrInvalidCSRFCookie         = errors.New("Invalid CSRF cookie")
)

type CookieService struct {
	name     string
	domain   string
	secure   bool
	sameSite http.SameSite
}

const csrfTokenBytes = 32

func NewCookieService(
	name string,
	domain string,
	secure bool,
	sameSiteValue string,
) (*CookieService, error) {

	name = strings.TrimSpace(name)
	domain = strings.TrimSpace(domain)
	sameSiteValue = strings.ToLower(
		strings.TrimSpace(sameSiteValue),
	)

	if name == "" {
		return nil, fmt.Errorf(
			"%w: cookie name is required",
			ErrInvalidCookieConfig,
		)
	}

	sameSite, err := parseSameSite(sameSiteValue)
	if err != nil {
		return nil, err
	}

	// SameSite=None ควรใช้ร่วมกับ Secure
	if sameSite == http.SameSiteNoneMode && !secure {
		return nil, fmt.Errorf(
			"%w: SameSite=None requires Secure=true",
			ErrInvalidCookieConfig,
		)
	}

	return &CookieService{
		name:     name,
		domain:   domain,
		secure:   secure,
		sameSite: sameSite,
	}, nil
}

func (s *CookieService) SetAccessToken(
	w http.ResponseWriter,
	accessToken *AccessToken,
) error {
	if w == nil {
		return fmt.Errorf(
			"%w: response writer is nil",
			ErrInvalidAccessTokenCookie,
		)
	}

	if accessToken == nil {
		return fmt.Errorf(
			"%w: access token is nil",
			ErrInvalidAccessTokenCookie,
		)
	}

	token := strings.TrimSpace(accessToken.Token)
	if token == "" {
		return fmt.Errorf(
			"%w: token value is empty",
			ErrInvalidAccessTokenCookie,
		)
	}

	maxAge := int(accessToken.ExpiresIn)
	if maxAge <= 0 {
		return fmt.Errorf(
			"%w: token expiration must be greater than zero",
			ErrInvalidAccessTokenCookie,
		)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    token,
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   maxAge,
		Expires:  accessToken.ExpiresAt.UTC(),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})

	return nil
}

func (s *CookieService) ReadAccessToken(
	r *http.Request,
) (string, error) {
	if r == nil {
		return "", fmt.Errorf(
			"%w: request is nil",
			ErrInvalidAccessTokenCookie,
		)
	}

	cookie, err := r.Cookie(s.name)

	if errors.Is(err, http.ErrNoCookie) {
		return "", ErrAccessTokenCookieNotFound
	}

	if err != nil {
		return "", fmt.Errorf(
			"read access token cookie: %w",
			err,
		)
	}

	token := strings.TrimSpace(cookie.Value)

	if token == "" {
		return "", ErrInvalidAccessTokenCookie
	}

	return token, nil
}

func (s *CookieService) ClearAccessToken(
	w http.ResponseWriter,
) {
	if w == nil {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.name,
		Value:    "",
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})
}

// SetCSRFToken creates a token that browser clients must echo in the
// X-CSRF-Token header for authenticated state-changing requests.
func (s *CookieService) SetCSRFToken(
	w http.ResponseWriter,
) (string, error) {
	if w == nil {
		return "", fmt.Errorf(
			"%w: response writer is nil",
			ErrInvalidCSRFCookie,
		)
	}

	randomBytes := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf(
			"generate CSRF token: %w",
			err,
		)
	}

	token := base64.RawURLEncoding.EncodeToString(randomBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookieName(),
		Value:    token,
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   int((24 * time.Hour).Seconds()),
		Expires:  time.Now().UTC().Add(24 * time.Hour),
		HttpOnly: false,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})

	return token, nil
}

func (s *CookieService) ReadCSRFToken(
	r *http.Request,
) (string, error) {
	if r == nil {
		return "", fmt.Errorf(
			"%w: request is nil",
			ErrInvalidCSRFCookie,
		)
	}

	cookie, err := r.Cookie(s.csrfCookieName())
	if errors.Is(err, http.ErrNoCookie) {
		return "", ErrCSRFCookieNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read CSRF cookie: %w", err)
	}

	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return "", ErrInvalidCSRFCookie
	}

	return token, nil
}

func (s *CookieService) ClearCSRFToken(w http.ResponseWriter) {
	if w == nil {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookieName(),
		Value:    "",
		Path:     "/",
		Domain:   s.domain,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: false,
		Secure:   s.secure,
		SameSite: s.sameSite,
	})
}

func (s *CookieService) csrfCookieName() string {
	return s.name + "_csrf"
}

func parseSameSite(
	value string,
) (http.SameSite, error) {
	switch value {
	case "", "lax":
		return http.SameSiteLaxMode, nil

	case "strict":
		return http.SameSiteStrictMode, nil

	case "none":
		return http.SameSiteNoneMode, nil

	default:
		return http.SameSiteDefaultMode, fmt.Errorf(
			"%w: COOKIE_SAME_SITE must be lax, strict, or none",
			ErrInvalidCookieConfig,
		)
	}
}
