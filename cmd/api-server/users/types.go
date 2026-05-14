package users

import (
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

type UserResponse struct {
	*models.User
	// Role is repeated as the effective role (post-fallback) so clients
	// don't have to know about the legacy client="conure" sentinel.
	Role models.Role `json:"role"`
}

func toUserResponse(u models.User) UserResponse {
	return UserResponse{User: &u, Role: u.EffectiveRole()}
}

type UserListResponse struct {
	Users []UserResponse `json:"users"`
}
