package models

import (
	"context"
	"errors"
	"fmt"
	"github.com/coffeenights/conure/internal/k8s"
	"log"
	"time"

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

const (
	OrgActive   OrganizationStatus = "active"
	OrgDeleted  OrganizationStatus = "deleted"
	OrgDisabled OrganizationStatus = "disabled"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Status    OrganizationStatus `bson:"status" json:"status"`
	AccountID primitive.ObjectID `bson:"accountId" json:"account_id"`
	Name      string             `bson:"name" json:"name"`
	CreatedAt time.Time          `bson:"createdAt" json:"created_at"`
	DeletedAt time.Time          `bson:"deletedAt,omitempty" json:"-"`
}

func OrganizationList(db *database.MongoDB, accountID string) ([]*Organization, error) {
	collection := db.Client.Database(db.DBName).Collection(OrganizationCollection)
	aID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"accountId": aID, "status": bson.M{"$ne": OrgDeleted}}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err = cursor.Close(ctx)
		if err != nil {
			log.Panicf("Error closing cursor: %v\n", err)
		}
	}(cursor, context.Background())
	var organizations []*Organization
	for cursor.Next(context.Background()) {
		var org Organization
		err = cursor.Decode(&org)
		if err != nil {
			return nil, err
		}
		organizations = append(organizations, &org)
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	return organizations, nil
}

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
	o.ID = insertResult.InsertedID.(primitive.ObjectID)
	log.Println("Inserted a single document: ", insertResult.InsertedID.(primitive.ObjectID).Hex())
	return insertResult.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (o *Organization) GetById(db *database.MongoDB, ID string) (*Organization, error) {
	collection := db.Client.Database(db.DBName).Collection(OrganizationCollection)
	oID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": oID, "status": bson.M{"$ne": OrgDeleted}}
	err = collection.FindOne(context.Background(), filter).Decode(o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, conureerrors.ErrObjectNotFound
	} else if err != nil {
		return nil, err
	}
	log.Println("Found a single document: ", o)
	return o, nil
}

func (o *Organization) Update(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.M{"_id": o.ID, "status": bson.M{"$ne": OrgDeleted}}
	update := bson.D{
		{"$set", o},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

func (o *Organization) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.D{{"_id", o.ID}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	log.Printf("Deleted %v documents in the organizations collection\n", deleteResult.DeletedCount)
	return nil
}

func (o *Organization) SoftDelete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	filter := bson.D{{"_id", o.ID}}
	update := bson.D{
		{"$set", bson.D{
			{"status", OrgDeleted},
			{"deletedAt", time.Now()},
		}},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and deleted %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

type Application struct {
	ID             primitive.ObjectID    `json:"id,omitempty" bson:"_id,omitempty"`
	OrganizationID primitive.ObjectID    `json:"organization_id" bson:"organizationID"`
	Name           string                `json:"name" bson:"name"`
	Description    string                `json:"description,omitempty" bson:"description,omitempty"`
	CreatedBy      primitive.ObjectID    `json:"created_by" bson:"createdBy"`
	AccountID      primitive.ObjectID    `json:"account_id" bson:"accountID"`
	Revisions      []ApplicationRevision `json:"revisions,omitempty" bson:"revisions,omitempty"`
	CreatedAt      time.Time             `json:"created_at" bson:"createdAt"`
	DeletedAt      time.Time             `json:"-" bson:"deletedAt,omitempty"`
	Environments   []Environment         `json:"environments,omitempty" bson:"environments,omitempty"`
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
		Revisions: []ApplicationRevision{
			{
				RevisionNumber: 0,
				CreatedAt:      time.Now(),
			},
		},
		CreatedBy: createdByoID,
		AccountID: createdByoID,
	}
}

func ApplicationList(db *database.MongoDB, organizationID string) ([]*Application, error) {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"organizationID": oID, "deletedAt": bson.M{"$exists": false}}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err = cursor.Close(ctx)
		if err != nil {
			log.Panicf("Error closing cursor: %v\n", err)
		}
	}(cursor, context.Background())
	var applications []*Application
	for cursor.Next(context.Background()) {
		var app Application
		err = cursor.Decode(&app)
		if err != nil {
			return nil, err
		}
		applications = append(applications, &app)
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	return applications, nil
}

func (a *Application) GetEnvironmentByName(db *database.MongoDB, environmentName string) (*Environment, error) {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	pipeline := mongo.Pipeline{
		{{"$match", bson.D{{"_id", a.ID}}}},
		{{"$unwind", "$environments"}},
		{{"$match", bson.D{{"environments.name", environmentName}}}},
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

func (a *Application) Create(mongo *database.MongoDB) (*Application, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
	a.CreatedAt = time.Now()
	insertResult, err := collection.InsertOne(context.Background(), a)
	if err != nil {
		return nil, err
	}
	log.Println("Inserted a single document: ", insertResult.InsertedID.(primitive.ObjectID).Hex())
	a.ID = insertResult.InsertedID.(primitive.ObjectID)
	return a, nil
}

func (a *Application) GetByID(db *database.MongoDB, ID string) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	oID, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": oID, "deletedAt": bson.M{"$exists": false}}
	err = collection.FindOne(context.Background(), filter).Decode(a)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	} else if err != nil {
		return err
	}
	log.Println("Found a single document: ", a)
	return nil
}

func (a *Application) Update(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	filter := bson.M{"_id": a.ID}
	update := bson.D{
		{"$set", a},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

func (a *Application) Delete(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	filter := bson.D{{"_id", a.ID}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	log.Printf("Deleted %v documents in the applications collection\n", deleteResult.DeletedCount)
	return nil
}

func (a *Application) SoftDelete(db *database.MongoDB) error {
	collection := db.Client.Database(db.DBName).Collection(ApplicationCollection)
	filter := bson.D{{"_id", a.ID}}
	update := bson.D{
		{"$set", bson.D{
			{"deletedAt", time.Now()},
		}},
	}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	log.Printf("Matched %v documents and deleted %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
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

type Component struct {
	Model         `bson:",inline"`
	Name          string             `json:"name" bson:"name"`
	Type          string             `json:"type" bson:"type"`
	Description   string             `json:"description" bson:"description"`
	ApplicationID primitive.ObjectID `json:"application_id" bson:"applicationID"`
	Settings      ComponentSettings  `json:"settings" bson:"settings"`
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

type ApplicationRevision struct {
	RevisionNumber int       `json:"revision_number" bson:"revisionNumber"`
	CreatedAt      time.Time `json:"created_at" bson:"createdAt"`
	DeletedAt      time.Time `json:"-" bson:"deletedAt,omitempty"`
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

type ResourcesSettings struct {
	Replicas int     `json:"replicas" bson:"replicas"`
	CPU      float32 `json:"cpu" bson:"cpu"`
	Memory   int     `json:"memory" bson:"memory"`
}

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
)

type Protocol string

const (
	TCP Protocol = "tcp"
	UDP Protocol = "udp"
)

type PortSettings struct {
	HostPort   int      `json:"host_port" bson:"hostPort"`
	TargetPort int      `json:"target_port" bson:"targetPort"`
	Protocol   Protocol `json:"protocol" bson:"protocol"`
}

type NetworkSettings struct {
	Exposed bool           `json:"exposed" bson:"exposed"`
	Type    AccessType     `json:"type" bson:"type"`
	Ports   []PortSettings `json:"ports" bson:"ports"`
}

type SourceSettings struct {
	Repository string `json:"repository" bson:"repository"`
	Command    string `json:"command" bson:"command"`
}

type StorageSettings struct {
	Size      float32 `json:"size" bson:"size"`
	Name      string  `json:"name" bson:"name"`
	MountPath string  `json:"mount_path" bson:"mountPath"`
}

type ComponentSettings struct {
	ResourcesSettings ResourcesSettings `json:"resources_settings" bson:"resourcesSettings"`
	SourceSettings    SourceSettings    `json:"source_settings" bson:"sourceSettings"`
	NetworkSettings   NetworkSettings   `json:"network_settings" bson:"networkSettings"`
	StorageSettings   []StorageSettings `json:"storage_settings" bson:"storageSettings"`
}
