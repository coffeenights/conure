package applications

import (
	"context"
	"fmt"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Status    OrganizationStatus `bson:"status"`
	AccountID string             `bson:"accountId"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"createdAt"`
	DeletedAt time.Time          `bson:"deletedAt,omitempty"`
}

type OrganizationStatus string

const (
	OrganizationCollection string             = "organizations"
	OrgActive              OrganizationStatus = "active"
	OrgDeleted             OrganizationStatus = "deleted"
	OrgDisabled            OrganizationStatus = "disabled"
)

func (o *Organization) String() string {
	return fmt.Sprintf("Organization: %s, %s", o.Status, o.AccountID)
}

func (o *Organization) Create(mongo *database.MongoDB) (string, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	o.CreatedAt = time.Now()
	o.Status = OrgActive
	insertResult, err := collection.InsertOne(context.Background(), o)
	if err != nil {
		return "", err
	}
	oID := insertResult.InsertedID.(primitive.ObjectID)
	log.Println("Inserted a single document: ", oID.Hex())
	return oID.Hex(), nil
}

func (o *Organization) GetById(mongo *database.MongoDB, Id string) (*Organization, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	oID, _ := primitive.ObjectIDFromHex(Id)
	filter := bson.M{"_id": oID, "status": bson.M{"$ne": OrgDeleted}}
	err := collection.FindOne(context.Background(), filter).Decode(o)
	if err != nil {
		return nil, err
	}
	log.Println("Found a single document: ", o)
	return o, nil
}

func (o *Organization) Update(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.M{"accountId": o.AccountID, "status": bson.M{"$ne": OrgDeleted}}
	update := bson.D{
		{"$set", bson.D{
			{"status", o.Status},
			{"accountId", o.AccountID},
			{"name", o.Name},
		}},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

func (o *Organization) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.D{{"accountId", o.AccountID}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Deleted %v documents in the organizations collection\n", deleteResult.DeletedCount)
	return nil
}

func (o *Organization) SoftDelete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.D{{"accountId", o.AccountID}}
	update := bson.D{
		{"$set", bson.D{
			{"status", OrgDeleted},
			{"deletedAt", time.Now()},
		}},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Matched %v documents and deleted %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}
