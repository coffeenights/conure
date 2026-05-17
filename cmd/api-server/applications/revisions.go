package applications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"github.com/coffeenights/conure/internal/fieldroles"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// liveComponent looks up a single Component CRD in the env namespace.
// Returns (nil, nil) if the namespace or CRD is missing — that's a normal
// "not active in this env" state, not an error.
func liveComponent(ctx context.Context, a *ApiHandler, namespace, name string) (*conurev1alpha1.Component, error) {
	clientset, err := a.kubeClient()
	if err != nil {
		return nil, err
	}
	comp, err := clientset.Conure.CoreV1alpha1().Components(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return comp, nil
}

// liveComponentAndAbsorb is the canonical "read live state for a known
// (component, env) pair" call. It fetches the CRD via liveComponent and,
// when one exists, reconciles Mongo's last deployed revision against it via
// absorbDriftIfAny. Every endpoint that reads live state for an existing
// Mongo identity should go through this so out-of-band CRD edits are
// recorded regardless of which endpoint observed them first.
//
// Returns (nil, nil) when the CRD is missing, matching liveComponent.
func liveComponentAndAbsorb(ctx context.Context, a *ApiHandler, component *models.Component, env *models.Environment) (*conurev1alpha1.Component, error) {
	live, err := liveComponent(ctx, a, env.GetNamespace(), component.Name)
	if err != nil || live == nil {
		return live, err
	}
	if err := absorbDriftIfAny(ctx, a, component, env, live); err != nil {
		return nil, err
	}
	return live, nil
}

// ListComponentsInEnv lists every Component CRD present in the env namespace
// for this application. Orphan CRDs (no Mongo identity yet) get one created
// via ensureComponentIdentity, and any drift between the live CRD and the
// last deployed revision is absorbed via absorbDriftIfAny.
//
// Path: GET /:orgID/a/:appID/e/:env/c
func (a *ApiHandler) ListComponentsInEnv(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	ctx := c.Request.Context()

	clientset, err := a.kubeClient()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	selector := fmt.Sprintf("%s=%s", k8sUtils.ApplicationIDLabel, handler.Model.ID.Hex())
	list, err := clientset.Conure.CoreV1alpha1().Components(env.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil && !k8sErrors.IsNotFound(err) {
		log.Printf("Error listing components in %s: %v\n", env.GetNamespace(), err)
		conureerrors.AbortWithError(c, err)
		return
	}

	response := ComponentInEnvListResponse{Components: []ComponentInEnvResponse{}}
	if list == nil {
		c.JSON(http.StatusOK, response)
		return
	}
	for i := range list.Items {
		live := &list.Items[i]
		identity, err := ensureComponentIdentity(ctx, a, handler.Model, live)
		if err != nil {
			log.Printf("Error onboarding orphan CRD %s: %v\n", live.Name, err)
			conureerrors.AbortWithError(c, err)
			return
		}
		if err := absorbDriftIfAny(ctx, a, identity, env, live); err != nil {
			log.Printf("Error absorbing drift for %s: %v\n", live.Name, err)
			conureerrors.AbortWithError(c, err)
			return
		}
		entry, err := buildEnvDetail(ctx, a, identity, env, live)
		if err != nil {
			log.Printf("Error building env detail for %s: %v\n", live.Name, err)
			conureerrors.AbortWithError(c, err)
			return
		}
		response.Components = append(response.Components, entry)
	}
	c.JSON(http.StatusOK, response)
}

// GetComponentInEnv returns live K8s state, last-deployed revision, latest
// draft, and a structured drift diff for a single (component, env) pair.
//
// Path: GET /:orgID/a/:appID/e/:env/c/:componentID
func (a *ApiHandler) GetComponentInEnv(c *gin.Context) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if component.ApplicationID != handler.Model.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	ctx := c.Request.Context()
	live, err := liveComponentAndAbsorb(ctx, a, component, env)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	resp, err := buildEnvDetail(ctx, a, component, env, live)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateRevision writes a new draft revision for (component, env). Promote
// is preferred to seed a new env, but this endpoint is the right call when
// the user is editing values directly in env X.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/revisions
func (a *ApiHandler) CreateRevision(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	var req CreateRevisionRequest
	if err := c.BindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        req.Values,
		Comment:       req.Comment,
		CreatedBy:     uID,
	}
	if err := rev.CreateDraft(c.Request.Context(), a.MongoDB); err != nil {
		log.Printf("Error creating revision: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	_ = handler
	c.JSON(http.StatusCreated, rev)
}

// ListRevisions returns every revision (draft and deployed) for the pair,
// newest first. Reads live state first so any out-of-band CRD edits surface
// as a fresh auto-imported revision in the list.
//
// Path: GET /:orgID/a/:appID/e/:env/c/:componentID/revisions
func (a *ApiHandler) ListRevisions(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := liveComponentAndAbsorb(ctx, a, component, env); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	revisions, err := models.ListRevisions(ctx, a.MongoDB, component.ID, env.ID)
	if err != nil {
		log.Printf("Error listing revisions: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, ComponentRevisionListResponse{Revisions: revisions})
}

// UpdateDraftRevision replaces the values on an existing draft revision.
// Deployed revisions are immutable and reject this call with 403.
//
// Path: PUT /:orgID/a/:appID/e/:env/c/:componentID/revisions/:revID
func (a *ApiHandler) UpdateDraftRevision(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	rev := &models.ComponentRevision{}
	ctx := c.Request.Context()
	if err := rev.GetByID(ctx, a.MongoDB, c.Param("revID")); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if rev.ComponentID != component.ID || rev.EnvironmentID != env.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	var req UpdateRevisionRequest
	if err := c.BindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if err := rev.UpdateDraft(ctx, a.MongoDB, req.Values, req.Comment); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, rev)
}

// DeployRevision applies the latest draft for (component, env) to K8s and
// flips it to deployed. The status transition is one-way and final.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/deploy
func (a *ApiHandler) DeployRevision(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	ctx := c.Request.Context()
	draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	if err := applyRevisionToK8s(ctx, a, handler.Model, env, component, draft.Values); err != nil {
		log.Printf("Error applying revision to K8s: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if err := draft.MarkDeployed(ctx, a.MongoDB); err != nil {
		log.Printf("Error marking deployed: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, draft)
}

// DeployRevisionByID is the rollback / redeploy primitive: load revision
// `revID`, copy its values into a fresh deployed revision at the head, and
// apply that to K8s. The original revision is not mutated.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/revisions/:revID/deploy
func (a *ApiHandler) DeployRevisionByID(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	source := &models.ComponentRevision{}
	ctx := c.Request.Context()
	if err := source.GetByID(ctx, a.MongoDB, c.Param("revID")); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if source.ComponentID != component.ID || source.EnvironmentID != env.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	if err := applyRevisionToK8s(ctx, a, handler.Model, env, component, source.Values); err != nil {
		log.Printf("Error applying revision to K8s: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	uID := c.MustGet("currentUser").(models.User).ID
	newRev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        source.Values,
		CreatedBy:     uID,
	}
	if err := newRev.CreateDeployed(ctx, a.MongoDB); err != nil {
		log.Printf("Error inserting deployed revision: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, newRev)
}

// RestartComponent triggers a rolling restart of the component's workload by
// stamping a fresh conure.io/restartedAt annotation on the Component CRD,
// re-applying the latest deployed values, and recording a new deployed
// revision (auto-commented "Restart at <ts>") so the action shows up in
// history.
//
// Component types without a pod template are a silent no-op: the annotation
// lands on the Component but the rendered manifests have nowhere to put it.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/restart
func (a *ApiHandler) RestartComponent(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	ctx := c.Request.Context()
	deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
	if err != nil {
		if errors.Is(err, conureerrors.ErrObjectNotFound) {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		conureerrors.AbortWithError(c, err)
		return
	}

	restartedAt := time.Now().UTC().Format(time.RFC3339)
	if err := applyRevisionToK8sWithAnnotations(ctx, a, handler.Model, env, component, deployed.Values, map[string]string{
		conurev1alpha1.RestartedAtAnnotation: restartedAt,
	}); err != nil {
		log.Printf("Error applying restart to K8s: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	newRev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        deployed.Values,
		Comment:       fmt.Sprintf("Restart at %s", restartedAt),
		CreatedBy:     uID,
	}
	if err := newRev.CreateDeployed(ctx, a.MongoDB); err != nil {
		log.Printf("Error inserting restart revision: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, newRev)
}

// UninstallFromEnv deletes the live Component CRD plus its variables
// ConfigMap/Secret in the env namespace and purges all draft revisions for
// the pair. Deployed history is retained.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/uninstall
func (a *ApiHandler) UninstallFromEnv(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	if abortIfCannotWriteApp(c, handler) {
		return
	}

	ctx := c.Request.Context()
	provider := newConureProvider(handler.Model, env)
	if err := provider.DeleteComponentCRD(ctx, component.Name); err != nil {
		log.Printf("Error deleting component CRD: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	if _, err := models.DeleteDraftsForPair(ctx, a.MongoDB, component.ID, env.ID); err != nil {
		log.Printf("Error deleting drafts: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeployEnvDrafts walks every component in the application that has a draft
// revision in `env` and deploys each. Failures are reported per-component;
// partial success is allowed.
//
// Path: POST /:orgID/a/:appID/e/:env/deploy
func (a *ApiHandler) DeployEnvDrafts(c *gin.Context) {
	handler, err := getHandlerFromRouteForWrite(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}

	ctx := c.Request.Context()
	componentIDs, err := models.ComponentsWithDraftInEnv(ctx, a.MongoDB, env.ID)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	response := DeployBatchResponse{
		Deployed: []DeployBatchEntry{},
		Failed:   []DeployBatchEntry{},
	}
	for _, componentID := range componentIDs {
		entry := DeployBatchEntry{ComponentID: componentID.Hex()}
		component := &models.Component{}
		if err := component.GetByID(a.MongoDB, componentID.Hex()); err != nil {
			entry.Error = fmt.Sprintf("loading component: %v", err)
			response.Failed = append(response.Failed, entry)
			continue
		}
		if component.ApplicationID != handler.Model.ID {
			// Concurrent re-parenting; skip rather than crash.
			continue
		}
		entry.ComponentName = component.Name

		draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
		if err != nil {
			entry.Error = fmt.Sprintf("loading draft: %v", err)
			response.Failed = append(response.Failed, entry)
			continue
		}

		if err := applyRevisionToK8s(ctx, a, handler.Model, env, component, draft.Values); err != nil {
			entry.Error = fmt.Sprintf("applying to k8s: %v", err)
			response.Failed = append(response.Failed, entry)
			continue
		}
		if err := draft.MarkDeployed(ctx, a.MongoDB); err != nil {
			entry.Error = fmt.Sprintf("marking deployed: %v", err)
			response.Failed = append(response.Failed, entry)
			continue
		}
		entry.RevisionID = draft.ID.Hex()
		entry.Version = draft.Version
		response.Deployed = append(response.Deployed, entry)
	}
	c.JSON(http.StatusOK, response)
}

// loadComponentEnv resolves application, component, and environment from the
// gin context, enforcing ownership and parent-app linkage. Returns ok=false
// (and emits the right HTTP error) on any miss.
func loadComponentEnv(c *gin.Context, a *ApiHandler) (*ApplicationHandler, *models.Component, *models.Environment, bool) {
	handler, err := getHandlerFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return nil, nil, nil, false
	}
	component, err := getComponentFromRoute(c, a.MongoDB)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return nil, nil, nil, false
	}
	if component.ApplicationID != handler.Model.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return nil, nil, nil, false
	}
	env, err := handler.Model.GetEnvironmentByName(a.MongoDB, c.Param("environment"))
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return nil, nil, nil, false
	}
	return handler, component, env, true
}

// applyRevisionToK8s gathers per-scope variables, builds the manifest, and
// applies it through the conure provider. Shared by DeployRevision,
// DeployRevisionByID, and the bulk deploy.
func applyRevisionToK8s(ctx context.Context, a *ApiHandler, application *models.Application, env *models.Environment, component *models.Component, values map[string]interface{}) error {
	return applyRevisionToK8sWithAnnotations(ctx, a, application, env, component, values, nil)
}

// applyRevisionToK8sWithAnnotations is the same as applyRevisionToK8s but
// merges extra annotations onto the Component CRD's metadata before apply.
// Used by RestartComponent to stamp conure.io/restartedAt; could be reused
// later for any one-shot annotation-driven trigger.
func applyRevisionToK8sWithAnnotations(ctx context.Context, a *ApiHandler, application *models.Application, env *models.Environment, component *models.Component, values map[string]interface{}, extraAnnotations map[string]string) error {
	cv, err := gatherVariables(a.MongoDB, application, env, component, a.KeyStorage)
	if err != nil {
		return err
	}
	app := BuildApplicationCRD(application, env)
	componentCRD, err := BuildComponentCRD(application, env, component, values)
	if err != nil {
		return err
	}
	if len(extraAnnotations) > 0 {
		if componentCRD.Annotations == nil {
			componentCRD.Annotations = map[string]string{}
		}
		for k, v := range extraAnnotations {
			componentCRD.Annotations[k] = v
		}
	}
	provider := newConureProvider(application, env)

	// Resolve the org's component definition from Mongo (the source of
	// truth) and materialize it into the target cluster before applying the
	// Component. This is what guarantees the CRD is present whenever a
	// deploy happens — the controller resolves it by (type, engine) scoped
	// to the org-id label and must never race a missing or stale row. A
	// missing definition is a request error, surfaced up-front rather than
	// as a later reconcile failure with no API-side signal.
	def, err := models.ResolveOneForOrg(ctx, a.MongoDB, application.OrganizationID, component.Type, component.Engine)
	if err != nil {
		return err
	}
	credResolver := &providers.CredentialResolver{DB: a.MongoDB, KeyStorage: a.KeyStorage}
	if err = provider.EnsureComponentDefinition(ctx, credResolver, def); err != nil {
		return fmt.Errorf("materializing component definition for type %q: %w", component.Type, err)
	}

	// Ensure the workload's image pull Secret exists in the env namespace
	// BEFORE applying the Component — otherwise the pod is created first and
	// ImagePullBackOffs while the Secret races in. Driven by the component's
	// optional image.credentialRef field role; empty → public image, no-op.
	pullRef, err := fieldroles.New(def.Buildable, def.FieldRoles).GetOptional(values, fieldroles.RoleImageCredentialRef)
	if err != nil {
		return fmt.Errorf("reading %s for pull secret: %w", fieldroles.RoleImageCredentialRef, err)
	}
	if pullRef != "" {
		clientset, kerr := a.kubeClient()
		if kerr != nil {
			return fmt.Errorf("getting kube client for pull secret projection: %w", kerr)
		}
		if perr := credResolver.EnsurePullSecret(ctx, clientset, application.OrganizationID, pullRef, env.GetNamespace()); perr != nil {
			return perr
		}
	}

	return provider.ApplyComponent(ctx, app, componentCRD, cv)
}

// buildEnvDetail assembles the env-scoped response for one component:
// drift, last-deployed snapshot, latest draft, health condition rollup.
func buildEnvDetail(ctx context.Context, a *ApiHandler, component *models.Component, env *models.Environment, live *conurev1alpha1.Component) (ComponentInEnvResponse, error) {
	resp := ComponentInEnvResponse{
		ComponentID:     component.ID.Hex(),
		Name:            component.Name,
		Type:            component.Type,
		Engine:          component.Engine,
		EnvironmentID:   env.ID,
		EnvironmentName: env.Name,
	}

	deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return resp, err
	}
	if deployed != nil {
		resp.DeployedRevision = deployed
	}

	draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return resp, err
	}
	if draft != nil {
		resp.LatestDraft = draft
	}

	if live != nil {
		liveValues, err := LiveValuesFromComponent(live)
		if err != nil {
			return resp, err
		}
		resp.LiveValues = liveValues

		var deployedValues map[string]interface{}
		if deployed != nil {
			deployedValues = deployed.Values
		}
		report, err := ComputeDrift(liveValues, deployedValues)
		if err != nil {
			return resp, err
		}
		resp.Drifted = report.Drifted
		resp.Diff = report.Diff

		if cond := readyCondition(live); cond != nil {
			resp.HealthCondition = cond.Type
			resp.HealthStatus = string(cond.Status)
			resp.HealthMessage = cond.Message
		}
	}
	return resp, nil
}

// readyCondition extracts the rollup Ready condition from a live Component
// CRD, returning nil when it hasn't been written yet.
func readyCondition(comp *conurev1alpha1.Component) *metav1.Condition {
	if comp == nil {
		return nil
	}
	for i := range comp.Status.Conditions {
		cond := comp.Status.Conditions[i]
		if cond.Type == conurev1alpha1.ComponentConditionTypeReady.String() {
			return &cond
		}
	}
	return nil
}

// autoImportComment is stamped on every revision created by absorbDriftIfAny
// so consumers can tell at a glance that the values came from the cluster,
// not from an API deploy. Pairs with CreatedBy == primitive.NilObjectID.
const autoImportComment = "auto-imported from cluster state"

// ensureComponentIdentity returns the Mongo identity row for a live CRD,
// creating one if no row matches (app.ID, live.Name). Used by the env-scoped
// list path to onboard Component CRDs that exist in the cluster but have no
// API-side record yet (e.g. someone kubectl-applied a Component directly).
//
// Idempotent under races: if Create loses to a concurrent caller, the row is
// re-read instead of failing.
func ensureComponentIdentity(ctx context.Context, a *ApiHandler, app *models.Application, live *conurev1alpha1.Component) (*models.Component, error) {
	component := &models.Component{}
	err := component.GetByApplicationAndName(ctx, a.MongoDB, app.ID, live.Name)
	if err == nil {
		return component, nil
	}
	if !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil, err
	}
	component = &models.Component{
		Name:          live.Name,
		Type:          live.Spec.ComponentType,
		Description:   live.Annotations["conure.io/description"],
		ApplicationID: app.ID,
	}
	if err := component.Create(a.MongoDB); err != nil {
		if !errors.Is(err, conureerrors.ErrObjectAlreadyExists) {
			return nil, err
		}
		// Lost the race with another caller — re-read.
		component = &models.Component{}
		if err := component.GetByApplicationAndName(ctx, a.MongoDB, app.ID, live.Name); err != nil {
			return nil, err
		}
	}
	return component, nil
}

// absorbDriftIfAny records a new deployed revision when the live CRD's
// Spec.Values differs from the last deployed revision in Mongo. Keeps Mongo's
// "latest deployed" honest about what's actually in the cluster, regardless
// of how it got there (API deploy, kubectl edit, anything else).
//
// CreatedBy is NilObjectID and Comment is autoImportComment because the
// caller of this endpoint is not who made the change — the change happened
// out-of-band in k8s. No-op when the live CRD already matches the last
// deployed revision, or when there is no prior deployed revision (callers
// that need first-time onboarding should use ensureComponentIdentity).
func absorbDriftIfAny(ctx context.Context, a *ApiHandler, component *models.Component, env *models.Environment, live *conurev1alpha1.Component) error {
	liveValues, err := LiveValuesFromComponent(live)
	if err != nil {
		return err
	}
	deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
	if errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	report, err := ComputeDrift(liveValues, deployed.Values)
	if err != nil {
		return err
	}
	if !report.Drifted {
		return nil
	}
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        liveValues,
		Comment:       autoImportComment,
		CreatedBy:     primitive.NilObjectID,
	}
	return rev.CreateDeployed(ctx, a.MongoDB)
}
