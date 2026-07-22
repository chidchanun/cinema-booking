package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidJWTConfig    = errors.New("Invalid JWT configuration")
	ErrInvalidAccessToken  = errors.New("Invalid access token")
	ErrAccessTokenExpired  = errors.New("Access token expired")
	ErrGenerateAccessToken = errors.New("Failed to generate access token")
)

type AccessTokenClaims struct {
	UserID string          `json:"user_id"`
	Role   models.UserRole `json:"role"`

	jwt.RegisteredClaims
}

type AccessToken struct {
	Token     string
	TokenType string
	ExpiresAt time.Time
	ExpiresIn int64
}

type JWTService struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTService(
	secret string,
	issuer string,
	accessTTL time.Duration,
) (*JWTService, error) {
	secret = strings.TrimSpace(secret)
	issuer = strings.TrimSpace(issuer)

	/*
		กำหนดขั้นต่ำ 32 ตัวอักษรเพื่อป้องกัน Secret ที่สั้นเกินไป

		อย่างไรก็ตาม Secret ที่แนะนำควรสร้างจาก
		cryptographically secure random generator
	*/
	if len(secret) < 32 {
		return nil, fmt.Errorf(
			"%w: JWT secret must contain at least 32 characters",
			ErrInvalidJWTConfig,
		)
	}

	if issuer == "" {
		return nil, fmt.Errorf(
			"%w: JWT issuer is required",
			ErrInvalidJWTConfig,
		)
	}

	if accessTTL <= 0 {
		return nil, fmt.Errorf(
			"%w: access token TTL must be greater than zero",
			ErrInvalidJWTConfig,
		)
	}

	return &JWTService{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}, nil
}

func (s *JWTService) GenerateAccessToken(
	user *models.User,
) (*AccessToken, error) {
	if user == nil {
		return nil, fmt.Errorf(
			"%w: user is nil",
			ErrGenerateAccessToken,
		)
	}

	if user.ID.IsZero() {
		return nil, fmt.Errorf(
			"%w: user is nil",
			ErrGenerateAccessToken,
		)
	}

	if !user.Role.IsValid() {
		return nil, fmt.Errorf(
			"%w: invalid user role",
			ErrGenerateAccessToken,
		)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.accessTTL)

	userID := user.ID.Hex()

	claims := AccessTokenClaims{
		UserID: userID,
		Role:   user.Role,

		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        primitive.NewObjectID().Hex(),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %v",
			ErrGenerateAccessToken,
			err,
		)
	}

	return &AccessToken{
		Token:     signedToken,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		ExpiresIn: int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *JWTService) ParseAccessToken(
	rawToken string,
) (*AccessTokenClaims, error) {
	rawToken = strings.TrimSpace(rawToken)

	if rawToken == "" {
		return nil, fmt.Errorf(
			"%w: token is required",
			ErrInvalidAccessToken,
		)
	}

	claims := &AccessTokenClaims{}

	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrAccessTokenExpired
		}

		return nil, fmt.Errorf(
			"%w: %v",
			ErrInvalidAccessToken,
			err,
		)
	}

	if token == nil || !token.Valid {
		return nil, ErrInvalidAccessToken
	}

	if claims.UserID == "" {
		return nil, fmt.Errorf(
			"%w: user_id claim is missing",
			ErrInvalidAccessToken,
		)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf(
			"%w: subject claim is missing",
			ErrInvalidAccessToken,
		)
	}

	if claims.Subject != claims.UserID {
		return nil, fmt.Errorf(
			"%w: subject and user_id do not match",
			ErrInvalidAccessToken,
		)
	}

	if _, err := primitive.ObjectIDFromHex(claims.UserID); err != nil {
		return nil, fmt.Errorf(
			"%w: invalid user_id",
			ErrInvalidAccessToken,
		)
	}

	if !claims.Role.IsValid() {
		return nil, fmt.Errorf(
			"%w: invalid role",
			ErrInvalidAccessToken,
		)
	}

	return claims, nil
}
