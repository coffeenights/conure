package auth

import (
	"log"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/internal/config"
)

func CreateSuperUser() {
	conf := config.LoadConfig(apiConfig.Config{})
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		panic(err)
	}

	email := "admin@conure.io"
	password := GenerateRandomPassword(10)
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		panic(err)
	}

	user := User{
		Email:    email,
		Password: hashedPassword,
	}
	err = user.Create(mongo)
	if err != nil {
		panic(err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Super user created")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
}
