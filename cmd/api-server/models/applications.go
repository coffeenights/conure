package models

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/coffeenights/conure/internal/k8s"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

type OrganizationStatus string

const OrganizationCollection string = "organizations"
const ApplicationCollection string = "applications"
const ComponentCollection string = "components"
const ComponentTypeSpecCollection string = "componenttypespecs"

const (
	OrgActive   OrganizationStatus = "active"
	OrgDeleted  OrganizationStatus = "deleted"
	OrgDisabled OrganizationStatus = "disabled"
)

type Organization struct {
	Model     `bson:",inline"`
	Status    OrganizationStatus `bson:"status" json:"status"`
	AccountID primitive.ObjectID `bson:"accountId" json:"account_id"`
	Name      string             `bson:"name" json:"name"`
}

func (o *Organization) GetCollectionName() string {
	return OrganizationCollection
}

func OrganizationList(db *database.MongoDB, accountID string) ([]*Organization, error) {
	aID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return nil, err
	}
	return List[*Organization](context.Background(), db, bson.M{"accountId": aID}, &Organization{})
}

func (o *Organization) String() string {
	return fmt.Sprintf("Organization: %s, %s", o.Status, o.AccountID)
}

func (o *Organization) Create(db *database.MongoDB) (string, error) {
	o.Status = OrgActive
	if err := Create(context.Background(), db, o); err != nil {
		return "", err
	}
	return o.ID.Hex(), nil
}

func (o *Organization) GetById(db *database.MongoDB, ID string) (*Organization, error) {
	if err := GetByID(context.Background(), db, ID, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Organization) Update(db *database.MongoDB) error {
	return Update(context.Background(), db, o)
}

func (o *Organization) Delete(db *database.MongoDB) error {
	return Delete(context.Background(), db, o)
}

// SoftDelete flips the org to the deleted status and stamps deletedAt. The
// status enum keeps the active/disabled distinction; deletedAt is what lookups
// filter on (matching the rest of the codebase).
func (o *Organization) SoftDelete(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(OrganizationCollection)
	filter := bson.D{{Key: "_id", Value: o.ID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: OrgDeleted},
			{Key: "deletedAt", Value: time.Now()},
		}},
	}
	_, err := collection.UpdateOne(context.Background(), filter, update)
	return err
}

type Application struct {
	Model          `bson:",inline"`
	OrganizationID primitive.ObjectID `json:"organization_id" bson:"organizationID"`
	Name           string             `json:"name" bson:"name"`
	Description    string             `json:"description,omitempty" bson:"description,omitempty"`
	CreatedBy      primitive.ObjectID `json:"created_by" bson:"createdBy"`
	AccountID      primitive.ObjectID `json:"account_id" bson:"accountID"`
	Environments   []Environment      `json:"environments,omitempty" bson:"environments,omitempty"`
}

func (a *Application) GetCollectionName() string {
	return ApplicationCollection
}

func NewApplication(organizationID string, name string, createdBy string) *Application {
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		log.Panicf("Error parsing organizationID: %v\n", err)
	}
	createdByoID, err := primitive.ObjectIDFromHex(createdBy)
	if err != nil {
		log.Panicf("Error parsing createdBy: %v\n", err)
	}

	return &Application{
		OrganizationID: oID,
		Name:           name,
		CreatedBy:      createdByoID,
		AccountID:      createdByoID,
	}
}

func ApplicationList(db *database.MongoDB, organizationID string) ([]*Application, error) {
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		return nil, err
	}
	return List[*Application](context.Background(), db, bson.M{"organizationID": oID}, &Application{})
}

func (a *Application) GetEnvironmentByName(db *database.MongoDB, environmentName string) (*Environment, error) {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	pipeline := mongo.Pipeline{
		{
			{Key: "$match", Value: bson.D{{Key: "_id", Value: a.ID}}},
		},
		{
			{Key: "$unwind", Value: "$environments"},
		},
		{
			{Key: "$match", Value: bson.D{{Key: "environments.name", Value: environmentName}}},
		},
	}
	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	var results []bson.M
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, conureerrors.ErrObjectNotFound
	}
	var env Environment
	bsonBytes, _ := bson.Marshal(results[0]["environments"])
	if err = bson.Unmarshal(bsonBytes, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

func (a *Application) Create(db *database.MongoDB) (*Application, error) {
	if err := Create(context.Background(), db, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Application) GetByID(db *database.MongoDB, ID string) error {
	return GetByID(context.Background(), db, ID, a)
}

func (a *Application) Update(db *database.MongoDB) error {
	return Update(context.Background(), db, a)
}

func (a *Application) Delete(db *database.MongoDB) error {
	return Delete(context.Background(), db, a)
}

func (a *Application) SoftDelete(db *database.MongoDB) error {
	return SoftDelete(context.Background(), db, a)
}

func (a *Application) ListComponents(db *database.MongoDB) ([]Component, error) {
	collection := db.Client.Database(db.DBName).Collection(ComponentCollection)
	filter := bson.M{"applicationID": a.ID, "deletedAt": bson.M{"$exists": false}}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			log.Panicf("Error closing cursor: %v\n", err)
		}
	}(cursor, context.Background())
	var components []Component
	for cursor.Next(context.Background()) {
		var comp Component
		err = cursor.Decode(&comp)
		if err != nil {
			return nil, err
		}
		components = append(components, comp)
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	return components, nil
}

func (a *Application) CountComponents(db *database.MongoDB) (int64, error) {
	collection := db.Client.Database(db.DBName).Collection(ComponentCollection)
	filter := bson.M{"applicationID": a.ID, "deletedAt": bson.M{"$exists": false}}
	count, err := collection.CountDocuments(context.Background(), filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (a *Application) CreateEnvironment(db *database.MongoDB, name string) (*Environment, error) {
	env := NewEnvironment(name)
	a.Environments = append(a.Environments, *env)
	err := a.Update(db)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (a *Application) DeleteEnvironmentByID(db *database.MongoDB, envID string) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	filter := bson.M{"_id": a.ID}
	update := bson.M{"$pull": bson.M{"environments": bson.M{"_id": envID}}}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	if updateResult.ModifiedCount == 0 {
		return conureerrors.ErrObjectNotFound
	}
	return nil
}

func (a *Application) DeleteEnvironmentByName(db *database.MongoDB, envName string) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	filter := bson.M{"_id": a.ID}
	update := bson.M{"$pull": bson.M{"environments": bson.M{"name": envName}}}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	if updateResult.ModifiedCount == 0 {
		return conureerrors.ErrObjectNotFound
	}
	return nil
}

type ComponentTypeSpec struct {
	Model          `bson:",inline"`
	OrganizationID primitive.ObjectID `json:"organization_id" bson:"organizationID"`
	Name           string             `json:"name" bson:"name"`
	Description    string             `json:"description" bson:"description"`
	Type           string             `json:"type" bson:"type"`
	OCIRepository  string             `json:"oci_repository" bson:"ociRepository"`
	OCITag         string             `json:"oci_tag" bson:"ociTag"`
	IconURL        *string            `json:"icon_url" bson:"iconURL"`
}

func ComponentTypeSpecList(ctx context.Context, db *database.MongoDB, organizationID string) ([]*ComponentTypeSpec, error) {
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"organizationID": oID}
	specs, err := List[*ComponentTypeSpec](ctx, db, filter, &ComponentTypeSpec{})
	if err != nil {
		return nil, err
	}
	return specs, nil
}

func (c *ComponentTypeSpec) GetCollectionName() string {
	return ComponentTypeSpecCollection
}

func (c *ComponentTypeSpec) Create(ctx context.Context, db *database.MongoDB) error {
	err := Create(ctx, db, c)
	return err
}

func (c *ComponentTypeSpec) Delete(ctx context.Context, db *database.MongoDB) error {
	err := Delete(ctx, db, c)
	return err
}

func (c *ComponentTypeSpec) GetByID(ctx context.Context, db *database.MongoDB, ID string) error {
	err := GetByID(ctx, db, ID, c)
	return err
}

func (c *ComponentTypeSpec) Update(ctx context.Context, db *database.MongoDB) error {
	err := Update(ctx, db, c)
	return err
}

func (c *ComponentTypeSpec) GetByType(ctx context.Context, db *database.MongoDB, organizationID string, specType string) error {
	collection := db.Client.Database(db.DBName).Collection(ComponentTypeSpecCollection)
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		return err
	}

	filter := bson.M{
		"organizationID": oID,
		"type":           specType,
	}

	err = collection.FindOne(ctx, filter).Decode(c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	} else if err != nil {
		return err
	}
	return nil
}

type Component struct {
	Model         `bson:",inline"`
	Name          string             `json:"name" bson:"name"`
	Type          string             `json:"type" bson:"type"`
	Description   string             `json:"description" bson:"description"`
	ApplicationID primitive.ObjectID `json:"application_id" bson:"applicationID"`
}

func (c *Component) GetCollectionName() string {
	return ComponentCollection
}

func (c *Component) Create(db *database.MongoDB) error {
	err := Create(context.Background(), db, c)
	return err
}

func (c *Component) Delete(db *database.MongoDB) error {
	err := Delete(context.Background(), db, c)
	return err
}

func (c *Component) GetByID(db *database.MongoDB, ID string) error {
	err := GetByID(context.Background(), db, ID, c)
	return err
}

func (c *Component) Update(db *database.MongoDB) error {
	err := Update(context.Background(), db, c)
	return err
}

// GetByApplicationAndName looks up a component by (applicationID, name). Used
// by the lazy auto-import path to find an existing identity row before
// creating one for an orphan CRD. Returns ErrObjectNotFound if no row exists.
func (c *Component) GetByApplicationAndName(ctx context.Context, db *database.MongoDB, applicationID primitive.ObjectID, name string) error {
	collection := db.Client.Database(db.DBName).Collection(ComponentCollection)
	filter := bson.M{
		"applicationID": applicationID,
		"name":          name,
		"deletedAt":     bson.M{"$exists": false},
	}
	err := collection.FindOne(ctx, filter).Decode(c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	}
	return err
}

type Environment struct {
	ID   string `json:"id" bson:"_id"`
	Name string `json:"name" bson:"name"`
}

func NewEnvironment(name string) *Environment {
	return &Environment{
		ID:   k8s.Generate8DigitHash(),
		Name: name,
	}
}

func (e *Environment) GetNamespace() string {
	return fmt.Sprintf("%s-%s", e.ID, e.Name)
}
