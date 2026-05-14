package users

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

type CreateUserRequest struct {
	Email          string      `json:"email" binding:"required,email"`
	Password       string      `json:"password" binding:"required"`
	Role           models.Role `json:"role"`
	OrganizationID string      `json:"organization_id"`
}

type UpdateUserRequest struct {
	// Pointers so the handler can distinguish "leave alone" from "set empty".
	Email          *string      `json:"email,omitempty"`
	Role           *models.Role `json:"role,omitempty"`
	OrganizationID *string      `json:"organization_id,omitempty"`
	IsActive       *bool        `json:"is_active,omitempty"`
}

type ResetPasswordRequest struct {
	// Password is optional — server generates a random one when empty, and
	// returns it in the response so the admin can hand it off.
	Password string `json:"password"`
}

type ResetPasswordResponse struct {
	Password string `json:"password"`
}

// UserResponse is the explicit wire format for users. We deliberately do
// not embed models.User — that would leak any internal field added later
// (the legacy Client column, password hash, anything else) into the API.
// Adding a new public field is a conscious, one-line change here.
type UserResponse struct {
	ID             primitive.ObjectID `json:"id"`
	Email          string             `json:"email"`
	Role           models.Role        `json:"role"`
	OrganizationID primitive.ObjectID `json:"organization_id,omitempty"`
	IsActive       bool               `json:"is_active"`
	LastLoginAt    *time.Time         `json:"last_login_at,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func toUserResponse(u models.User) UserResponse {
	return UserResponse{
		ID:             u.ID,
		Email:          u.Email,
		Role:           u.Role,
		OrganizationID: u.OrganizationID,
		IsActive:       u.IsActive,
		LastLoginAt:    u.LastLoginAt,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}

type UserListResponse struct {
	Users []UserResponse `json:"users"`
}
