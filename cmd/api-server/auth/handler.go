package auth

import (
	"net/http"

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
	c.JSON(http.StatusOK, gin.H{"message": "Login"})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Me"})
}
