package models

import (
	"context"
	"errors"
	"log"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

const (
	OrganizationType VariableType = "organization"
	EnvironmentType  VariableType = "environment"
	ComponentType    VariableType = "component"
)

type VariableType string

func (vt VariableType) IsValid() bool {
	return vt == OrganizationType || vt == EnvironmentType || vt == ComponentType
}

const VariableCollection string = "variables"

type Variable struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name           string              `bson:"name" json:"name" binding:"required"`
	Value          string              `bson:"value" json:"value" binding:"required"`
	Type           VariableType        `bson:"type" json:"type"`
	OrganizationID primitive.ObjectID  `bson:"organizationId,omitempty" json:"organization_id,omitempty"`
	ApplicationID  *primitive.ObjectID `bson:"applicationId,omitempty" json:"application_id,omitempty"`
	EnvironmentID  *string             `bson:"environmentId,omitempty" json:"environment_id,omitempty"`
	ComponentID    *primitive.ObjectID `bson:"componentId,omitempty" json:"component_id,omitempty"`
	IsEncrypted    bool                `bson:"isEncrypted" json:"is_encrypted"`
	CreatedAt      time.Time           `bson:"createdAt" json:"created_at"`
	UpdatedAt      time.Time           `bson:"updatedAt" json:"updated_at"`
}

func (v *Variable) ValidateName() bool {
	// env must match the pattern: ([a-z], [A-Z], [0-9]) or underscores (_)
	// it must start with a letter ([a-z], [A-Z]) or underscores (_)

	pattern := "^[a-zA-Z_][a-zA-Z0-9_]*$"
	matched, err := regexp.MatchString(pattern, v.Name)
	if err != nil {
		return false
	}
	return matched
}

func (v *Variable) Create(mongo *database.MongoDB) (string, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	v.CreatedAt = time.Now()
	v.UpdatedAt = v.CreatedAt

	insertResult, err := collection.InsertOne(context.Background(), v)
	if err != nil {
		return "", err
	}
	v.ID = insertResult.InsertedID.(primitive.ObjectID)
	log.Println("Inserted a single document: ", v.ID.Hex())
	return v.ID.Hex(), nil
}

func (v *Variable) Update(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	v.UpdatedAt = time.Now()

	_, err := collection.ReplaceOne(context.Background(), primitive.M{"_id": v.ID}, v)
	if err != nil {
		return err
	}
	return nil
}

func (v *Variable) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	_, err := collection.DeleteOne(context.Background(), primitive.M{"_id": v.ID})
	if err != nil {
		return err
	}
	return nil
}

func (v *Variable) ListByOrg(mongo *database.MongoDB, organizationID primitive.ObjectID) ([]Variable, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"name", 1}})
	cursor, err := collection.Find(context.Background(), primitive.M{"organizationId": organizationID, "type": OrganizationType}, findOptions)
	if err != nil {
		return nil, err
	}
	var variables = make([]Variable, 0)
	err = cursor.All(context.Background(), &variables)
	if err != nil {
		return nil, err
	}
	return variables, nil
}

func (v *Variable) ListByEnv(mongo *database.MongoDB, organizationID, applicationID primitive.ObjectID, environmentID string) ([]Variable, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"name", 1}})
	cursor, err := collection.Find(context.Background(), primitive.M{
		"organizationId": organizationID, "type": EnvironmentType, "applicationId": applicationID,
		"environmentId": environmentID}, findOptions)
	if err != nil {
		return nil, err
	}
	var variables = make([]Variable, 0)
	err = cursor.All(context.Background(), &variables)
	if err != nil {
		return nil, err
	}
	return variables, nil
}

func (v *Variable) ListByComp(mongo *database.MongoDB, organizationID, applicationID primitive.ObjectID, environmentID string, componentID primitive.ObjectID) ([]Variable, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	findOptions := options.Find()
	findOptions.SetSort(bson.D{{"name", 1}})
	cursor, err := collection.Find(context.Background(), primitive.M{
		"organizationId": organizationID, "type": ComponentType, "applicationId": applicationID,
		"environmentId": environmentID, "componentId": componentID}, findOptions)
	if err != nil {
		return nil, err
	}
	var variables = make([]Variable, 0)
	err = cursor.All(context.Background(), &variables)
	if err != nil {
		return nil, err
	}
	return variables, nil
}

func (v *Variable) GetByOrgAndName(mongo *database.MongoDB, organizationID primitive.ObjectID, name string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	err := collection.FindOne(context.Background(), primitive.M{"organizationId": organizationID, "name": name}).Decode(v)
	if err != nil {
		return err
	}
	return nil
}

func (v *Variable) GetByAppIDAndEnvAndName(mongo *database.MongoDB, applicationID primitive.ObjectID, t VariableType, environmentID *string, name string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	err := collection.FindOne(context.Background(), primitive.M{"applicationId": applicationID,
		"type": t, "name": name, "environmentId": environmentID}).Decode(v)
	if err != nil {
		return err
	}
	return nil
}

func (v *Variable) GetByAppIDAndEnvAndCompAndName(mongo *database.MongoDB, applicationID primitive.ObjectID, t VariableType, environmentID *string, componentID *primitive.ObjectID, name string) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(VariableCollection)
	err := collection.FindOne(context.Background(), primitive.M{"applicationId": applicationID,
		"type": t, "name": name, "environmentId": environmentID, "componentId": componentID}).Decode(v)
	if err != nil {
		return err
	}
	return nil
}

func (v *Variable) GetByID(db *database.MongoDB, id string) error {
	collection := db.Client.Database(db.DBName).Collection(VariableCollection)
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	err = collection.FindOne(context.Background(), primitive.M{"_id": objectID}).Decode(v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	} else if err != nil {
		return err
	}
	return nil
}
