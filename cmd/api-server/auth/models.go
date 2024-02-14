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
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email       string             `bson:"email" json:"email"`
	Password    string             `bson:"password" json:"-"`
	IsActive    bool               `bson:"isActive" json:"is_active"`
	LastLoginAt *time.Time         `bson:"lastLoginAt,omitempty" json:"last_login_at"`
	CreatedAt   time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updated_at"`
	Client      string             `bson:"client,omitempty" json:"client"`
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

func (u *User) UpdatePassword(mongo *database.MongoDB, password string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	u.UpdatedAt = time.Now()
	u.Password = password
	filter := bson.M{"_id": u.ID}
	update := bson.M{"$set": bson.M{"password": u.Password, "updatedAt": u.UpdatedAt}}
	_, err := collection.UpdateOne(context.Background(), filter, update)
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

func ValidatePasswords(password string, password2 string) error {
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

	if password != password2 {
		return ErrPasswordsNotMatch
	}
	return nil
}
