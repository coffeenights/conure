package database

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type MongoDB struct {
	Client *mongo.Client
}

func ConnectToMongoDB() (*MongoDB, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	ctx := context.Background()
	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")
	return &MongoDB{Client: client}, nil
}
