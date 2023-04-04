package models

import (
	"gorm.io/gorm"
	"log"
	"time"
)

type User struct {
	gorm.Model
	Name       string
	Email      string
	Password   string
	AuthTokens []AuthToken
}

type AuthToken struct {
	Token     string `gorm:"primarykey"`
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	UserId    uint
}

func Migrate(db *gorm.DB) {
	err := db.AutoMigrate(&User{})
	err = db.AutoMigrate(&AuthToken{})
	if err != nil {
		log.Fatalf("Error during migration: %s", err)
	}
}
