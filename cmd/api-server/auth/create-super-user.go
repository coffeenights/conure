package auth

import (
	"log"

	"github.com/coffeenights/conure/cmd/api-server/database"
)

func CreateSuperuser(mongo *database.MongoDB) {
	client := "conure"
	email := "admin@conure.io"
	password := GenerateRandomPassword(10)
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		panic(err)
	}

	user := User{
		Email:    email,
		Password: hashedPassword,
		Client:   client,
	}
	err = user.Create(mongo)
	if err != nil {
		panic(err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser created")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
}

func ResetSuperuserPassword(mongo *database.MongoDB) {
	client := "conure"
	email := "admin@conure.io"
	password := GenerateRandomPassword(10)
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		panic(err)
	}

	user := User{
		Email:    email,
		Password: hashedPassword,
		Client:   client,
	}
	err = user.GetByEmail(mongo, email)
	if err != nil {
		panic(err)
	}
	err = user.UpdatePassword(mongo, hashedPassword)
	if err != nil {
		panic(err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser password reset")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
}
