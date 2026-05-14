package applications

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/config"
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
	appConfig := config.LoadConfig(apiConfig.Config{})
	appConfig.MongoDBName = appConfig.MongoDBName + "-test-applications"
	db, err := database.ConnectToMongoDB(appConfig.MongoDBURI, appConfig.MongoDBName)
	if err != nil {
		log.Panic(err)
	}
	app := NewApiHandler(appConfig, db, nil, nil)
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
	if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
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
	// GenerateToken takes a time.Duration. The previous bare `3600` was
	// interpreted as 3600 nanoseconds, which meant the token expired in
	// the same Unix second it was issued — tests would pass or fail based
	// on whether the clock ticked over before the request was served.
	testConf.JWT, err = auth.GenerateToken(time.Hour, payload, testConf.app.Config.JWTSecret)
	if err != nil {
		log.Panic(err)
	}
}

func teardown() {
	// Drop the whole test database rather than per-row cleanup. The suite
	// is the sole writer to `<dbname>-test-applications`; leaving rows
	// behind across runs causes flakes when tests insert by unique fields
	// (notably the users.email index) and the runner reuses the DB.
	if testConf.app != nil && testConf.app.MongoDB != nil {
		_ = testConf.app.MongoDB.Client.Database(testConf.app.MongoDB.DBName).Drop(context.Background())
		_ = testConf.app.MongoDB.Client.Disconnect(context.Background())
	}
}
