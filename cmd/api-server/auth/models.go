package auth

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/database"
)

const UserCollection string = "users"

type User struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Email       string             `bson:"email"`
	Password    string             `bson:"password"`
	IsActive    bool               `bson:"isActive"`
	LastLoginAt *time.Time         `bson:"lastLoginAt,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
}

func (u *User) Create(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	u.IsActive = true

	// check if email exists
	filter := bson.M{"email": u.Email}
	err := collection.FindOne(context.Background(), filter).Decode(u)
	if err == nil {
		return ErrEmailExists
	}

	insertResult, err := collection.InsertOne(context.Background(), u)
	if err != nil {
		return err
	}
	u.ID = insertResult.InsertedID.(primitive.ObjectID)
	return nil
}

func (u *User) GetById(mongo *database.MongoDB, id string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	IDHex, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": IDHex, "isActive": true}
	err := collection.FindOne(context.Background(), filter).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *User) GetByEmail(mongo *database.MongoDB, email string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	filter := bson.M{"email": email, "isActive": true}
	err := collection.FindOne(context.Background(), filter).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func ValidateEmail(email string) error {
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	if matched, err := regexp.MatchString(pattern, email); err != nil {
		return ErrEmailNotValid
	} else if !matched {
		return ErrEmailNotValid
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordNotValid
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return ErrPasswordNotValid
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return ErrPasswordNotValid
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return ErrPasswordNotValid
	}

	return nil
}
