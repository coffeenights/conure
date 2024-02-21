package auth

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := User{}
	err = user.GetByEmail(h.MongoDB, loginRequest.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": ErrEmailPasswordValid.Error()})
		return
	}

	matched, err := ComparePasswordAndHash(loginRequest.Password, user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !matched {
		c.JSON(http.StatusUnauthorized, gin.H{"error": ErrEmailPasswordValid.Error()})
		return
	}

	payload := JWTData{
		Email:  user.Email,
		Client: user.Client,
	}
	ttl := time.Duration(h.Config.JWTExpiration) * time.Hour * 24
	jwt, err := GenerateToken(ttl, payload, h.Config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = user.UpdateLastLoginAt(h.MongoDB)
	if err != nil {
		log.Print(err)
		log.Println("Failed to update last login at")
	}

	c.JSON(http.StatusOK, gin.H{"token": jwt})
}

func (h *Handler) Me(c *gin.Context) {
	user := c.MustGet("currentUser").(User)
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	changePasswordRequest := ChangePasswordRequest{}
	err := c.ShouldBindJSON(&changePasswordRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = ValidatePasswords(changePasswordRequest.Password, changePasswordRequest.Password2)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := c.MustGet("currentUser").(User)
	matched, err := ComparePasswordAndHash(changePasswordRequest.OldPassword, user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrOldPasswordInvalid.Error()})
		return
	}

	hashedPassword, err := GenerateFromPassword(changePasswordRequest.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = user.UpdatePassword(h.MongoDB, hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
