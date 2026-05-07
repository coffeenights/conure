package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/coffeenights/conure/cmd/api-server/database"
)

type Handler struct {
	MongoDB *database.MongoDB
}

func NewHandler(mongo *database.MongoDB) *Handler {
	return &Handler{MongoDB: mongo}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if h.MongoDB == nil || h.MongoDB.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "mongodb client not initialized"})
		return
	}
	if err := h.MongoDB.Client.Ping(ctx, nil); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func GenerateRoutes(r *gin.Engine, handler *Handler) {
	r.GET("/healthz", handler.Healthz)
	r.GET("/readyz", handler.Readyz)
}
