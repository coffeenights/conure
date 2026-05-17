package providers

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	"github.com/coffeenights/conure/internal/config"
	"github.com/coffeenights/conure/internal/credentials"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// setupProvidersDB returns a per-package test DB (parallel-safe with the
// other suites) plus a local AES key store for encrypt/decrypt round-trips.
func setupProvidersDB(t *testing.T) (*database.MongoDB, variables.SecretKeyStorage) {
	t.Helper()
	appConfig := config.LoadConfig(apiConfig.Config{})
	dbName := appConfig.MongoDBName + "-test-providers"
	client, err := database.ConnectToMongoDB(appConfig.MongoDBURI, dbName)
	if err != nil {
		t.Fatal(err)
	}
	db := &database.MongoDB{Client: client.Client, DBName: dbName}
	// Generate the AES key into a temp file so the suite is independent of
	// the working directory (the providers package dir has no secret.key,
	// unlike the variables/credentials package dirs).
	ks := variables.NewLocalSecretKey(filepath.Join(t.TempDir(), "secret.key"))
	if err := ks.Generate(); err != nil {
		t.Fatalf("generate AES key: %v", err)
	}
	return db, ks
}

func clearCreds(ctx context.Context, db *database.MongoDB, orgs ...primitive.ObjectID) {
	coll := db.Client.Database(db.DBName).Collection(models.CredentialCollection)
	for _, o := range orgs {
		_, _ = coll.DeleteMany(ctx, bson.M{"organizationId": o})
	}
}

func seedRegistryCred(t *testing.T, ctx context.Context, db *database.MongoDB, ks variables.SecretKeyStorage, org primitive.ObjectID, name, plaintext string) {
	t.Helper()
	enc, err := variables.EncryptValue(ks, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	c := &models.Credential{
		OrganizationID: org,
		Name:           name,
		Kind:           models.CredentialKindRegistry,
		RegistryURL:    "ghcr.io",
		Username:       "octocat",
		Secret:         enc,
	}
	if _, err := c.Create(ctx, db); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
}

func TestProjectRegistryCredential_EmptyRefIsPublicNoSecret(t *testing.T) {
	db, ks := setupProvidersDB(t)
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	cr := &CredentialResolver{DB: db, KeyStorage: ks}

	name, err := cr.projectRegistryCredential(context.Background(), cs, primitive.NewObjectID(), "")
	if err != nil {
		t.Fatalf("empty ref must not error, got %v", err)
	}
	if name != "" {
		t.Fatalf("empty ref must yield empty Secret name (anonymous pull), got %q", name)
	}
	// No Secret should have been created.
	secrets, _ := cs.K8s.CoreV1().Secrets(credentials.SystemNamespace).List(context.Background(), metav1.ListOptions{})
	if len(secrets.Items) != 0 {
		t.Fatalf("empty ref must not project a Secret, found %d", len(secrets.Items))
	}
}

func TestProjectRegistryCredential_MissingCredentialIsHardError(t *testing.T) {
	db, ks := setupProvidersDB(t)
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(context.Background(), db, org) })
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	cr := &CredentialResolver{DB: db, KeyStorage: ks}

	_, err := cr.projectRegistryCredential(context.Background(), cs, org, "ghcr")
	if err == nil {
		t.Fatal("a referenced-but-missing credential must be a hard error (no fallback)")
	}
	if !strings.Contains(err.Error(), "conure credential set") {
		t.Fatalf("error should tell the operator how to fix it, got: %v", err)
	}
}

func TestProjectRegistryCredential_WrongKindErrors(t *testing.T) {
	db, ks := setupProvidersDB(t)
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(context.Background(), db, org) })

	// A git credential under the name the definition expects for registry.
	gc := &models.Credential{OrganizationID: org, Name: "ghcr", Kind: models.CredentialKindGit, Secret: "x"}
	if _, err := gc.Create(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	cr := &CredentialResolver{DB: db, KeyStorage: ks}

	if _, err := cr.projectRegistryCredential(context.Background(), cs, org, "ghcr"); err == nil {
		t.Fatal("a git credential used where a registry credential is required must error")
	}
}

func TestProjectRegistryCredential_HappyPathProjectsAndReturnsConcreteName(t *testing.T) {
	db, ks := setupProvidersDB(t)
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(context.Background(), db, org) })
	seedRegistryCred(t, context.Background(), db, ks, org, "ghcr", "s3cr3t-token")

	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	cr := &CredentialResolver{DB: db, KeyStorage: ks}

	name, err := cr.projectRegistryCredential(context.Background(), cs, org, "ghcr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := credentials.SecretName(org.Hex(), "ghcr")
	if name != want {
		t.Fatalf("returned Secret name = %q, want concrete %q", name, want)
	}

	sec, err := cs.K8s.CoreV1().Secrets(credentials.SystemNamespace).Get(context.Background(), want, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not projected: %v", err)
	}
	if sec.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("projected Secret type = %s, want dockerconfigjson", sec.Type)
	}
	if sec.Labels[k8sUtils.OrganizationIDLabel] != org.Hex() {
		t.Fatalf("org label = %q, want %q", sec.Labels[k8sUtils.OrganizationIDLabel], org.Hex())
	}
	// The decrypted password must be what landed in the dockerconfigjson.
	dcj := string(sec.Data[corev1.DockerConfigJsonKey])
	if !strings.Contains(dcj, "s3cr3t-token") {
		t.Fatalf("projected dockerconfigjson missing decrypted password")
	}
}

func TestProjectRegistryCredential_RotationUpdatesInPlace(t *testing.T) {
	db, ks := setupProvidersDB(t)
	org := primitive.NewObjectID()
	t.Cleanup(func() { clearCreds(context.Background(), db, org) })
	seedRegistryCred(t, context.Background(), db, ks, org, "ghcr", "old-token")

	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	cr := &CredentialResolver{DB: db, KeyStorage: ks}

	if _, err := cr.projectRegistryCredential(context.Background(), cs, org, "ghcr"); err != nil {
		t.Fatal(err)
	}
	// Rotate the stored credential, re-project: same Secret, new material,
	// no duplicate.
	enc, _ := variables.EncryptValue(ks, "new-token")
	upd := &models.Credential{}
	if err := upd.GetByOrgAndName(context.Background(), db, org, "ghcr"); err != nil {
		t.Fatal(err)
	}
	upd.Secret = enc
	if err := upd.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := cr.projectRegistryCredential(context.Background(), cs, org, "ghcr"); err != nil {
		t.Fatal(err)
	}

	secrets, _ := cs.K8s.CoreV1().Secrets(credentials.SystemNamespace).List(context.Background(), metav1.ListOptions{})
	if len(secrets.Items) != 1 {
		t.Fatalf("rotation must not create a second Secret, found %d", len(secrets.Items))
	}
	if !strings.Contains(string(secrets.Items[0].Data[corev1.DockerConfigJsonKey]), "new-token") {
		t.Fatal("rotated Secret should carry the new token")
	}
}
