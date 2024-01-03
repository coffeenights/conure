package applications

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Status    string             `bson:"status,omitempty"`
	AccountId string             `bson:"accountId,omitempty"`
}

func main() {
	// Set client options
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	collection := client.Database("test").Collection("organizations")

	org := Organization{Status: "Active", AccountId: "12345"}

	// Create
	insertResult, err := collection.InsertOne(context.TODO(), org)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Inserted a single document: ", insertResult.InsertedID)

	// Read
	var result Organization
	err = collection.FindOne(context.TODO(), bson.M{"accountid": "12345"}).Decode(&result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Found a single document: ", result)

	// Update
	filter := bson.D{{"accountid", "12345"}}
	update := bson.D{
		{"$set", bson.D{
			{"status", "Inactive"},
		}},
	}
	updateResult, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)

	// Delete
	deleteResult, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Deleted %v documents in the organizations collection\n", deleteResult.DeletedCount)
}
