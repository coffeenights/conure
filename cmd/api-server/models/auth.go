package models

import (
	"context"
	"errors"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

const UserCollection string = "users"

// EnsureUserIndexes installs the indexes the users collection depends on.
// Idempotent — Mongo silently no-ops a second CreateOne for the same spec.
// Called once at server startup; we deliberately don't lazy-create at write
// time because that would race when concurrent inserts each try to install
// the index, and would also hide schema setup behind the request path.
func EnsureUserIndexes(ctx context.Context, db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(UserCollection)
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_email"),
	})
	return err
}

// Role enumerates the two coarse-grained authorization tiers exposed by the
// API. Admin is the superuser tier — sees and mutates everything across
// organizations. Developer is the regular org-scoped tier — reads everything
// inside its OrganizationID, writes only resources it created.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type User struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email          string             `bson:"email" json:"email"`
	Password       string             `bson:"password" json:"-"`
	IsActive       bool               `bson:"isActive" json:"is_active"`
	LastLoginAt    *time.Time         `bson:"lastLoginAt,omitempty" json:"last_login_at"`
	CreatedAt      time.Time          `bson:"createdAt" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updated_at"`
	Client         string             `bson:"client,omitempty" json:"client"`
	Role           Role               `bson:"role,omitempty" json:"role"`
	OrganizationID primitive.ObjectID `bson:"organizationId,omitempty" json:"organization_id,omitempty"`
}

// IsAdmin is shorthand used by authorization middleware and handlers.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

func (u *User) Create(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(UserCollection)
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	u.IsActive = true

	insertResult, err := collection.InsertOne(context.Background(), u)
	if err != nil {
		// The unique-email index turns a race into a clean error here
		// rather than a phantom duplicate. The previous check-then-insert
		// race could pass both checks and then succeed twice; with the
		// index, exactly one insert wins and the other gets 11000.
		var writeException mongo.WriteException
		if errors.As(err, &writeException) {
			for _, we := range writeException.WriteErrors {
				if we.Code == 11000 {
					return conureerrors.ErrEmailAlreadyExists
				}
			}
		}
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

func (u *User) UpdateLastLoginAt(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	now := time.Now()
	u.LastLoginAt = &now
	filter := bson.M{"_id": u.ID}
	update := bson.M{"$set": bson.M{"lastLoginAt": u.LastLoginAt}}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (u *User) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	filter := bson.M{"_id": u.ID}
	_, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	return nil
}

// Update writes mutable fields (email, role, organization, isActive) back to
// the document. Password is intentionally excluded — use UpdatePassword.
// A duplicate-key error from the unique email index is translated to
// ErrEmailAlreadyExists so handlers can surface a stable 400 instead of a
// generic 500.
func (u *User) Update(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(UserCollection)
	u.UpdatedAt = time.Now()
	filter := bson.M{"_id": u.ID}
	update := bson.M{"$set": bson.M{
		"email":          u.Email,
		"role":           u.Role,
		"organizationId": u.OrganizationID,
		"isActive":       u.IsActive,
		"updatedAt":      u.UpdatedAt,
	}}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		var writeException mongo.WriteException
		if errors.As(err, &writeException) {
			for _, we := range writeException.WriteErrors {
				if we.Code == 11000 {
					return conureerrors.ErrEmailAlreadyExists
				}
			}
		}
		return err
	}
	return nil
}

// ListUsers returns every user row. Used by the admin user-management
// endpoints; not exposed to developers.
func ListUsers(mongo *database.MongoDB) ([]User, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(UserCollection)
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	var users []User
	for cursor.Next(context.Background()) {
		var u User
		if err := cursor.Decode(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, cursor.Err()
}

func ValidateEmail(email string) error {
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	if matched, err := regexp.MatchString(pattern, email); err != nil {
		return conureerrors.ErrInvalidEmail
	} else if !matched {
		return conureerrors.ErrInvalidEmail
	}
	return nil
}

func ValidatePasswords(password string, password2 string) error {
	if len(password) < 8 {
		return conureerrors.ErrInvalidPassword
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return conureerrors.ErrInvalidPassword
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return conureerrors.ErrInvalidPassword
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return conureerrors.ErrInvalidPassword
	}

	if password != password2 {
		return conureerrors.ErrPasswordConfirmationMismatch
	}
	return nil
}
