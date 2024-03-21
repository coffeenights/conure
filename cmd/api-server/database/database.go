package database

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type MongoDB struct {
	Client *mongo.Client
	DBName string
	err    mongo.WriteException
}

func ConnectToMongoDB(uri string, dbName string) (*MongoDB, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)
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

	log.Println("Connected to MongoDB!")
	return &MongoDB{Client: client, DBName: dbName}, nil
}
