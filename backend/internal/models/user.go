package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

func (r UserRole) IsValid() bool {
	switch r {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

type User struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	GoogleID      string `bson:"google_id" json:"google_id"`
	Email         string `bson:"email" json:"email"`
	EmailVerified bool   `bson:"email_verified" json:"email_verified"`

	Name    string `bson:"name" json:"name"`
	Picture string `bson:"picture,omitempty" json:"picture,omitempty"`

	Role UserRole `bson:"role" json:"role"`

	LastLoginAt time.Time `bson:"last_login_at" json:"last_login_at"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

// GoogleUserProfile คือข้อมูลที่ได้หลังจาก Backend
// ตรวจสอบ Google ID Token สำเร็จแล้ว
type GoogleUserProfile struct {
	GoogleID      string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// UserResponse ใช้ส่งข้อมูลผู้ใช้กลับไปยัง Frontend
// โดยไม่จำเป็นต้องเปิดเผย GoogleID
type UserResponse struct {
	ID primitive.ObjectID `json:"id"`

	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`

	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`

	Role UserRole `json:"role"`
}

func NewUserResponse(user *User) UserResponse {
	if user == nil {
		return UserResponse{}
	}

	return UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		Picture:       user.Picture,
		Role:          user.Role,
	}
}
