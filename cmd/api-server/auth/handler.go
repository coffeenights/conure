package auth

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

type Handler struct {
	Config  *apiConfig.Config
	MongoDB *database.MongoDB
}

func NewAuthHandler(config *apiConfig.Config, mongo *database.MongoDB) *Handler {
	return &Handler{
		Config:  config,
		MongoDB: mongo,
	}
}

func (h *Handler) Login(c *gin.Context) {
	loginRequest := LoginRequest{}
	err := c.ShouldBindJSON(&loginRequest)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	user := models.User{}
	err = user.GetByEmail(h.MongoDB, loginRequest.Email)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidCredentials)
		return
	}

	matched, err := ComparePasswordAndHash(loginRequest.Password, user.Password)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if !matched {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidCredentials)
		return
	}

	payload := JWTData{
		Email:  user.Email,
		Client: user.Client,
	}
	ttl := time.Duration(h.Config.JWTExpiration) * time.Hour * 24
	jwt, err := GenerateToken(ttl, payload, h.Config.JWTSecret)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	err = user.UpdateLastLoginAt(h.MongoDB)
	if err != nil {
		log.Print(err)
		log.Println("Failed to update last login at")
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("auth", jwt, int(ttl.Seconds()), "/", h.Config.FrontendDomain, h.Config.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"token": jwt})
}

func (h *Handler) Me(c *gin.Context) {
	user := c.MustGet("currentUser").(models.User)
	c.JSON(http.StatusOK, user)
}

// UpdateMe lets a logged-in user edit fields they own — currently just
// email. Role/OrganizationID changes go through the admin endpoint.
func (h *Handler) UpdateMe(c *gin.Context) {
	req := UpdateMeRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	user := c.MustGet("currentUser").(models.User)
	if req.Email != nil {
		if err := models.ValidateEmail(*req.Email); err != nil {
			conureerrors.AbortWithError(c, err)
			return
		}
		user.Email = *req.Email
	}
	if err := user.Update(h.MongoDB); err != nil {
		if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
			conureerrors.AbortWithError(c, err)
			return
		}
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	changePasswordRequest := ChangePasswordRequest{}
	err := c.ShouldBindJSON(&changePasswordRequest)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	err = models.ValidatePasswords(changePasswordRequest.Password, changePasswordRequest.Password2)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	user := c.MustGet("currentUser").(models.User)
	matched, err := ComparePasswordAndHash(changePasswordRequest.OldPassword, user.Password)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if !matched {
		conureerrors.AbortWithError(c, conureerrors.ErrOldPasswordInvalid)
		return
	}

	hashedPassword, err := GenerateFromPassword(changePasswordRequest.Password)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}
	err = user.UpdatePassword(h.MongoDB, hashedPassword)
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrDatabaseError)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
