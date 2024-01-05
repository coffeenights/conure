package applications

import (
	"context"
	"fmt"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Status    string             `bson:"status,omitempty"`
	AccountId string             `bson:"accountId,omitempty"`
	Name      string             `bson:"name,omitempty"`
}

const (
	OrganizationCollection = "organizations"
)

func (o *Organization) String() string {
	return fmt.Sprintf("Organization: %s, %s", o.Status, o.AccountId)
}

func (o *Organization) Create(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	insertResult, err := collection.InsertOne(context.Background(), o)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Inserted a single document: ", insertResult.InsertedID)
	return nil
}

func (o *Organization) GetById(mongo *database.MongoDB, Id string) *Organization {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	err := collection.FindOne(context.Background(), bson.M{"accountid": Id}).Decode(o)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Found a single document: ", o)
	return o
}

func (o *Organization) Update(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.D{{"accountid", o.AccountId}}
	update := bson.D{
		{"$set", bson.D{
			{"status", o.Status},
			{"accountId", o.AccountId},
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
	filter := bson.D{{"accountid", o.AccountId}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Deleted %v documents in the organizations collection\n", deleteResult.DeletedCount)
	return nil
}

//func (o *Organization) Migrate(client *mongo.Client) error {
//	log.Println("Migrating MongoDB")
//	coll := client.Database(mongo.DBName).Collection(OrganizationCollection)
//	validator := bson.M{
//		"$jsonSchema": bson.M{
//			"bsonType": "object",
//			"required": []string{"field1", "field2"},
//			"properties": bson.M{
//				"field1": bson.M{
//					"bsonType":    "string",
//					"description": "must be a string and is required",
//				},
//				"field2": bson.M{
//					"bsonType":    "int",
//					"description": "must be an integer and is required",
//				},
//			},
//		},
//	}
//	opts := options.CreateCollection().SetValidator(validator)
//	err := client.Database(mongo.DBName).RunCommand(context.Background(), bson.D{{"create", OrganizationCollection}, {"validator", validator}}).Err()
//	if err != nil {
//		// Handle error
//	}
//}
