package applications

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
)

func setupDB() (*mongo.Client, error) {
	// Connect to your MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func TestOrganization_Create(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: "Active", AccountId: "12345"}

	err = org.Create(client)
	if err != nil {
		t.Errorf("Failed to create organization: %v", err)
	}
}

func TestOrganization_GetById(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: "Active", AccountId: "12345"}
	err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	got := org.GetById(client, org.AccountId)
	if got.AccountId != org.AccountId {
		t.Errorf("Got %v, want %v", got.AccountId, org.AccountId)
	}
}

func TestOrganization_Update(t *testing.T) {
	client, err := setupDB()
	if err != nil {
		t.Fatal(err)
	}

	org := &Organization{Status: "Active", AccountId: "12345"}
	err = org.Create(client)
	if err != nil {
		t.Fatal(err)
	}

	org.Status = "Inactive"
	err = org.Update(client)
	if err != nil {
		t.Errorf("Failed to update organization: %v", err)
	}

	got := org.GetById(client, org.AccountId)
	if got.Status != "Inactive" {
		t.Errorf("Got %v, want %v", got.Status, "Inactive")
	}
}