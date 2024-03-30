package routes

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	"github.com/coffeenights/conure/internal/config"
)

func GenerateRouter() *gin.Engine {
	conf := config.LoadConfig(apiConfig.Config{})
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		panic(err)
	}

	var keyStorage variables.SecretKeyStorage
	if conf.AESStorageStrategy == "local" {
		keyStorage = variables.NewLocalSecretKey("secret.key")
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), getCorsMiddleware(conf.CorsOrigins))
	appHandler := apps.NewApiHandler()
	authHandler := auth.NewAuthHandler(conf, mongo)
	variablesHandler := variables.NewVariablesHandler(conf, mongo, keyStorage)
	apps.GenerateRoutes("/organizations", router, appHandler)
	auth.GenerateRoutes("/auth", router, authHandler)
	variables.GenerateRoutes("/variables", router, variablesHandler)
	return router
}

func getCorsMiddleware(origins string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{origins},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowCredentials: true,
	})
}
