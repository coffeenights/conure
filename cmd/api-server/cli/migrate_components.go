// Package cli holds API-server admin sub-commands. This file implements the
// one-shot migration that promotes Kubernetes from "render target" to source
// of truth for component values.
//
// Walk every Application in Mongo, scan each environment namespace for
// `Component` CRDs labelled with this application, and for each CRD found
// either reuse the matching Mongo identity or auto-import a new one. Every
// CRD then gets a single v1 deployed `ComponentRevision` whose `values`
// match the live `Component.Spec.Values`. Legacy `Component.Values` and
// `Application.Revisions` fields are dropped after writes complete.
//
// The migration is idempotent over its own progress: components that already
// have a v1 deployed revision in an env are not double-written.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// MigrateComponentsConfig drives one execution of the migration.
type MigrateComponentsConfig struct {
	DryRun bool
}

// MigrationCounters reports how much work the migration did or would do.
type MigrationCounters struct {
	Apps               int
	Envs               int
	CRDsObserved       int
	IdentitiesCreated  int
	RevisionsWritten   int
	RevisionsSkipped   int
	UnboundCRDs        int
	VerificationErrors []string
}

// RunMigrateComponents executes the migration. Dry-run mode logs what would
// happen and writes nothing.
func RunMigrateComponents(ctx context.Context, db *database.MongoDB, cfg MigrateComponentsConfig) (*MigrationCounters, error) {
	if !cfg.DryRun {
		if err := models.EnsureComponentRevisionIndexes(ctx, db); err != nil {
			return nil, fmt.Errorf("ensure revision indexes: %w", err)
		}
		if err := models.EnsureComponentIndexes(ctx, db); err != nil {
			return nil, fmt.Errorf("ensure component indexes: %w", err)
		}
	}

	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, fmt.Errorf("k8s clientset: %w", err)
	}

	counters := &MigrationCounters{}

	apps, err := listAllApplications(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("listing applications: %w", err)
	}
	counters.Apps = len(apps)

	for i := range apps {
		app := apps[i]
		for _, env := range app.Environments {
			counters.Envs++
			ns := env.GetNamespace()
			selector := fmt.Sprintf("%s=%s", k8sUtils.ApplicationIDLabel, app.ID.Hex())
			list, err := clientset.Conure.CoreV1alpha1().Components(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					continue
				}
				return counters, fmt.Errorf("listing components in %s: %w", ns, err)
			}
			for j := range list.Items {
				live := &list.Items[j]
				counters.CRDsObserved++
				if !crdHasRequiredLabels(live) {
					counters.UnboundCRDs++
					log.Printf("migration: skipping unbound CRD %s/%s (missing required labels)", ns, live.Name)
					continue
				}
				if err := importOne(ctx, db, &app, &env, live, counters, cfg); err != nil {
					return counters, fmt.Errorf("importing %s/%s: %w", ns, live.Name, err)
				}
			}
		}
	}

	if !cfg.DryRun {
		if err := dropLegacyFields(ctx, db); err != nil {
			return counters, fmt.Errorf("dropping legacy fields: %w", err)
		}
	}

	if err := verify(ctx, db, clientset, apps, counters); err != nil {
		return counters, err
	}
	return counters, nil
}

func listAllApplications(ctx context.Context, db *database.MongoDB) ([]models.Application, error) {
	coll := db.Client.Database(db.DBName).Collection(models.ApplicationCollection)
	cursor, err := coll.Find(ctx, bson.M{"deletedAt": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var apps []models.Application
	if err := cursor.All(ctx, &apps); err != nil {
		return nil, err
	}
	return apps, nil
}

func crdHasRequiredLabels(c *conurev1alpha1.Component) bool {
	if _, ok := c.Labels[k8sUtils.ApplicationIDLabel]; !ok {
		return false
	}
	if _, ok := c.Labels[k8sUtils.OrganizationIDLabel]; !ok {
		return false
	}
	return true
}

func importOne(ctx context.Context, db *database.MongoDB, app *models.Application, env *models.Environment, live *conurev1alpha1.Component, counters *MigrationCounters, cfg MigrateComponentsConfig) error {
	component := &models.Component{}
	err := component.GetByApplicationAndName(ctx, db, app.ID, live.Name)
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		log.Printf("migration: importing identity for %s/%s", env.GetNamespace(), live.Name)
		if cfg.DryRun {
			counters.IdentitiesCreated++
			counters.RevisionsWritten++
			return nil
		}
		component = &models.Component{
			Name:          live.Name,
			Type:          live.Spec.ComponentType,
			Description:   live.Annotations["conure.io/description"],
			ApplicationID: app.ID,
		}
		if err := component.Create(db); err != nil {
			return err
		}
		counters.IdentitiesCreated++
	} else if err != nil {
		return err
	}

	values, err := decodeLiveValues(live)
	if err != nil {
		return err
	}

	existing, err := models.LatestDeployed(ctx, db, component.ID, env.ID)
	if err == nil && existing != nil && deepEqualValues(existing.Values, values) {
		counters.RevisionsSkipped++
		return nil
	}

	if cfg.DryRun {
		counters.RevisionsWritten++
		return nil
	}

	deployedAt := live.CreationTimestamp.Time
	if deployedAt.IsZero() {
		deployedAt = time.Now()
	}
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        values,
		DeployedAt:    &deployedAt,
	}
	if err := rev.CreateDeployed(ctx, db); err != nil {
		return err
	}
	counters.RevisionsWritten++
	return nil
}

func decodeLiveValues(c *conurev1alpha1.Component) (map[string]interface{}, error) {
	if c.Spec.Values == nil || len(c.Spec.Values.Raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(c.Spec.Values.Raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]interface{}{}, nil
	}
	return m, nil
}

func deepEqualValues(a, b map[string]interface{}) bool {
	an, _ := json.Marshal(a)
	bn, _ := json.Marshal(b)
	return string(an) == string(bn)
}

func dropLegacyFields(ctx context.Context, db *database.MongoDB) error {
	components := db.Client.Database(db.DBName).Collection(models.ComponentCollection)
	if _, err := components.UpdateMany(ctx, bson.M{}, bson.M{"$unset": bson.M{"values": ""}}); err != nil {
		return fmt.Errorf("unset components.values: %w", err)
	}
	apps := db.Client.Database(db.DBName).Collection(models.ApplicationCollection)
	if _, err := apps.UpdateMany(ctx, bson.M{}, bson.M{"$unset": bson.M{"revisions": ""}}); err != nil {
		return fmt.Errorf("unset applications.revisions: %w", err)
	}
	return nil
}

// verify re-walks every CRD across the apps' env namespaces and asserts that
// each has a corresponding identity row plus at least one deployed revision
// whose values match. Any mismatch is recorded; the caller decides whether
// to treat the migration as failed.
func verify(ctx context.Context, db *database.MongoDB, clientset *k8sUtils.GenericClientset, apps []models.Application, counters *MigrationCounters) error {
	for i := range apps {
		app := apps[i]
		for _, env := range app.Environments {
			ns := env.GetNamespace()
			selector := fmt.Sprintf("%s=%s", k8sUtils.ApplicationIDLabel, app.ID.Hex())
			list, err := clientset.Conure.CoreV1alpha1().Components(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					continue
				}
				return err
			}
			for j := range list.Items {
				live := &list.Items[j]
				if !crdHasRequiredLabels(live) {
					continue
				}
				component := &models.Component{}
				if err := component.GetByApplicationAndName(ctx, db, app.ID, live.Name); err != nil {
					counters.VerificationErrors = append(counters.VerificationErrors, fmt.Sprintf("missing identity for %s/%s", ns, live.Name))
					continue
				}
				deployed, err := models.LatestDeployed(ctx, db, component.ID, env.ID)
				if err != nil {
					counters.VerificationErrors = append(counters.VerificationErrors, fmt.Sprintf("missing deployed revision for %s/%s", ns, live.Name))
					continue
				}
				values, err := decodeLiveValues(live)
				if err != nil {
					counters.VerificationErrors = append(counters.VerificationErrors, fmt.Sprintf("decoding live values for %s/%s: %v", ns, live.Name, err))
					continue
				}
				if !deepEqualValues(values, deployed.Values) {
					counters.VerificationErrors = append(counters.VerificationErrors, fmt.Sprintf("value mismatch for %s/%s (live vs deployed v%d)", ns, live.Name, deployed.Version))
				}
			}
		}
	}
	return nil
}
