package models

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

const ComponentDefinitionCollection string = "componentdefinitions"

// DefaultComponentDefinitionOwner is the sentinel OrganizationID under which
// the Helm-shipped default definitions are seeded. It is the zero ObjectID and
// is intentionally NOT a real Organization row — no account owns it, it never
// appears in org listings. Every org transparently inherits these definitions
// until it creates its own row (an override) or a tombstone (a hide) for the
// same (type, engine). Resolution is org-specific first, then this owner.
//
// The controller never sees this value: by deploy time the API has already
// resolved the definition and materialized it into the cluster under the real
// org's id/label, so cluster-side tenant isolation is unaffected.
var DefaultComponentDefinitionOwner = primitive.NilObjectID

// ComponentDefinition is the org-scoped, MongoDB source of truth for a
// component type. It mirrors the fields of the cluster-scoped
// ComponentDefinition CRD (apis/core/v1alpha1) but is keyed by
// OrganizationID so multiple orgs — and multiple clusters — can carry
// different definitions for the same type. The API materializes the resolved
// row into whatever cluster a deploy targets; Mongo stays authoritative.
//
// A row with Hidden=true is a tombstone: it carries no spec, it only
// suppresses the default-owner fallback for its (Type, Engine) within the
// owning org. Tombstones are reversible (delete the row to restore the
// default).
type ComponentDefinition struct {
	Model          `bson:",inline"`
	OrganizationID primitive.ObjectID `bson:"organizationId" json:"organization_id"`
	// Hidden marks this row as a tombstone (see type doc). When true the spec
	// fields are ignored.
	Hidden bool `bson:"hidden,omitempty" json:"hidden,omitempty"`

	// --- spec, mirrors conurev1alpha1.ComponentDefinitionSpec ---
	Type        string `bson:"type" json:"type"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	// Engine is always persisted as a concrete value ("timoni" when the
	// caller left it unset — see normalizeEngine). It deliberately has no
	// omitempty: a missing-vs-"" -vs-"timoni" trichotomy is exactly what made
	// rows un-findable, so storage is normalized on write and every read path
	// can do a plain exact match.
	Engine        string `bson:"engine" json:"engine,omitempty"`
	OCIRepository string `bson:"ociRepository,omitempty" json:"oci_repository,omitempty"`
	OCITag        string `bson:"ociTag,omitempty" json:"oci_tag,omitempty"`
	OCIDigest     string `bson:"ociDigest,omitempty" json:"oci_digest,omitempty"`
	OCIRegistry   string `bson:"ociRegistry,omitempty" json:"oci_registry,omitempty"`
	// RegistrySecretName is the name of a dockerconfigjson Secret in the
	// controller's namespace (mirrors the CRD's RegistrySecretRef.Name).
	RegistrySecretName string                 `bson:"registrySecretName,omitempty" json:"registry_secret_name,omitempty"`
	Helm               *ComponentDefHelm      `bson:"helm,omitempty" json:"helm,omitempty"`
	Buildable          bool                   `bson:"buildable,omitempty" json:"buildable,omitempty"`
	FieldRoles         map[string]string      `bson:"fieldRoles,omitempty" json:"field_roles,omitempty"`
	IconURL            *string                `bson:"iconUrl,omitempty" json:"icon_url,omitempty"`
	Extra              map[string]interface{} `bson:"extra,omitempty" json:"extra,omitempty"`
}

// ComponentDefHelm mirrors conurev1alpha1.HelmEngineSpec.
type ComponentDefHelm struct {
	ReleaseName string   `bson:"releaseName,omitempty" json:"release_name,omitempty"`
	Namespace   string   `bson:"namespace,omitempty" json:"namespace,omitempty"`
	KubeVersion string   `bson:"kubeVersion,omitempty" json:"kube_version,omitempty"`
	APIVersions []string `bson:"apiVersions,omitempty" json:"api_versions,omitempty"`
}

func (cd *ComponentDefinition) GetCollectionName() string {
	return ComponentDefinitionCollection
}

// normalizeEngineValue is the single definition of "the empty engine means
// timoni". Storage and lookups both route through it so a row written with no
// engine and one written as "timoni" are byte-identical on disk and an exact
// match finds either.
func normalizeEngineValue(engine string) string {
	if engine == "" {
		return "timoni"
	}
	return engine
}

// EngineKey normalizes the engine for keying/lookup: an empty engine is
// equivalent to "timoni" (the CRD default), so a definition stored with no
// engine and one stored as "timoni" collide for the same type, exactly as
// they would at the controller's (type, engine) resolution.
func (cd *ComponentDefinition) EngineKey() string {
	return normalizeEngineValue(cd.Engine)
}

// normalizeEngine pins Engine to a concrete value before any write, so the
// stored field is never missing/empty (see the Engine field doc).
func (cd *ComponentDefinition) normalizeEngine() {
	cd.Engine = normalizeEngineValue(cd.Engine)
}

func (cd *ComponentDefinition) Create(ctx context.Context, db *database.MongoDB) (string, error) {
	cd.normalizeEngine()
	if err := Create(ctx, db, cd); err != nil {
		return "", err
	}
	return cd.ID.Hex(), nil
}

func (cd *ComponentDefinition) GetById(ctx context.Context, db *database.MongoDB, ID string) (*ComponentDefinition, error) {
	if err := GetByID(ctx, db, ID, cd); err != nil {
		return nil, err
	}
	return cd, nil
}

func (cd *ComponentDefinition) Update(ctx context.Context, db *database.MongoDB) error {
	cd.normalizeEngine()
	return Update(ctx, db, cd)
}

// Delete hard-deletes the row. For a default-owner row this restores it on the
// next seed; for an org override/tombstone it restores the inherited default.
func (cd *ComponentDefinition) Delete(ctx context.Context, db *database.MongoDB) error {
	return Delete(ctx, db, cd)
}

// listOwnerDefinitions returns all non-deleted rows for one owner, sorted by
// name for stable output.
func listOwnerDefinitions(ctx context.Context, db *database.MongoDB, ownerID primitive.ObjectID) ([]ComponentDefinition, error) {
	collection := db.Client.Database(db.DBName).Collection(ComponentDefinitionCollection)
	findOptions := options.Find().SetSort(bson.D{{Key: "type", Value: 1}, {Key: "engine", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{
		"organizationId": ownerID,
		"deletedAt":      bson.M{"$exists": false},
	}, findOptions)
	if err != nil {
		return nil, err
	}
	out := make([]ComponentDefinition, 0)
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveForOrg returns the effective set of component definitions visible to
// an org: the default-owner set, with the org's own rows layered on top by
// (type, engine). An org override replaces the default; an org tombstone
// (Hidden=true) removes the inherited default and is itself excluded from the
// result. Org-only definitions (no matching default) are included.
//
// This is the single resolution rule the listing endpoint and the deploy-time
// materialization both call, so the CLI picker and the controller-facing
// projection can never disagree.
func ResolveForOrg(ctx context.Context, db *database.MongoDB, organizationID primitive.ObjectID) ([]ComponentDefinition, error) {
	defaults, err := listOwnerDefinitions(ctx, db, DefaultComponentDefinitionOwner)
	if err != nil {
		return nil, err
	}
	type key struct{ t, e string }
	merged := make(map[key]ComponentDefinition, len(defaults))
	for _, d := range defaults {
		merged[key{d.Type, d.EngineKey()}] = d
	}

	if organizationID != DefaultComponentDefinitionOwner {
		orgRows, err := listOwnerDefinitions(ctx, db, organizationID)
		if err != nil {
			return nil, err
		}
		for _, o := range orgRows {
			k := key{o.Type, o.EngineKey()}
			if o.Hidden {
				delete(merged, k)
				continue
			}
			merged[k] = o
		}
	}

	out := make([]ComponentDefinition, 0, len(merged))
	for _, d := range merged {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].EngineKey() < out[j].EngineKey()
	})
	return out, nil
}

// ResolveOneForOrg resolves a single definition by (type, engine) for an org,
// applying the same default→org-override / tombstone rule as ResolveForOrg.
//
// engine may be empty: it then matches any single definition for the type
// (mirroring the controller, which treats an unset Component.spec.engine as
// "any single match, ambiguity is an error"). A non-empty engine must match
// exactly (empty stored engine == "timoni").
//
// Returns conureerrors.ErrComponentNotFound when nothing matches and
// conureerrors.ErrAmbiguousComponentEngine when engine is empty and more than
// one definition matches the type — the same up-front contract the create
// path already enforces against the cluster.
func ResolveOneForOrg(ctx context.Context, db *database.MongoDB, organizationID primitive.ObjectID, compType, engine string) (*ComponentDefinition, error) {
	all, err := ResolveForOrg(ctx, db, organizationID)
	if err != nil {
		return nil, err
	}
	var matches []ComponentDefinition
	for _, d := range all {
		if d.Type != compType {
			continue
		}
		if engine != "" && d.EngineKey() != engine {
			continue
		}
		matches = append(matches, d)
	}
	switch len(matches) {
	case 0:
		return nil, conureerrors.ErrComponentNotFound
	case 1:
		out := matches[0]
		return &out, nil
	default:
		return nil, conureerrors.ErrAmbiguousComponentEngine
	}
}

// GetOwnRow fetches one org-owned row (override or tombstone) by (type,
// engine), without applying default fallback. Used by the CRUD endpoints to
// decide create-vs-update and by the seed command for idempotency. engine ""
// is normalized to "timoni" before matching so callers don't have to; since
// storage is normalized on write too, this is a plain exact match.
func (cd *ComponentDefinition) GetOwnRow(ctx context.Context, db *database.MongoDB, organizationID primitive.ObjectID, compType, engine string) error {
	collection := db.Client.Database(db.DBName).Collection(ComponentDefinitionCollection)
	filter := bson.M{
		"organizationId": organizationID,
		"type":           compType,
		"engine":         normalizeEngineValue(engine),
		"deletedAt":      bson.M{"$exists": false},
	}
	err := collection.FindOne(ctx, filter).Decode(cd)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	}
	return err
}

// UpsertSeed inserts or refreshes a default-owner row keyed by (type, engine).
// Idempotent: re-running the Helm seed Job on upgrade reconciles the shipped
// defaults without duplicating rows or clobbering createdAt.
func (cd *ComponentDefinition) UpsertSeed(ctx context.Context, db *database.MongoDB) error {
	cd.OrganizationID = DefaultComponentDefinitionOwner
	cd.Hidden = false
	cd.normalizeEngine()
	collection := db.Client.Database(db.DBName).Collection(ComponentDefinitionCollection)
	filter := bson.M{
		"organizationId": DefaultComponentDefinitionOwner,
		"type":           cd.Type,
		"engine":         cd.Engine,
	}
	now := time.Now()
	set := bson.M{
		"organizationId":     DefaultComponentDefinitionOwner,
		"hidden":             false,
		"type":               cd.Type,
		"description":        cd.Description,
		"engine":             cd.Engine,
		"ociRepository":      cd.OCIRepository,
		"ociTag":             cd.OCITag,
		"ociDigest":          cd.OCIDigest,
		"ociRegistry":        cd.OCIRegistry,
		"registrySecretName": cd.RegistrySecretName,
		"helm":               cd.Helm,
		"buildable":          cd.Buildable,
		"fieldRoles":         cd.FieldRoles,
		"iconUrl":            cd.IconURL,
		"extra":              cd.Extra,
		"updatedAt":          now,
	}
	update := bson.M{
		"$set":         set,
		"$setOnInsert": bson.M{"_id": primitive.NewObjectID(), "createdAt": now},
	}
	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
