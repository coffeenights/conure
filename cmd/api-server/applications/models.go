package applications

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Status    string             `bson:"status,omitempty"`
	AccountId string             `bson:"accountId,omitempty"`
	Name      string             `bson:"name,omitempty"`
}

func (o *Organization) String() string {
	return fmt.Sprintf("Organization: %s, %s", o.Status, o.AccountId)
}

func (o *Organization) Create(client *mongo.Client) error {
	collection := client.Database("test").Collection("organizations")
	insertResult, err := collection.InsertOne(context.Background(), o)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Inserted a single document: ", insertResult.InsertedID)
	return nil
}

func (o *Organization) GetById(client *mongo.Client, Id string) *Organization {
	collection := client.Database("test").Collection("organizations")
	err := collection.FindOne(context.Background(), bson.M{"accountid": Id}).Decode(o)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Found a single document: ", o)
	return o
}

func (o *Organization) Update(client *mongo.Client) error {
	collection := client.Database("test").Collection("organizations")
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

func (o *Organization) Delete(client *mongo.Client) error {
	collection := client.Database("test").Collection("organizations")
	filter := bson.D{{"accountid", o.AccountId}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Deleted %v documents in the organizations collection\n", deleteResult.DeletedCount)
	return nil
}
