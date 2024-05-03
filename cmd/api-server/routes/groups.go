package routes

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log"
	"strings"

	apps "github.com/coffeenights/conure/cmd/api-server/applications"
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	"github.com/coffeenights/conure/internal/config"
)

func GenerateRouter() *gin.Engine {
	conf := config.LoadConfig(apiConfig.Config{})
	log.Println("Connecting to MongoDB")
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to MongoDB")
	var keyStorage variables.SecretKeyStorage
	if conf.AESStorageStrategy == "local" {
		keyStorage = variables.NewLocalSecretKey("secret.key")
		_, err := keyStorage.Load()
		if err != nil {
			err = keyStorage.Generate()
			if err != nil {
				log.Panic(err)
			}
		}
	}

	router := gin.New()
	router.RedirectTrailingSlash = false
	router.Use(gin.Logger(), gin.Recovery(), getCorsMiddleware())
	appHandler := apps.NewApiHandler(conf, mongo)
	authHandler := auth.NewAuthHandler(conf, mongo)
	variablesHandler := variables.NewVariablesHandler(conf, mongo, keyStorage)
	apps.GenerateRoutes("/organizations", router, appHandler)
	auth.GenerateRoutes("/auth", router, authHandler)
	variables.GenerateRoutes("/variables", router, variablesHandler)
	return router
}

func allowOrigin(origin string) bool {
	conf := config.LoadConfig(apiConfig.Config{})
	origins := strings.Split(conf.CorsOrigins, ";")
	for _, o := range origins {
		if o == origin || o == "*" {
			return true
		}
	}
	return true
}

func getCorsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		AllowCredentials: true,
		AllowOriginFunc:  allowOrigin,
	})
}
