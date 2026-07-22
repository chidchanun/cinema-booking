package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authpkg "cinema-booking/internal/auth"
	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrIDTokenRequired = errors.New("Google ID token is required")

type GoogleUserRepository interface {
	UpsertGoogleUser(
		ctx context.Context,
		profile models.GoogleUserProfile,
	) (*models.User, error)
	FindByID(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.User, error)
}

// GetUser returns the current user profile represented by the access token.
func (s *AuthService) GetUser(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.User, error) {
	user, err := s.userRepository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find authenticated user: %w", err)
	}

	return user, nil
}

type AccessTokenIssuer interface {
	GenerateAccessToken(
		user *models.User,
	) (*authpkg.AccessToken, error)
}

type GoogleLoginResult struct {
	User        *models.User
	AccessToken *authpkg.AccessToken
}

type AuthService struct {
	googleVerifier authpkg.GoogleTokenVerifier
	userRepository GoogleUserRepository
	jwtService     AccessTokenIssuer
}

func NewAuthService(
	googleVerifier authpkg.GoogleTokenVerifier,
	userRepository GoogleUserRepository,
	jwtService AccessTokenIssuer,
) *AuthService {
	if googleVerifier == nil {
		panic("auth service: Google verifier is nil")
	}

	if userRepository == nil {
		panic("auth service: user repository is nil")
	}

	if jwtService == nil {
		panic("auth service: JWT service is nil")
	}

	return &AuthService{
		googleVerifier: googleVerifier,
		userRepository: userRepository,
		jwtService:     jwtService,
	}
}

func (s *AuthService) LoginWithGoogle(
	ctx context.Context,
	idToken string,
) (*GoogleLoginResult, error) {
	idToken = strings.TrimSpace(idToken)

	if idToken == "" {
		return nil, ErrIDTokenRequired
	}

	profile, err := s.googleVerifier.Verify(
		ctx,
		idToken,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"verify Google ID token: %w",
			err,
		)
	}

	user, err := s.userRepository.UpsertGoogleUser(
		ctx,
		*profile,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"upsert Google user: %w",
			err,
		)
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf(
			"generate access token: %w",
			err,
		)
	}

	return &GoogleLoginResult{
		User:        user,
		AccessToken: accessToken,
	}, nil
}
