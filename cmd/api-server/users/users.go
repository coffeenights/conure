package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func (h *Handler) List(c *gin.Context) {
	users, err := models.ListUsers(h.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	resp := UserListResponse{Users: make([]UserResponse, len(users))}
	for i := range users {
		resp.Users[i] = toUserResponse(users[i])
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	u := models.User{}
	if err := u.GetById(h.MongoDB, c.Param("userID")); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
			return
		}
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(u))
}

func (h *Handler) Create(c *gin.Context) {
	req := CreateUserRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if err := models.ValidateEmail(req.Email); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	// Password is required at creation — duplicates the password rules used
	// for self-service change-password so admins can't bypass them.
	if err := models.ValidatePasswords(req.Password, req.Password); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	hashed, err := auth.GenerateFromPassword(req.Password)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	role := req.Role
	if role == "" {
		role = models.RoleDeveloper
	}
	if role != models.RoleAdmin && role != models.RoleDeveloper {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	var orgID primitive.ObjectID
	if req.OrganizationID != "" {
		orgID, err = primitive.ObjectIDFromHex(req.OrganizationID)
		if err != nil {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
	}

	u := models.User{
		Email:          req.Email,
		Password:       hashed,
		Role:           role,
		OrganizationID: orgID,
	}
	if err := u.Create(h.MongoDB); err != nil {
		if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
			conureerrors.AbortWithError(c, err)
			return
		}
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusCreated, toUserResponse(u))
}

func (h *Handler) Update(c *gin.Context) {
	u := models.User{}
	if err := u.GetById(h.MongoDB, c.Param("userID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	req := UpdateUserRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if req.Email != nil {
		if err := models.ValidateEmail(*req.Email); err != nil {
			conureerrors.AbortWithError(c, err)
			return
		}
		u.Email = *req.Email
	}
	if req.Role != nil {
		if *req.Role != models.RoleAdmin && *req.Role != models.RoleDeveloper {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		u.Role = *req.Role
	}
	if req.OrganizationID != nil {
		if *req.OrganizationID == "" {
			u.OrganizationID = primitive.NilObjectID
		} else {
			oid, err := primitive.ObjectIDFromHex(*req.OrganizationID)
			if err != nil {
				conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
				return
			}
			u.OrganizationID = oid
		}
	}
	if req.IsActive != nil {
		u.IsActive = *req.IsActive
	}
	if err := u.Update(h.MongoDB); err != nil {
		// Pass conure errors (e.g. ErrEmailAlreadyExists) through with
		// their original status, fall back to generic 500 otherwise.
		if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
			conureerrors.AbortWithError(c, err)
			return
		}
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(u))
}

// Delete hard-deletes the user. Admins shouldn't be able to delete themselves
// — that would leave the system without a superuser if the deleter happens
// to be the last admin.
func (h *Handler) Delete(c *gin.Context) {
	current := c.MustGet("currentUser").(models.User)
	u := models.User{}
	if err := u.GetById(h.MongoDB, c.Param("userID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if u.ID == current.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrNotAllowed)
		return
	}
	if err := u.Delete(h.MongoDB); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// ResetPassword sets a new password chosen by the admin (or random when
// omitted) and returns the new plaintext password in the response. There is
// no old-password check — that's the whole point of an admin reset.
func (h *Handler) ResetPassword(c *gin.Context) {
	u := models.User{}
	if err := u.GetById(h.MongoDB, c.Param("userID")); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	req := ResetPasswordRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	password := req.Password
	if password == "" {
		password = auth.GenerateRandomPassword(12)
	} else if err := models.ValidatePasswords(password, password); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	hashed, err := auth.GenerateFromPassword(password)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}
	if err := u.UpdatePassword(h.MongoDB, hashed); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusOK, ResetPasswordResponse{Password: password})
}
