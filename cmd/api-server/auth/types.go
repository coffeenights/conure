package auth

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	Password    string `json:"password" binding:"required"`
	Password2   string `json:"password2" binding:"required"`
}

// UpdateMeRequest covers the fields a logged-in user is allowed to mutate
// on their own account. Role and OrganizationID are not in here — those are
// admin-only and live behind /users/:id.
type UpdateMeRequest struct {
	Email *string `json:"email,omitempty"`
}
