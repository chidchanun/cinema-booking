package auth

import (
	"errors"
	"testing"
	"time"

	"cinema-booking/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestJWTServiceRoundTrip(t *testing.T) {
	service, err := NewJWTService(
		testJWTSecret,
		"test-issuer",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create JWT service: %v", err)
	}

	user := &models.User{
		ID:   primitive.NewObjectID(),
		Role: models.RoleUser,
	}

	accessToken, err := service.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims, err := service.ParseAccessToken(accessToken.Token)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.UserID != user.ID.Hex() {
		t.Fatalf("expected user ID %s, got %s", user.ID.Hex(), claims.UserID)
	}
	if claims.Role != models.RoleUser {
		t.Fatalf("expected role %s, got %s", models.RoleUser, claims.Role)
	}
}

func TestJWTServiceRejectsExpiredToken(t *testing.T) {
	service, err := NewJWTService(
		testJWTSecret,
		"test-issuer",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create JWT service: %v", err)
	}

	userID := primitive.NewObjectID().Hex()
	now := time.Now().UTC()
	claims := AccessTokenClaims{
		UserID: userID,
		Role:   models.RoleUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
		},
	}

	rawToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	_, err = service.ParseAccessToken(rawToken)
	if !errors.Is(err, ErrAccessTokenExpired) {
		t.Fatalf("expected ErrAccessTokenExpired, got %v", err)
	}
}

func TestJWTServiceRejectsDifferentSecret(t *testing.T) {
	issuer, _ := NewJWTService(
		testJWTSecret,
		"test-issuer",
		15*time.Minute,
	)
	parser, _ := NewJWTService(
		"abcdef0123456789abcdef0123456789",
		"test-issuer",
		15*time.Minute,
	)

	accessToken, err := issuer.GenerateAccessToken(&models.User{
		ID:   primitive.NewObjectID(),
		Role: models.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	_, err = parser.ParseAccessToken(accessToken.Token)
	if !errors.Is(err, ErrInvalidAccessToken) {
		t.Fatalf("expected ErrInvalidAccessToken, got %v", err)
	}
}
