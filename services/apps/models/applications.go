package models

import (
	"gorm.io/gorm"
	"log"
	"time"
)

type BaseModel struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Application struct {
	BaseModel
	Name        string
	Description string
	AccountId   uint64 `gorm:"index"`
}

func Migrate(db *gorm.DB) {
	err := db.AutoMigrate(&Application{})
	if err != nil {
		log.Fatalf("Error during migration: %s", err)
	}
}
