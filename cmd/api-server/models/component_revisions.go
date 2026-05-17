package models

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

const ComponentRevisionCollection string = "component_revisions"

type ComponentRevisionStatus string

const (
	RevisionStatusDraft    ComponentRevisionStatus = "draft"
	RevisionStatusDeployed ComponentRevisionStatus = "deployed"
)

// ComponentRevision is a versioned snapshot of a component's values for a
// specific (component, environment) pair. Drafts are mutable; deployed
// revisions are immutable and form the trail of past deploys.
type ComponentRevision struct {
	ID            primitive.ObjectID      `bson:"_id,omitempty" json:"id"`
	ComponentID   primitive.ObjectID      `bson:"componentID" json:"component_id"`
	EnvironmentID string                  `bson:"environmentID" json:"environment_id"`
	Version       int                     `bson:"version" json:"version"`
	Values        map[string]interface{}  `bson:"values" json:"values"`
	Status        ComponentRevisionStatus `bson:"status" json:"status"`
	Comment       string                  `bson:"comment,omitempty" json:"comment,omitempty"`
	DeployedAt    *time.Time              `bson:"deployedAt,omitempty" json:"deployed_at,omitempty"`
	CreatedBy     primitive.ObjectID      `bson:"createdBy,omitempty" json:"created_by"`
	CreatedAt     time.Time               `bson:"createdAt" json:"created_at"`
}

// EnsureComponentRevisionIndexes creates the indexes for the
// component_revisions collection. Safe to call repeatedly.
//
// Deployed revisions and drafts use SEPARATE version sequences:
//   - deployed history is a dense, append-only 1,2,3… trail; the unique
//     index is PARTIAL (status=deployed) so it never sees draft rows
//   - a draft is "the pending edit", not a numbered version: it carries
//     version 0 and there is AT MOST ONE per (componentID, environmentID),
//     enforced by a partial unique index on the pair filtered to status=draft
func EnsureComponentRevisionIndexes(ctx context.Context, db *database.MongoDB) error {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "componentID", Value: 1},
				{Key: "environmentID", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("componentID_environmentID_deployedVersion").
				SetPartialFilterExpression(bson.M{"status": RevisionStatusDeployed}),
		},
		{
			Keys: bson.D{
				{Key: "componentID", Value: 1},
				{Key: "environmentID", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("componentID_environmentID_singleDraft").
				SetPartialFilterExpression(bson.M{"status": RevisionStatusDraft}),
		},
		{
			Keys: bson.D{
				{Key: "componentID", Value: 1},
				{Key: "environmentID", Value: 1},
				{Key: "status", Value: 1},
			},
			Options: options.Index().SetName("componentID_environmentID_status"),
		},
	})
	return err
}

// EnsureComponentIndexes adds the unique (applicationID, name) index used to
// keep lazy auto-import safe under concurrency. Safe to call repeatedly.
func EnsureComponentIndexes(ctx context.Context, db *database.MongoDB) error {
	coll := db.Client.Database(db.DBName).Collection(ComponentCollection)
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "applicationID", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true).SetName("applicationID_name"),
	})
	return err
}

// nextDeployedVersion returns max(version)+1 over DEPLOYED rows only, so the
// deployed history is a dense 1,2,3… trail unaffected by drafts (which carry
// version 0 and are not part of this sequence).
func nextDeployedVersion(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string) (int, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})
	var latest ComponentRevision
	err := coll.FindOne(ctx, bson.M{
		"componentID":   componentID,
		"environmentID": environmentID,
		"status":        RevisionStatusDeployed,
	}, opts).Decode(&latest)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return latest.Version + 1, nil
}

// UpsertDraft makes r the single pending draft for (componentID,
// environmentID): if a draft already exists it is updated in place with r's
// values and comment; otherwise a new draft is inserted. A draft is "the
// pending edit", not a numbered version — it always has version 0 and there
// is never more than one per pair. On return r reflects the persisted row
// (ID, CreatedAt) so callers can echo it back.
func (r *ComponentRevision) UpsertDraft(ctx context.Context, db *database.MongoDB) error {
	if r.Status == "" {
		r.Status = RevisionStatusDraft
	}
	if r.Status != RevisionStatusDraft {
		return conureerrors.ErrInvalidRequest
	}
	if r.Values == nil {
		r.Values = map[string]interface{}{}
	}
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)

	existing, err := LatestDraft(ctx, db, r.ComponentID, r.EnvironmentID)
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return err
	}
	if existing != nil {
		res, err := coll.UpdateOne(ctx,
			bson.M{"_id": existing.ID, "status": RevisionStatusDraft},
			bson.M{"$set": bson.M{"values": r.Values, "comment": r.Comment}},
		)
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return conureerrors.ErrObjectNotFound
		}
		r.ID = existing.ID
		r.Version = 0
		r.CreatedAt = existing.CreatedAt
		return nil
	}

	r.Version = 0
	r.CreatedAt = time.Now()
	if r.ID.IsZero() {
		r.ID = primitive.NewObjectID()
	}
	_, err = coll.InsertOne(ctx, r)
	if err != nil {
		var we mongo.WriteException
		if errors.As(err, &we) && len(we.WriteErrors) > 0 && we.WriteErrors[0].Code == 11000 {
			// Lost a race to a concurrent UpsertDraft — the single-draft
			// partial unique index fired. Surface as already-exists; the
			// caller's retry will take the update branch.
			return conureerrors.ErrObjectAlreadyExists
		}
		return err
	}
	return nil
}

// CreateDeployed inserts a new deployed revision at the next dense deployed
// version (drafts do not consume numbers). Used by every deploy path plus the
// migration / auto-import flows where the revision is born deployed.
func (r *ComponentRevision) CreateDeployed(ctx context.Context, db *database.MongoDB) error {
	r.Status = RevisionStatusDeployed
	if r.DeployedAt == nil {
		now := time.Now()
		r.DeployedAt = &now
	}
	version, err := nextDeployedVersion(ctx, db, r.ComponentID, r.EnvironmentID)
	if err != nil {
		return err
	}
	r.Version = version
	r.CreatedAt = time.Now()
	if r.ID.IsZero() {
		r.ID = primitive.NewObjectID()
	}
	if r.Values == nil {
		r.Values = map[string]interface{}{}
	}
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	_, err = coll.InsertOne(ctx, r)
	if err != nil {
		var we mongo.WriteException
		if errors.As(err, &we) && len(we.WriteErrors) > 0 && we.WriteErrors[0].Code == 11000 {
			return conureerrors.ErrObjectAlreadyExists
		}
		return err
	}
	return nil
}

// SupersedeDrafts removes every draft revision for (componentID,
// environmentID). Called after any successful deploy/build: a deploy is the
// new truth for the pair, so all pending drafts are stale by definition.
// This is the single rule that keeps the version trail clean — deployed
// history is always fresh immutable rows, drafts never linger or get promoted
// into history. Returns the number of drafts removed; zero is not an error.
func SupersedeDrafts(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string) (int64, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	res, err := coll.DeleteMany(ctx, bson.M{
		"componentID":   componentID,
		"environmentID": environmentID,
		"status":        RevisionStatusDraft,
	})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// UpdateDraft replaces values and comment on an existing draft revision.
// Rejects updates to deployed revisions — the deployed history is immutable.
func (r *ComponentRevision) UpdateDraft(ctx context.Context, db *database.MongoDB, values map[string]interface{}, comment string) error {
	if r.Status != RevisionStatusDraft {
		return conureerrors.ErrNotAllowed
	}
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	res, err := coll.UpdateOne(ctx,
		bson.M{"_id": r.ID, "status": RevisionStatusDraft},
		bson.M{"$set": bson.M{"values": values, "comment": comment}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return conureerrors.ErrObjectNotFound
	}
	r.Values = values
	r.Comment = comment
	return nil
}

// GetByID loads a revision by its Mongo _id.
func (r *ComponentRevision) GetByID(ctx context.Context, db *database.MongoDB, id string) error {
	oID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return conureerrors.ErrObjectNotFound
	}
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	err = coll.FindOne(ctx, bson.M{"_id": oID}).Decode(r)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	}
	return err
}

// LatestDraft returns the highest-version draft for (componentID, environmentID),
// or ErrObjectNotFound if no draft exists.
func LatestDraft(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string) (*ComponentRevision, error) {
	return latestByStatus(ctx, db, componentID, environmentID, RevisionStatusDraft)
}

// LatestDeployed returns the highest-version deployed revision for
// (componentID, environmentID), or ErrObjectNotFound if none.
func LatestDeployed(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string) (*ComponentRevision, error) {
	return latestByStatus(ctx, db, componentID, environmentID, RevisionStatusDeployed)
}

func latestByStatus(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string, status ComponentRevisionStatus) (*ComponentRevision, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})
	var rev ComponentRevision
	err := coll.FindOne(ctx, bson.M{
		"componentID":   componentID,
		"environmentID": environmentID,
		"status":        status,
	}, opts).Decode(&rev)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, conureerrors.ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// ListRevisions returns every revision (draft + deployed) for the pair,
// ordered by version descending.
func ListRevisions(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string) ([]ComponentRevision, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}})
	cursor, err := coll.Find(ctx, bson.M{
		"componentID":   componentID,
		"environmentID": environmentID,
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var revisions []ComponentRevision
	if err := cursor.All(ctx, &revisions); err != nil {
		return nil, err
	}
	return revisions, nil
}

// EnvironmentsWithRevisions returns the distinct environment IDs that have at
// least one revision for the given component.
func EnvironmentsWithRevisions(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID) ([]string, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	rawIDs, err := coll.Distinct(ctx, "environmentID", bson.M{"componentID": componentID})
	if err != nil {
		return nil, err
	}
	envs := make([]string, 0, len(rawIDs))
	for _, raw := range rawIDs {
		if s, ok := raw.(string); ok {
			envs = append(envs, s)
		}
	}
	return envs, nil
}

// DeleteAllRevisionsForComponent removes every revision (draft and deployed) for
// the component. Used by the app-wide DeleteComponent path which wipes
// identity, all revisions, and all env CRDs.
func DeleteAllRevisionsForComponent(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID) (int64, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	res, err := coll.DeleteMany(ctx, bson.M{"componentID": componentID})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// ComponentsWithDraftInEnv returns the component IDs that have at least one
// draft revision in the given environment. Used by the bulk-deploy
// "deploy every draft" endpoint.
func ComponentsWithDraftInEnv(ctx context.Context, db *database.MongoDB, environmentID string) ([]primitive.ObjectID, error) {
	coll := db.Client.Database(db.DBName).Collection(ComponentRevisionCollection)
	rawIDs, err := coll.Distinct(ctx, "componentID", bson.M{
		"environmentID": environmentID,
		"status":        RevisionStatusDraft,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		if oid, ok := raw.(primitive.ObjectID); ok {
			ids = append(ids, oid)
		}
	}
	return ids, nil
}
