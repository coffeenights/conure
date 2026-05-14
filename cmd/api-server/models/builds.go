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

const BuildCollection = "builds"

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusBuilding  BuildStatus = "building"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
)

// BuildLocation distinguishes the CLI-driven local push from the
// server-orchestrated BuildKit Job in the cluster.
type BuildLocation string

const (
	BuildLocationLocal  BuildLocation = "local"
	BuildLocationRemote BuildLocation = "remote"
)

// BuildTool selects which BuildKit frontend the remote job runs, and which
// builder the CLI invokes for local builds. Railpack is rejected for remote
// builds by the API — see applications/builds.go.
type BuildTool string

const (
	BuildToolRailpack   BuildTool = "railpack"
	BuildToolDockerfile BuildTool = "dockerfile"
)

// Build is one build attempt for a (component, environment) pair. Local
// builds are durably recorded as succeeded the moment the CLI POSTs (the
// image was already pushed). Remote builds start as pending → building →
// terminal, watched by a goroutine that holds a Mongo lease so multi-replica
// API deployments don't race.
type Build struct {
	Model         `bson:",inline"`
	ComponentID   primitive.ObjectID `bson:"componentID"   json:"component_id"`
	ApplicationID primitive.ObjectID `bson:"applicationID" json:"application_id"`
	EnvironmentID string             `bson:"environmentID" json:"environment_id"`
	Status        BuildStatus        `bson:"status"        json:"status"`
	BuildTool     BuildTool          `bson:"buildTool"     json:"build_tool"`
	BuildLocation BuildLocation      `bson:"buildLocation" json:"build_location"`
	Platform      string             `bson:"platform,omitempty"      json:"platform,omitempty"`
	GitRepository string             `bson:"gitRepository,omitempty" json:"git_repository,omitempty"`
	GitBranch     string             `bson:"gitBranch,omitempty"     json:"git_branch,omitempty"`
	ImageRef      string             `bson:"imageRef,omitempty"      json:"image_ref,omitempty"`
	JobName       string             `bson:"jobName,omitempty"       json:"job_name,omitempty"`
	JobNamespace  string             `bson:"jobNamespace,omitempty"  json:"job_namespace,omitempty"`
	ErrorMessage  string             `bson:"errorMessage,omitempty"  json:"error_message,omitempty"`
	FinishedAt    *time.Time         `bson:"finishedAt,omitempty"    json:"finished_at,omitempty"`

	// CreatedBy is the user who triggered the build. Empty (NilObjectID) on
	// system-triggered builds (none exist today; reserved for future
	// promotion-rebuilds).
	CreatedBy primitive.ObjectID `bson:"createdBy,omitempty" json:"created_by,omitempty"`

	// Lease bookkeeping — see TryAcquireLease/Heartbeat. WatcherID is the
	// UUID of the API replica currently polling the BuildKit Job;
	// WatcherExpiresAt is the deadline after which any replica may take
	// over. Both empty/zero means "no live watcher; up for grabs."
	WatcherID        string     `bson:"watcherID,omitempty"        json:"-"`
	WatcherExpiresAt *time.Time `bson:"watcherExpiresAt,omitempty" json:"-"`
}

func (b *Build) GetCollectionName() string { return BuildCollection }

// EnsureBuildIndexes creates the lookup + lease indexes. Safe to call
// repeatedly. Composite (status, watcherExpiresAt) backs the periodic
// adoption scan; (componentID, environmentID) backs the per-component list.
func EnsureBuildIndexes(ctx context.Context, db *database.MongoDB) error {
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "componentID", Value: 1},
				{Key: "environmentID", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("componentID_environmentID_createdAt"),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "watcherExpiresAt", Value: 1},
			},
			Options: options.Index().SetName("status_watcherExpiresAt"),
		},
	})
	return err
}

// ListBuildsForComponent returns every build for the (component, env) pair
// in newest-first order. Bounded — callers display the latest N.
func ListBuildsForComponent(ctx context.Context, db *database.MongoDB, componentID primitive.ObjectID, environmentID string, limit int64) ([]Build, error) {
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cursor, err := coll.Find(ctx, bson.M{
		"componentID":   componentID,
		"environmentID": environmentID,
		"deletedAt":     bson.M{"$exists": false},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []Build
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetBuildByID loads a single Build. Returns conureerrors.ErrObjectNotFound
// when nothing matches, mirroring the rest of the model package.
func GetBuildByID(ctx context.Context, db *database.MongoDB, id string) (*Build, error) {
	oID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, conureerrors.ErrObjectNotFound
	}
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	var b Build
	err = coll.FindOne(ctx, bson.M{"_id": oID, "deletedAt": bson.M{"$exists": false}}).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, conureerrors.ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// MarkTerminal flips a build to succeeded/failed and stamps FinishedAt. Also
// clears the lease fields so a future scan never tries to adopt a finished
// build. Idempotent — calling it twice with the same status is a no-op.
func (b *Build) MarkTerminal(ctx context.Context, db *database.MongoDB, status BuildStatus, imageRef, errMsg string) error {
	now := time.Now()
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	update := bson.M{
		"status":           status,
		"finishedAt":       now,
		"updatedAt":        now,
		"watcherID":        "",
		"watcherExpiresAt": nil,
	}
	if imageRef != "" {
		update["imageRef"] = imageRef
	}
	if errMsg != "" {
		update["errorMessage"] = errMsg
	}
	_, err := coll.UpdateOne(ctx, bson.M{"_id": b.ID}, bson.M{"$set": update})
	if err != nil {
		return err
	}
	b.Status = status
	b.FinishedAt = &now
	b.WatcherID = ""
	b.WatcherExpiresAt = nil
	if imageRef != "" {
		b.ImageRef = imageRef
	}
	if errMsg != "" {
		b.ErrorMessage = errMsg
	}
	return nil
}

// SetJob records the BuildKit Job's identity on the build and flips status
// to building. Called after Job creation succeeds.
func (b *Build) SetJob(ctx context.Context, db *database.MongoDB, name, namespace string) error {
	now := time.Now()
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	_, err := coll.UpdateOne(ctx,
		bson.M{"_id": b.ID},
		bson.M{"$set": bson.M{
			"jobName":      name,
			"jobNamespace": namespace,
			"status":       BuildStatusBuilding,
			"updatedAt":    now,
		}},
	)
	if err != nil {
		return err
	}
	b.JobName = name
	b.JobNamespace = namespace
	b.Status = BuildStatusBuilding
	return nil
}

// TryAcquireLease atomically claims the build for `watcherID` for `ttl`
// going forward. Wins when the existing lease is empty, already ours, or
// expired. Returns (true, nil) when this caller now owns the lease;
// (false, nil) when another replica owns it. The local b is updated on
// success.
//
// This is the core multi-replica safety: Mongo's single-document
// conditional update guarantees at most one winner per race. Heartbeat
// extends the lease while the watcher is alive; expiration releases it when
// the watcher dies.
func (b *Build) TryAcquireLease(ctx context.Context, db *database.MongoDB, watcherID string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expires := now.Add(ttl)
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	filter := bson.M{
		"_id":    b.ID,
		"status": bson.M{"$in": []BuildStatus{BuildStatusPending, BuildStatusBuilding}},
		"$or": []bson.M{
			{"watcherID": bson.M{"$in": []interface{}{"", nil}}},
			{"watcherID": watcherID},
			{"watcherExpiresAt": bson.M{"$lt": now}},
		},
	}
	res, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{
		"watcherID":        watcherID,
		"watcherExpiresAt": expires,
		"updatedAt":        now,
	}})
	if err != nil {
		return false, err
	}
	if res.MatchedCount == 0 {
		return false, nil
	}
	b.WatcherID = watcherID
	b.WatcherExpiresAt = &expires
	return true, nil
}

// Heartbeat refreshes the lease while this watcher is the rightful owner.
// Returns (false, nil) if another replica has stolen the lease — caller
// should abandon the watcher goroutine.
func (b *Build) Heartbeat(ctx context.Context, db *database.MongoDB, watcherID string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expires := now.Add(ttl)
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	filter := bson.M{
		"_id":       b.ID,
		"watcherID": watcherID,
	}
	res, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{
		"watcherExpiresAt": expires,
		"updatedAt":        now,
	}})
	if err != nil {
		return false, err
	}
	if res.MatchedCount == 0 {
		return false, nil
	}
	b.WatcherExpiresAt = &expires
	return true, nil
}

// AdoptableBuilds returns builds whose status is still pending/building and
// whose lease is missing or expired. Used both at startup and by the
// periodic scan to pick up orphaned remote builds.
func AdoptableBuilds(ctx context.Context, db *database.MongoDB, now time.Time, limit int64) ([]Build, error) {
	coll := db.Client.Database(db.DBName).Collection(BuildCollection)
	// jobName must exist and be non-empty: a remote build with no Job
	// recorded yet can't be watched. Mongo `$ne: ""` would match missing
	// fields too, so combine $exists with the inequality.
	filter := bson.M{
		"status":        bson.M{"$in": []BuildStatus{BuildStatusPending, BuildStatusBuilding}},
		"buildLocation": BuildLocationRemote,
		"jobName":       bson.M{"$exists": true, "$nin": []interface{}{"", nil}},
		"$or": []bson.M{
			{"watcherID": bson.M{"$in": []interface{}{"", nil}}},
			{"watcherExpiresAt": bson.M{"$exists": false}},
			{"watcherExpiresAt": bson.M{"$lt": now}},
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var out []Build
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
