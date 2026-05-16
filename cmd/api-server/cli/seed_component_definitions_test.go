package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.mongodb.org/mongo-driver/bson"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/internal/config"
)

func seedTestDB(t *testing.T) *database.MongoDB {
	t.Helper()
	appConfig := config.LoadConfig(apiConfig.Config{})
	dbName := appConfig.MongoDBName + "-test-cli"
	client, err := database.ConnectToMongoDB(appConfig.MongoDBURI, dbName)
	if err != nil {
		t.Fatalf("connect test DB: %v", err)
	}
	db := &database.MongoDB{Client: client.Client, DBName: dbName}
	t.Cleanup(func() {
		_, _ = db.Client.Database(db.DBName).
			Collection(models.ComponentDefinitionCollection).
			DeleteMany(context.Background(), bson.M{"organizationId": models.DefaultComponentDefinitionOwner})
	})
	return db
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunSeedComponentDefinitions(t *testing.T) {
	db := seedTestDB(t)
	ctx := context.Background()
	dir := t.TempDir()

	// A normal single-doc definition.
	writeFile(t, dir, "webservice.yaml", `apiVersion: core.conure.io/v1alpha1
kind: ComponentDefinition
metadata:
  name: default-webservice-timoni
spec:
  type: webservice
  engine: timoni
  ociRepository: ghcr.io/acme/web
  ociTag: "1.0.0"
  buildable: true
  fieldRoles:
    image.repository: source.ociRepository
`)
	// Multi-document file: two defs plus a non-ComponentDefinition doc that
	// must be skipped, not error (flat ConfigMap mounts can carry anything).
	writeFile(t, dir, "multi.yaml", `apiVersion: core.conure.io/v1alpha1
kind: ComponentDefinition
spec:
  type: worker
  engine: timoni
  ociRepository: ghcr.io/acme/worker
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-definition
---
apiVersion: core.conure.io/v1alpha1
kind: ComponentDefinition
spec:
  type: webservice
  engine: helm
  ociRepository: ghcr.io/acme/web-helm
`)
	// A non-yaml file is ignored entirely.
	writeFile(t, dir, "README.txt", "ignore me")

	counters, err := RunSeedComponentDefinitions(ctx, db, dir)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if counters.FilesRead != 2 {
		t.Errorf("FilesRead = %d, want 2 (only .yaml)", counters.FilesRead)
	}
	if counters.Upserted != 3 {
		t.Errorf("Upserted = %d, want 3 (webservice/timoni, worker, webservice/helm)", counters.Upserted)
	}
	if counters.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (the ConfigMap doc)", counters.Skipped)
	}

	defs, err := models.ResolveForOrg(ctx, db, models.DefaultComponentDefinitionOwner)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 3 {
		t.Fatalf("expected 3 default rows, got %d: %+v", len(defs), defs)
	}

	// Field fidelity: the webservice/timoni row must carry the parsed
	// buildable flag and field roles through the CRD→model projection.
	ws, err := models.ResolveOneForOrg(ctx, db, models.DefaultComponentDefinitionOwner, "webservice", "timoni")
	if err != nil {
		t.Fatal(err)
	}
	if !ws.Buildable || ws.FieldRoles["image.repository"] != "source.ociRepository" {
		t.Fatalf("CRD→model projection lost fields: %+v", ws)
	}

	// Re-running reconciles in place (idempotent), not duplicates.
	if _, err := RunSeedComponentDefinitions(ctx, db, dir); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	defs2, err := models.ResolveForOrg(ctx, db, models.DefaultComponentDefinitionOwner)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs2) != 3 {
		t.Fatalf("re-seed duplicated rows: got %d, want 3", len(defs2))
	}
}

func TestSplitYAMLDocuments(t *testing.T) {
	in := []byte("a: 1\n---\nb: 2\n---\nc: 3\n")
	docs := splitYAMLDocuments(in)
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d: %q", len(docs), docs)
	}
}
