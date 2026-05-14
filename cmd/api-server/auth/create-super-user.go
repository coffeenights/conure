package auth

import (
	"errors"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func CreateSuperuser(db *database.MongoDB, email, password string) error {
	if password == "" {
		password = GenerateRandomPassword(10)
	}
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		Email:    email,
		Password: hashedPassword,
		Role:     models.RoleAdmin,
	}
	if err := user.Create(db); err != nil {
		if errors.Is(err, conureerrors.ErrEmailAlreadyExists) {
			log.Printf("Superuser %s already exists, skipping creation", email)
			return nil
		}
		return fmt.Errorf("create superuser %s: %w", email, err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser created")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	return nil
}

func ResetSuperuserPassword(db *database.MongoDB, email, password string) error {
	if password == "" {
		password = GenerateRandomPassword(10)
	}
	hashedPassword, err := GenerateFromPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := models.User{}
	if err := user.GetByEmail(db, email); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("superuser %s not found", email)
		}
		return fmt.Errorf("lookup superuser %s: %w", email, err)
	}
	if err := user.UpdatePassword(db, hashedPassword); err != nil {
		return fmt.Errorf("update password for %s: %w", email, err)
	}

	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	log.Println("Superuser password reset")
	log.Println("Email:", email)
	log.Println("Password:", password)
	log.Println("x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x")
	return nil
}
