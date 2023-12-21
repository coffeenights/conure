package auth

//import (
//	"github.com/gin-gonic/gin"
//
//	c "github.com/coffeenights/conure/cmd/api-server/auth/controllers"
//	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
//	"github.com/coffeenights/conure/internal/config"
//)
//
//func GenerateRoutes(relativePath string, r *gin.Engine) {
//	cfg := config.LoadConfig(apiConfig.Config{})
//	handler := c.NewAuthHandler(cfg)
//
//	auth := r.Group(relativePath)
//	{
//		auth.POST("auth/register/", handler.CreateUserHandler)
//		auth.GET("auth/verify-email/", handler.VerifyEmailHandler)
//		auth.POST("auth/login/", handler.LoginHandler)
//		auth.POST("auth/reset-password/", handler.ResetPasswordHandler)
//		auth.POST("auth/reset-password-confirmation/", handler.ResetPasswordConfirmationHandler)
//	}
//}
