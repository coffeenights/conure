package models

import (
	"context"
	"errors"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type Model struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt time.Time          `bson:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty"`
	DeleteAt  time.Time          `bson:"deletedAt,omitempty"`
}

func (c *Model) SetCreatedAt(t time.Time) {
	c.CreatedAt = t
}

func (c *Model) SetDeletedAt() {
	c.DeleteAt = time.Now()
}

func (c *Model) SetID(id primitive.ObjectID) {
	c.ID = id
}

func (c *Model) GetID() primitive.ObjectID {
	return c.ID
}

type ModelInterface interface {
	GetCollectionName() string
	SetCreatedAt(time.Time)
	SetID(primitive.ObjectID)
	GetID() primitive.ObjectID
}

func GetByID(ctx context.Context, db *database.MongoDB, ID string, model ModelInterface) error {
	collection := db.Client.Database(db.DBName).Collection(model.GetCollectionName())
	oID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": oID, "deletedAt": bson.M{"$exists": false}}
	err = collection.FindOne(ctx, filter).Decode(model)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	} else if err != nil {
		return err
	}
	log.Println("Found a single document: ", model)
	return nil
}

func Create(ctx context.Context, db *database.MongoDB, model ModelInterface) error {
	collection := db.Client.Database(db.DBName).Collection(model.GetCollectionName())
	model.SetCreatedAt(time.Now())
	insertResult, err := collection.InsertOne(ctx, model)
	if err != nil {
		return err
	}
	model.SetID(insertResult.InsertedID.(primitive.ObjectID))
	log.Println("Inserted a single document: ", insertResult.InsertedID.(primitive.ObjectID).Hex())
	return nil
}

func Update(ctx context.Context, db *database.MongoDB, model ModelInterface) error {
	collection := db.Client.Database(db.DBName).Collection(model.GetCollectionName())
	filter := bson.M{"_id": model.GetID()}
	update := bson.D{
		{"$set", model},
	}
	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

func Delete(ctx context.Context, db *database.MongoDB, model ModelInterface) error {
	collection := db.Client.Database(db.DBName).Collection(model.GetCollectionName())
	filter := bson.D{{"_id", model.GetID()}}
	deleteResult, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	log.Printf("Deleted %v documents in the components collection\n", deleteResult.DeletedCount)
	return nil
}

func SoftDelete(ctx context.Context, db *database.MongoDB, model ModelInterface) error {
	collection := db.Client.Database(db.DBName).Collection(model.GetCollectionName())
	filter := bson.D{{"_id", model.GetID()}}
	update := bson.D{
		{"$set", bson.D{
			{"deletedAt", time.Now()},
		}},
	}
	updateResult, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and deleted %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}
