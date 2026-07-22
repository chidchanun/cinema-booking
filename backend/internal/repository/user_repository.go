package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidUserID        = errors.New("invalid user id")
	ErrInvalidGoogleProfile = errors.New("invalid Google user profile")
	ErrUserIdentityConflict = errors.New("user identity conflict")
)

const usersCollection = "users"

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	if db == nil {
		panic("user repository: MongoDB database is nil")
	}

	return &UserRepository{
		collection: db.Collection(usersCollection),
	}
}

func (r *UserRepository) FindByID(
	ctx context.Context,
	id primitive.ObjectID,
) (*models.User, error) {
	if id.IsZero() {
		return nil, ErrInvalidUserID
	}

	return r.findOne(ctx, bson.M{
		"_id": id,
	})
}

func (r *UserRepository) FindByGoogleID(
	ctx context.Context,
	googleID string,
) (*models.User, error) {
	googleID = strings.TrimSpace(googleID)

	if googleID == "" {
		return nil, fmt.Errorf(
			"%w: Google ID is required",
			ErrInvalidGoogleProfile,
		)
	}

	return r.findOne(ctx, bson.M{
		"google_id": googleID,
	})
}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*models.User, error) {
	email = normalizeEmail(email)

	if email == "" {
		return nil, fmt.Errorf(
			"%w: email is required",
			ErrInvalidGoogleProfile,
		)
	}

	return r.findOne(ctx, bson.M{
		"email": email,
	})
}

func (r *UserRepository) UpsertGoogleUser(
	ctx context.Context,
	profile models.GoogleUserProfile,
) (*models.User, error) {
	profile.GoogleID = strings.TrimSpace(profile.GoogleID)
	profile.Email = normalizeEmail(profile.Email)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Picture = strings.TrimSpace(profile.Picture)

	if profile.GoogleID == "" {
		return nil, fmt.Errorf(
			"%w: Google ID is required",
			ErrInvalidGoogleProfile,
		)
	}

	if profile.Email == "" {
		return nil, fmt.Errorf(
			"%w: email is required",
			ErrInvalidGoogleProfile,
		)
	}

	if !profile.EmailVerified {
		return nil, fmt.Errorf(
			"%w: Google email is not verified",
			ErrInvalidGoogleProfile,
		)
	}

	if profile.Name == "" {
		profile.Name = profile.Email
	}

	now := time.Now().UTC()

	// ใช้ Google sub เป็น Identity หลัก
	filter := bson.M{
		"google_id": profile.GoogleID,
	}

	update := bson.M{
		"$set": bson.M{
			"email":          profile.Email,
			"email_verified": profile.EmailVerified,
			"name":           profile.Name,
			"picture":        profile.Picture,
			"last_login_at":  now,
			"updated_at":     now,
		},
		"$setOnInsert": bson.M{
			"_id":        primitive.NewObjectID(),
			"google_id":  profile.GoogleID,
			"role":       models.RoleUser,
			"created_at": now,
		},
	}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var user models.User

	err := r.collection.
		FindOneAndUpdate(
			ctx,
			filter,
			update,
			opts,
		).
		Decode(&user)

	if err == nil {
		return &user, nil
	}

	if mongo.IsDuplicateKeyError(err) {
		/*
			รองรับกรณี Login ครั้งแรกพร้อมกันหลาย Request
			โดยลอง Update User ที่ Request อื่นสร้างสำเร็จแล้ว
		*/
		retryOptions := options.FindOneAndUpdate().
			SetReturnDocument(options.After)

		retryErr := r.collection.
			FindOneAndUpdate(
				ctx,
				filter,
				update,
				retryOptions,
			).
			Decode(&user)

		if retryErr == nil {
			return &user, nil
		}

		if errors.Is(retryErr, mongo.ErrNoDocuments) {
			// Docker may seed an ADMIN account by verified email before its
			// first Google login. Atomically attach the Google identity while
			// preserving the preassigned role.
			claimOptions := options.FindOneAndUpdate().
				SetReturnDocument(options.After)

			claimErr := r.collection.FindOneAndUpdate(
				ctx,
				bson.M{
					"email": profile.Email,
					"$or": bson.A{
						bson.M{"google_id": bson.M{"$exists": false}},
						bson.M{"google_id": ""},
					},
				},
				bson.M{
					"$set": bson.M{
						"google_id":      profile.GoogleID,
						"email_verified": profile.EmailVerified,
						"name":           profile.Name,
						"picture":        profile.Picture,
						"last_login_at":  now,
						"updated_at":     now,
					},
				},
				claimOptions,
			).Decode(&user)
			if claimErr == nil {
				return &user, nil
			}
			if errors.Is(claimErr, mongo.ErrNoDocuments) ||
				mongo.IsDuplicateKeyError(claimErr) {
				return nil, fmt.Errorf(
					"%w: email is already used by another user",
					ErrUserIdentityConflict,
				)
			}

			return nil, fmt.Errorf(
				"claim seeded user identity: %w",
				claimErr,
			)
		}

		return nil, fmt.Errorf(
			"retry upsert Google user: %w",
			retryErr,
		)
	}

	return nil, fmt.Errorf(
		"upsert Google user: %w",
		err,
	)
}

func (r *UserRepository) findOne(
	ctx context.Context,
	filter interface{},
) (*models.User, error) {
	var user models.User

	err := r.collection.
		FindOne(ctx, filter).
		Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"find user: %w",
			err,
		)
	}

	return &user, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(
		strings.TrimSpace(email),
	)
}
