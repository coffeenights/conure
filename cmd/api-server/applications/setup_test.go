package applications

import (
	"context"
	"errors"
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/config"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"net/http"
	"os"
	"testing"
)

type testConfig struct {
	app      *ApiHandler
	router   *gin.Engine
	authUser *models.User
	password string
	JWT      string
}

var testConf testConfig

func (tc *testConfig) generateCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "auth",
		Value:    tc.JWT,
		MaxAge:   3600,
		Path:     "/",
		Domain:   tc.app.Config.FrontendDomain,
		Secure:   tc.app.Config.CookieSecure,
		HttpOnly: true,
	}
}

func setupRouter() (*gin.Engine, *ApiHandler) {
	router := gin.Default()
	db, err := models.SetupDB()
	appConfig := config.LoadConfig(apiConfig.Config{})
	appConfig.MongoDBName = appConfig.MongoDBName + "-test"
	if err != nil {
		log.Panic(err)
	}
	app := NewApiHandler(appConfig, db)
	GenerateRoutes("/organizations", router, app)
	return router, app
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	router, app := setupRouter()
	testConf.app = app
	testConf.router = router

	// Create test user
	client := "conure"
	email := "admin@conure.io"
	password := auth.GenerateRandomPassword(10)
	hashedPassword, err := auth.GenerateFromPassword(password)
	if err != nil {
		log.Panic(err)
	}

	user := models.User{
		Email:    email,
		Password: hashedPassword,
		Client:   client,
	}
	err = user.Create(app.MongoDB)
	if errors.Is(err, models.ErrEmailExists) {
		err = user.GetByEmail(app.MongoDB, email)
		if err != nil {
			log.Panic(err)
		}
	} else if err != nil {
		log.Panic(err)
	}

	testConf.authUser = &user
	testConf.password = password

	payload := auth.JWTData{
		Email:  user.Email,
		Client: user.Client,
	}
	testConf.JWT, err = auth.GenerateToken(3600, payload, testConf.app.Config.JWTSecret)
}

func teardown() {
	err := testConf.authUser.Delete(testConf.app.MongoDB)
	if err != nil {
		log.Panic(err)
	}
	_ = testConf.app.MongoDB.Client.Disconnect(context.Background())
}
