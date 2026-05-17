package models

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

const CredentialCollection string = "credentials"

// CredentialKind discriminates what a credential is for. The two kinds map to
// the two K8s Secret shapes the deploy-time projection emits:
//
//   - registry: a kubernetes.io/dockerconfigjson Secret. Covers BOTH the
//     ComponentDefinition's private OCI template pull and the build Job's
//     image push (same dockerconfigjson shape, referenced from different
//     places).
//   - git: an Opaque Secret carrying a token (+ optional username). Consumed
//     by the build Job's git-clone init container for a private source repo.
type CredentialKind string

const (
	CredentialKindRegistry CredentialKind = "registry"
	CredentialKindGit      CredentialKind = "git"
)

func (k CredentialKind) IsValid() bool {
	return k == CredentialKindRegistry || k == CredentialKindGit
}

// Credential is the org-scoped, MongoDB source of truth for one named
// credential. It is encrypted at rest exactly like an encrypted Variable: the
// model only ever holds ciphertext, never plaintext — encryption/decryption
// happens at the handler layer via the AES keyStorage. The deploy path
// resolves a Credential by (org, name), decrypts it, and projects it into a
// K8s Secret alongside the Component (mirroring how variables are gathered and
// applied). Nothing here is written to Kubernetes; the projection owns that.
//
// Material fields by kind:
//
//	registry: RegistryURL (plaintext, not secret), Username (plaintext),
//	          Secret = the registry password/token (ENCRYPTED)
//	git:      Username (plaintext, defaults to "x-access-token" when empty),
//	          Secret = the git token (ENCRYPTED); RegistryURL unused
//
// Name is the logical name a ComponentDefinition (CredentialRef, registry
// kind) or a component's revision values (git.credentialRef /
// image.credentialRef field-roles) reference. It is unique per (org, name).
type Credential struct {
	Model          `bson:",inline"`
	OrganizationID primitive.ObjectID `bson:"organizationId" json:"organization_id"`
	Name           string             `bson:"name" json:"name"`
	Kind           CredentialKind     `bson:"kind" json:"kind"`

	// --- material ---
	// NB: bson tags must NOT be `omitempty`. Update() does
	// `$set: <whole struct>` (cmd/api-server/models/model.go), so an
	// omitempty bson tag would drop an empty field from the $set doc and
	// Mongo would never clear it. That silently preserves stale values
	// across a cross-kind rotation (e.g. a registry cred re-`set` as git by
	// the same name would keep its old registryUrl). The json tags keep
	// `omitempty` so empty metadata still stays out of API responses.
	RegistryURL string `bson:"registryUrl" json:"registry_url,omitempty"`
	Username    string `bson:"username" json:"username,omitempty"`
	// Secret is the encrypted password/token. It is intentionally NOT
	// serialized to JSON: list/get responses must never leak credential
	// material, and the value is ciphertext anyway.
	Secret string `bson:"secret" json:"-"`
}

func (c *Credential) GetCollectionName() string {
	return CredentialCollection
}

func (c *Credential) Create(ctx context.Context, db *database.MongoDB) (string, error) {
	if err := Create(ctx, db, c); err != nil {
		return "", err
	}
	return c.ID.Hex(), nil
}

func (c *Credential) Update(ctx context.Context, db *database.MongoDB) error {
	return Update(ctx, db, c)
}

// Delete soft-deletes the row so a deleted credential stops resolving while
// remaining auditable, consistent with the other soft-deleted models.
func (c *Credential) Delete(ctx context.Context, db *database.MongoDB) error {
	return SoftDelete(ctx, db, c)
}

// GetByOrgAndName fetches one credential by (org, name), ignoring
// soft-deleted rows. Returns conureerrors.ErrObjectNotFound when absent so
// the deploy path can surface an actionable "no such credential" error rather
// than a raw mongo error.
func (c *Credential) GetByOrgAndName(ctx context.Context, db *database.MongoDB, organizationID primitive.ObjectID, name string) error {
	collection := db.Client.Database(db.DBName).Collection(CredentialCollection)
	filter := bson.M{
		"organizationId": organizationID,
		"name":           name,
		"deletedAt":      bson.M{"$exists": false},
	}
	err := collection.FindOne(ctx, filter).Decode(c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return conureerrors.ErrObjectNotFound
	}
	return err
}

// ListByOrg returns all non-deleted credentials for an org, sorted by name.
// Callers that serialize the result still get ciphertext-only material
// because Secret is json:"-"; metadata-only listing is enforced by the type,
// not by the handler remembering to strip it.
func (c *Credential) ListByOrg(ctx context.Context, db *database.MongoDB, organizationID primitive.ObjectID) ([]Credential, error) {
	collection := db.Client.Database(db.DBName).Collection(CredentialCollection)
	findOptions := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{
		"organizationId": organizationID,
		"deletedAt":      bson.M{"$exists": false},
	}, findOptions)
	if err != nil {
		return nil, err
	}
	out := make([]Credential, 0)
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
