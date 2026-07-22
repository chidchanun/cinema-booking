package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"cinema-booking/internal/models"

	"google.golang.org/api/idtoken"
)

var (
	ErrInvalidGoogleIDToken = errors.New("invalid Google ID token")
	ErrInvalidGoogleClaims  = errors.New("invalid Google token claims")
)

type GoogleTokenVerifier interface {
	Verify(
		ctx context.Context,
		rawToken string,
	) (*models.GoogleUserProfile, error)
}

type GoogleVerifier struct {
	clientID string
}

func NewGoogleVerifier(clientID string) (*GoogleVerifier, error) {
	clientID = strings.TrimSpace(clientID)

	if clientID == "" {
		return nil, fmt.Errorf("Google client ID is required")
	}

	return &GoogleVerifier{
		clientID: clientID,
	}, nil
}

func (v *GoogleVerifier) Verify(
	ctx context.Context,
	rawToken string,
) (*models.GoogleUserProfile, error) {
	rawToken = strings.TrimSpace(rawToken)

	if rawToken == "" {
		return nil, fmt.Errorf(
			"%w: token is required",
			ErrInvalidGoogleIDToken,
		)
	}

	/*
		clientID ถูกส่งเป็น audience

		จึงป้องกัน Token ที่ออกให้ Application อื่น
		ถูกนำมาใช้ Login กับ Backend ของเรา
	*/

	payload, err := idtoken.Validate(
		ctx,
		rawToken,
		v.clientID,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"%w: %v",
			ErrInvalidGoogleIDToken,
			err,
		)
	}

	if payload.Issuer != "accounts.google.com" &&
		payload.Issuer != "https://accounts.google.com" {
		return nil, fmt.Errorf(
			"%w: invalid issuer",
			ErrInvalidGoogleIDToken,
		)
	}

	googleID := strings.TrimSpace(payload.Subject)
	if googleID == "" {
		return nil, fmt.Errorf(
			"%w: sub claim is required",
			ErrInvalidGoogleClaims,
		)
	}

	email, err := requiredStringClaim(
		payload.Claims,
		"email",
	)

	if err != nil {
		return nil, err
	}

	emailVerified, err := requiredBoolClaim(
		payload.Claims,
		"email_verified",
	)

	if err != nil {
		return nil, err
	}

	name := optionalStringClaim(
		payload.Claims,
		"name",
	)

	picture := optionalStringClaim(
		payload.Claims,
		"picture",
	)

	return &models.GoogleUserProfile{
		GoogleID:      googleID,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
		Picture:       picture,
	}, nil

}

func requiredStringClaim(
	claims map[string]interface{},
	key string,
) (string, error) {
	value, ok := claims[key]
	if !ok {
		return "", fmt.Errorf(
			"%w: %s is required",
			ErrInvalidGoogleClaims,
			key,
		)
	}

	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf(
			"%w: %s must be a string",
			ErrInvalidGoogleClaims,
			key,
		)
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf(
			"%w: %s cannot be empty",
			ErrInvalidGoogleClaims,
			key,
		)
	}

	return result, nil
}

func optionalStringClaim(
	claims map[string]interface{},
	key string,
) string {
	value, ok := claims[key]
	if !ok {
		return ""
	}

	result, ok := value.(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(result)
}

func requiredBoolClaim(
	claims map[string]interface{},
	key string,
) (bool, error) {
	value, ok := claims[key]
	if !ok {
		return false, fmt.Errorf(
			"%w: %s is required",
			ErrInvalidGoogleClaims,
			key,
		)
	}

	switch typedValue := value.(type) {
	case bool:
		return typedValue, nil

	case string:
		parsedValue, err := strconv.ParseBool(typedValue)
		if err != nil {
			return false, fmt.Errorf(
				"%w: %s must be a boolean",
				ErrInvalidGoogleClaims,
				key,
			)
		}

		return parsedValue, nil

	default:
		return false, fmt.Errorf(
			"%w: %s must be a boolean",
			ErrInvalidGoogleClaims,
			key,
		)
	}
}
