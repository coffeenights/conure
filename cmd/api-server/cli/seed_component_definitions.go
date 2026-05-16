package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// SeedCounters summarizes one run of RunSeedComponentDefinitions.
type SeedCounters struct {
	FilesRead int
	Upserted  int
	Skipped   int
}

// RunSeedComponentDefinitions reads ComponentDefinition YAML documents from
// dir and upserts each one into MongoDB under the default-owner sentinel
// (models.DefaultComponentDefinitionOwner). It is the body of the
// `seeddefaultcomponentdefinitions` subcommand, invoked by the Helm
// post-install/post-upgrade Job against the chart-mounted ConfigMap.
//
// The default-owner rows are the platform's shipped defaults: every org
// inherits them via models.ResolveForOrg until it overrides or hides one.
// Upsert keying is (type, engine), so re-running on every chart upgrade
// reconciles the shipped set in place rather than duplicating it — the Job is
// safe to run unconditionally.
//
// Files are parsed as the cluster ComponentDefinition CRD shape (same YAML
// ops already author by hand) so the chart's defaults and a hand-applied CRD
// are interchangeable source material. Multi-document files (--- separated)
// are supported; non-ComponentDefinition documents are skipped, not errors,
// so the directory can be a flat ConfigMap mount.
func RunSeedComponentDefinitions(ctx context.Context, db *database.MongoDB, dir string) (*SeedCounters, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading seed dir %q: %w", dir, err)
	}
	// Stable order so logs are deterministic across runs/replicas.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml") {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	counters := &SeedCounters{}
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return counters, fmt.Errorf("reading %q: %w", path, err)
		}
		counters.FilesRead++
		for _, doc := range splitYAMLDocuments(raw) {
			if strings.TrimSpace(string(doc)) == "" {
				continue
			}
			var crd conurev1alpha1.ComponentDefinition
			if err := yaml.Unmarshal(doc, &crd); err != nil {
				return counters, fmt.Errorf("parsing %q: %w", path, err)
			}
			if crd.Kind != "" && crd.Kind != "ComponentDefinition" {
				counters.Skipped++
				continue
			}
			if crd.Spec.ComponentType == "" {
				// Not a usable definition document (likely a stray
				// ConfigMap key or comment-only file).
				counters.Skipped++
				continue
			}
			def := crdToModel(&crd)
			if err := def.UpsertSeed(ctx, db); err != nil {
				return counters, fmt.Errorf("upserting default definition (type=%q engine=%q) from %q: %w",
					def.Type, def.EngineKey(), path, err)
			}
			counters.Upserted++
		}
	}
	return counters, nil
}

// crdToModel projects a cluster ComponentDefinition CRD (the YAML authoring
// shape) into the Mongo model used as the source of truth.
func crdToModel(crd *conurev1alpha1.ComponentDefinition) *models.ComponentDefinition {
	def := &models.ComponentDefinition{
		Type:          crd.Spec.ComponentType,
		Description:   crd.Spec.Description,
		Engine:        string(crd.Spec.Engine),
		OCIRepository: crd.Spec.OCIRepository,
		OCITag:        crd.Spec.OCITag,
		OCIDigest:     crd.Spec.OCIDigest,
		OCIRegistry:   crd.Spec.OCIRegistry,
		Buildable:     crd.Spec.Buildable,
		FieldRoles:    crd.Spec.FieldRoles,
	}
	if crd.Spec.RegistrySecretRef != nil {
		def.RegistrySecretName = crd.Spec.RegistrySecretRef.Name
	}
	if crd.Spec.Helm != nil {
		def.Helm = &models.ComponentDefHelm{
			ReleaseName: crd.Spec.Helm.ReleaseName,
			Namespace:   crd.Spec.Helm.Namespace,
			KubeVersion: crd.Spec.Helm.KubeVersion,
			APIVersions: crd.Spec.Helm.APIVersions,
		}
	}
	return def
}

// splitYAMLDocuments splits a YAML stream on the standalone `---` separator.
// Kept deliberately simple: seed files are conure-authored, not arbitrary
// untrusted YAML, so a literal line-level split is sufficient and avoids
// pulling in a streaming decoder.
func splitYAMLDocuments(raw []byte) [][]byte {
	parts := strings.Split(string(raw), "\n---")
	docs := make([][]byte, 0, len(parts))
	for _, p := range parts {
		docs = append(docs, []byte(strings.TrimPrefix(p, "---")))
	}
	return docs
}
