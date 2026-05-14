package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		user       models.User
		expectCode int
	}{
		{
			name:       "explicit admin role passes",
			user:       models.User{ID: primitive.NewObjectID(), Role: models.RoleAdmin},
			expectCode: http.StatusOK,
		},
		{
			name:       "legacy superuser client sentinel still passes",
			user:       models.User{ID: primitive.NewObjectID(), Client: models.SuperuserClient},
			expectCode: http.StatusOK,
		},
		{
			name:       "developer is rejected",
			user:       models.User{ID: primitive.NewObjectID(), Role: models.RoleDeveloper},
			expectCode: http.StatusForbidden,
		},
		{
			name:       "missing role defaults to developer and is rejected",
			user:       models.User{ID: primitive.NewObjectID()},
			expectCode: http.StatusForbidden,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("currentUser", tc.user)
				c.Next()
			}, RequireAdmin())
			router.GET("/", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{})
			})
			req, _ := http.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.expectCode {
				t.Errorf("expected %d, got %d", tc.expectCode, w.Code)
			}
		})
	}
}
