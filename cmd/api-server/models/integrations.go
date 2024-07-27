package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/coffeenights/conure/cmd/api-server/database"
)

const IntegrationCollection = "integrations"

type Integration struct {
	Model            `bson:",inline"`
	OrganizationID   primitive.ObjectID `json:"organization_id" bson:"organizationID"`
	IntegrationType  string             `json:"integration_type" bson:"integrationType"`
	Name             string             `json:"name" bson:"name"`
	IntegrationValue interface{}        `json:"-" bson:"integrationValue"`
}

func (i *Integration) GetCollectionName() string {
	return IntegrationCollection
}

func (i *Integration) Create(db *database.MongoDB) error {
	err := Create(context.Background(), db, i)
	return err
}

func (i *Integration) Delete(db *database.MongoDB) error {
	err := Delete(context.Background(), db, i)
	return err
}

func (i *Integration) GetByID(db *database.MongoDB, ID string) error {
	err := GetByID(context.Background(), db, ID, i)
	return err
}

func (i *Integration) Update(db *database.MongoDB) error {
	err := Update(context.Background(), db, i)
	return err
}

func (i *Integration) ListIntegrations(db *database.MongoDB) ([]Integration, error) {
	collection := db.Client.Database(db.DBName).Collection(IntegrationCollection)
	filter := bson.M{"organizationID": i.OrganizationID}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	var integrations = make([]Integration, 0)
	err = cursor.All(context.Background(), &integrations)
	if err != nil {
		return nil, err
	}
	return integrations, nil
}

type IntegrationTypeInterface interface {
	EncryptValues() error
	DecryptValues() error
}
