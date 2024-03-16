package applications

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"log"
	"time"
)

type Organization struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Status    OrganizationStatus `bson:"status"`
	AccountID string             `bson:"accountId"`
	Name      string             `bson:"name"`
	CreatedAt time.Time          `bson:"createdAt"`
	DeletedAt time.Time          `bson:"deletedAt,omitempty"`
}

type OrganizationStatus string

const OrganizationCollection string = "organizations"
const ApplicationCollection string = "applications"
const ComponentCollection string = "components"

const (
	OrgActive   OrganizationStatus = "active"
	OrgDeleted  OrganizationStatus = "deleted"
	OrgDisabled OrganizationStatus = "disabled"
)

func (o *Organization) String() string {
	return fmt.Sprintf("Organization: %s, %s", o.Status, o.AccountID)
}

func (o *Organization) Create(mongo *database.MongoDB) (string, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	o.CreatedAt = time.Now()
	o.Status = OrgActive
	insertResult, err := collection.InsertOne(context.Background(), o)
	o.ID = insertResult.InsertedID.(primitive.ObjectID)
	if err != nil {
		return "", err
	}
	log.Println("Inserted a single document: ", insertResult.InsertedID.(primitive.ObjectID).Hex())
	return insertResult.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (o *Organization) GetById(mongo *database.MongoDB, ID string) (*Organization, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(OrganizationCollection)
	oID, _ := primitive.ObjectIDFromHex(ID)
	filter := bson.M{"_id": oID, "status": bson.M{"$ne": OrgDeleted}}
	err := collection.FindOne(context.Background(), filter).Decode(o)
	if err != nil {
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

func ApplicationList(mongo *database.MongoDB, organizationID string) ([]*Application, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
	oID, err := primitive.ObjectIDFromHex(organizationID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"organizationID": oID, "deletedAt": bson.M{"$exists": false}}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
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

func (a *Application) GetNamespace() string {
	return fmt.Sprintf("%s-%s", a.OrganizationID.Hex(), a.ID.Hex())
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

func (a *Application) GetByID(mongo *database.MongoDB, ID string) (*Application, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
	oID, _ := primitive.ObjectIDFromHex(ID)
	filter := bson.M{"_id": oID, "deletedAt": bson.M{"$exists": false}}
	err := collection.FindOne(context.Background(), filter).Decode(a)
	if err != nil {
		return nil, err
	}
	log.Println("Found a single document: ", a)
	return a, nil
}

func (a *Application) Update(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
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

func (a *Application) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
	filter := bson.D{{"_id", a.ID}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	log.Printf("Deleted %v documents in the applications collection\n", deleteResult.DeletedCount)
	return nil
}

func (a *Application) SoftDelete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(ApplicationCollection)
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

func (a *Application) ListComponents(mongo *database.MongoDB) ([]Component, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(ComponentCollection)
	filter := bson.M{"applicationID": a.ID, "deletedAt": bson.M{"$exists": false}}
	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
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

func (a *Application) CreateEnvironment(mongo *database.MongoDB, name string) (*Environment, error) {
	env := NewEnvironment(name)
	a.Environments = append(a.Environments, *env)
	err := a.Update(mongo)
	if err != nil {
		return nil, err
	}
	return env, nil
}

type Component struct {
	ID            primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	Name          string                 `json:"name" bson:"name"`
	Type          string                 `json:"type" bson:"type"`
	Description   string                 `json:"description" bson:"description"`
	ApplicationID primitive.ObjectID     `json:"application_id" bson:"applicationID"`
	Properties    map[string]interface{} `json:"properties,omitempty" bson:"properties,omitempty"`
	CreatedAt     time.Time              `json:"created_at" bson:"createdAt"`
	DeletedAt     time.Time              `json:"-" bson:"deletedAt,omitempty"`
}

func NewComponent(a *Application, name string, componentType string) *Component {
	return &Component{
		ApplicationID: a.ID,
		Name:          name,
		Type:          componentType,
	}
}
func (c *Component) Create(mongo *database.MongoDB) (*Component, error) {
	collection := mongo.Client.Database(mongo.DBName).Collection(ComponentCollection)
	c.CreatedAt = time.Now()
	r, err := collection.InsertOne(context.Background(), c)
	if err != nil {
		return nil, err
	}
	c.ID = r.InsertedID.(primitive.ObjectID)
	return c, nil
}

func (c *Component) Delete(mongo *database.MongoDB) error {
	collection := mongo.Client.Database(mongo.DBName).Collection(ComponentCollection)
	filter := bson.D{{"_id", c.ID}}
	deleteResult, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	log.Printf("Deleted %v documents in the components collection\n", deleteResult.DeletedCount)
	return nil

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
		ID:   generate8DigitHash(),
		Name: name,
	}
}

func (e *Environment) GetNamespace() string {
	return fmt.Sprintf("%s-%s", e.ID, e.Name)
}

func generate8DigitHash() string {
	// Create a new random seed
	seed := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, seed)
	if err != nil {
		log.Panicf("Error generating random seed")
	}
	// Hash the seed
	hash := sha256.Sum256(seed)
	// Return the first 8 characters of the hexadecimal representation of the hash
	return fmt.Sprintf("%x", hash)[:8]
}
