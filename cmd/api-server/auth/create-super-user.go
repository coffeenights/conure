package auth

import (
	"errors"
	"log"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func CreateSuperuser(mongo *database.MongoDB, email, password string) {
	client := "conure"
	if password == "" {
		password = GenerateRandomPassword(10)
	}
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		log.Panic(err)
	}

	user := models.User{
		Email:    email,
		Password: hashedPassword,
		Client:   client,
	}
	err = user.Create(mongo)
	if err != nil {
		if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
			log.Printf("Superuser %s already exists, skipping creation", email)
			return
		}
		log.Panic(err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser created")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
}

func ResetSuperuserPassword(mongo *database.MongoDB, email string) {
	client := "conure"
	password := GenerateRandomPassword(10)
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		panic(err)
	}

	user := models.User{
		Email:    email,
		Password: hashedPassword,
		Client:   client,
	}
	err = user.GetByEmail(mongo, email)
	if err != nil {
		log.Panic(err)
	}
	err = user.UpdatePassword(mongo, hashedPassword)
	if err != nil {
		log.Panic(err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser password reset")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
}
