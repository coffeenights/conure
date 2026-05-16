package component

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/coffeenights/conure/internal/render"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ComponentHandler struct {
	Component  *conurev1alpha1.Component
	Reconciler *ComponentReconciler
	Ctx        context.Context
	Logger     logr.Logger
	// engine is the render.Engine selected for this component instance. It is
	// set by renderComponent (full render path) or ReconcileDeployedObjects
	// (drift path) before applyResources is invoked.
	engine   render.Engine
	applySet []*unstructured.Unstructured
}

var orderMap = map[string]int{
	"Namespace":                1,
	"ResourceQuota":            2,
	"LimitRange":               3,
	"PodSecurityPolicy":        4,
	"Secret":                   5,
	"ConfigMap":                6,
	"StorageClass":             7,
	"PersistentVolume":         8,
	"PersistentVolumeClaim":    9,
	"ServiceAccount":           10,
	"CustomResourceDefinition": 11,
	"ClusterRole":              12,
	"ClusterRoleBinding":       13,
	"Role":                     14,
	"RoleBinding":              15,
	"Service":                  16,
	"DaemonSet":                17,
	"Pod":                      18,
	"ReplicationController":    19,
	"ReplicaSet":               20,
	"Deployment":               21,
	"StatefulSet":              22,
	"Job":                      23,
	"CronJob":                  24,
}

func timoniCacheDir() string {
	if dir := os.Getenv("CONURE_TIMONI_CACHE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "conure-timoni-cache")
}

func NewComponentHandler(ctx context.Context, component *conurev1alpha1.Component, reconciler *ComponentReconciler) *ComponentHandler {
	return &ComponentHandler{
		Component:  component,
		Reconciler: reconciler,
		Ctx:        ctx,
		Logger:     log.FromContext(ctx),
	}
}

func (c *ComponentHandler) renderComponent() error {
	compDef, err := c.lookupComponentDefinition()
	if err != nil {
		return err
	}

	builder, err := c.Reconciler.SelectBuilder(compDef)
	if err != nil {
		return err
	}

	creds, err := resolveRegistryCredentials(c.Ctx, c.Reconciler.Client, compDef.Spec.RegistrySecretRef, compDef.Spec.OCIRepository)
	if err != nil {
		c.Logger.Error(err, "failed to resolve registry credentials", "ociRepository", compDef.Spec.OCIRepository)
		return err
	}
	if creds == "" {
		c.Logger.V(1).Info("no registrySecretRef set on ComponentDefinition, attempting anonymous pull", "componentDefinition", compDef.Name, "ociRepository", compDef.Spec.OCIRepository)
	}

	eng, err := builder.Build(c.Ctx, compDef, c.Component, creds)
	if err != nil {
		c.Logger.Error(err, "failed to initialize render engine", "engine", engineOf(compDef), "ociRepository", compDef.Spec.OCIRepository, "ociTag", compDef.Spec.OCITag)
		return err
	}
	c.engine = eng

	sets, err := c.engine.Render(c.Ctx)
	if err != nil {
		c.Logger.Error(err, "failed to render apply sets")
		return err
	}

	if compDef.Spec.OCIDigest != "" {
		resolvedDigest := c.engine.Digest()
		if resolvedDigest != compDef.Spec.OCIDigest {
			return fmt.Errorf("OCI digest mismatch for component type %q: expected %s, got %s", compDef.Spec.ComponentType, compDef.Spec.OCIDigest, resolvedDigest)
		}
	}

	// Propagate conure identity labels onto every rendered child (and into pod
	// templates of workload kinds), then add the spec hash. Labels must be
	// applied before hashing so the hash reflects the final spec.
	conureLabels := common.SelectConureLabels(c.Component.GetLabels())
	restartAnnotations := common.SelectRestartAnnotations(c.Component.GetAnnotations())
	for _, set := range sets {
		for _, o := range set.Objects {
			if err := common.PropagateLabelsToRendered(o, conureLabels); err != nil {
				return fmt.Errorf("propagating conure labels to %s/%s: %w", o.GetKind(), o.GetName(), err)
			}
			if err := common.PropagateAnnotationsToPodTemplate(o, restartAnnotations); err != nil {
				return fmt.Errorf("propagating restart annotations to %s/%s: %w", o.GetKind(), o.GetName(), err)
			}
			hash := common.GetHashForSpec(o.Object["spec"].(map[string]interface{}))
			labels := common.SetHashToLabels(o.GetLabels(), hash)
			o.SetLabels(labels)
			c.applySet = append(c.applySet, o)
		}
	}

	if err := c.writeApplySetsAnnotation(engineOf(compDef), sets); err != nil {
		return err
	}
	return nil
}

// lookupComponentDefinition finds the ComponentDefinition matching the
// component's spec.type. When the Component pins spec.engine, the match must
// also agree on engine (timoni vs helm); otherwise a single type match wins
// and finding more than one match is reported as an ambiguity the user must
// resolve by setting spec.engine.
//
// ComponentDefinitions are cluster-scoped and a definition's spec.type is not
// unique across orgs (two orgs can each ship a "webservice"). The Component
// carries its org id as a label; the API materializes each org's definitions
// with the same OrganizationIDLabel. Scoping the list by that label is what
// keeps one org's reconcile from matching another org's definition and
// failing as a false "ambiguous" — the controller stays org-unaware, it only
// propagates a label it already received onto the lookup. An empty/absent
// label falls back to a cluster-wide list, preserving behavior for any
// Component not created by the API.
func (c *ComponentHandler) lookupComponentDefinition() (*conurev1alpha1.ComponentDefinition, error) {
	compDefList := &conurev1alpha1.ComponentDefinitionList{}
	var listOpts []client.ListOption
	if orgID := c.Component.GetLabels()[k8sUtils.OrganizationIDLabel]; orgID != "" {
		listOpts = append(listOpts, client.MatchingLabels{k8sUtils.OrganizationIDLabel: orgID})
	}
	if err := c.Reconciler.List(c.Ctx, compDefList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list component definitions: %w", err)
	}

	wantEngine := c.Component.Spec.Engine
	var matches []*conurev1alpha1.ComponentDefinition
	for i := range compDefList.Items {
		if compDefList.Items[i].Spec.ComponentType != c.Component.Spec.ComponentType {
			continue
		}
		if wantEngine != "" && engineOf(&compDefList.Items[i]) != wantEngine {
			continue
		}
		matches = append(matches, &compDefList.Items[i])
	}
	switch len(matches) {
	case 0:
		if wantEngine != "" {
			return nil, fmt.Errorf("component definition not found for type %q with engine %q", c.Component.Spec.ComponentType, wantEngine)
		}
		return nil, fmt.Errorf("component definition not found for type %q", c.Component.Spec.ComponentType)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("%d ComponentDefinitions match type %q; set spec.engine on the Component to disambiguate", len(matches), c.Component.Spec.ComponentType)
	}
}

// writeApplySetsAnnotation marshals the apply sets through the engine, wraps
// them in the versioned envelope tagged with the engine identifier, compresses
// and base64-encodes the result, and persists it under
// ApplySetsAnnotation. The envelope lets the drift sweep dispatch
// UnmarshalApplySets to the right adapter on the next reconcile.
func (c *ComponentHandler) writeApplySetsAnnotation(engineName conurev1alpha1.ComponentEngine, sets []render.ResourceSet) error {
	setsJSON, err := c.engine.MarshalApplySets(sets)
	if err != nil {
		c.Logger.Error(err, "failed to marshal apply sets")
		return err
	}
	envelope, err := render.WrapEnvelope(engineName, setsJSON)
	if err != nil {
		c.Logger.Error(err, "failed to wrap apply-set envelope")
		return err
	}
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(envelope); err != nil {
		c.Logger.Error(err, "failed to compress apply sets")
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		c.Logger.Error(err, "failed to close gzip writer")
		return err
	}
	setsBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	if c.Component.Annotations == nil {
		c.Component.Annotations = make(map[string]string)
	}
	c.Component.Annotations[conurev1alpha1.ApplySetsAnnotation] = setsBase64
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": c.Component.GetAnnotations(),
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		c.Logger.Error(err, "failed to marshal annotation patch")
		return err
	}
	if err := c.Reconciler.Patch(c.Ctx, c.Component, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		c.Logger.Error(err, "failed to patch component annotations")
		return err
	}
	return nil
}

// bufferCondition mutates Component.Status.Conditions in memory only;
// callers must invoke flushStatus to persist the result. The reason is
// validated against the known set so typos surface at runtime.
func (c *ComponentHandler) bufferCondition(conditionType conurev1alpha1.ComponentConditionType, status metav1.ConditionStatus, reason conurev1alpha1.ComponentConditionReason, message string) error {
	if !reason.IsValid() {
		return fmt.Errorf("unknown ComponentConditionReason %q", reason)
	}
	c.Component.Status.Conditions = common.SetCondition(c.Component.Status.Conditions, conditionType.String(), status, reason.String(), message)
	return nil
}

func (c *ComponentHandler) flushStatus() error {
	return common.ApplyStatus(c.Ctx, c.Component, c.Reconciler.Client)
}

// rollupReady recomputes the Ready condition from the Rendered and Deployed
// conditions. Ready=True only when both are True; otherwise it carries the
// most informative failure reason.
func (c *ComponentHandler) rollupReady() {
	rendered := c.GetCondition(conurev1alpha1.ComponentConditionTypeRendered)
	deployed := c.GetCondition(conurev1alpha1.ComponentConditionTypeDeployed)

	switch {
	case rendered != nil && rendered.Status == metav1.ConditionFalse:
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeReady, metav1.ConditionFalse, conurev1alpha1.ComponentReasonRenderingFailed, rendered.Message)
	case deployed != nil && deployed.Status == metav1.ConditionFalse:
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeReady, metav1.ConditionFalse, conurev1alpha1.ComponentReasonDeploymentFailed, deployed.Message)
	case rendered != nil && rendered.Status == metav1.ConditionTrue && deployed != nil && deployed.Status == metav1.ConditionTrue:
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeReady, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady, "Component is ready")
	default:
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeReady, metav1.ConditionFalse, conurev1alpha1.ComponentReasonInProgress, "Reconciliation in progress")
	}
}

func (c *ComponentHandler) GetCondition(conditionType conurev1alpha1.ComponentConditionType) *metav1.Condition {
	index, exists := common.ContainsCondition(c.Component.Status.Conditions, conditionType.String())
	if exists {
		return &c.Component.Status.Conditions[index]
	}
	return nil
}

// GetConditionReady is a convenience wrapper around GetCondition for the Ready
// condition; preserved as a stable accessor for callers that read the rollup.
func (c *ComponentHandler) GetConditionReady() *metav1.Condition {
	return c.GetCondition(conurev1alpha1.ComponentConditionTypeReady)
}

// RenderComponent renders the component template and applies resources.
// Intermediate state is buffered in memory and the Ready rollup is computed
// from per-step Rendered/Deployed conditions, so a successful or failed
// reconcile produces a single status write.
func (c *ComponentHandler) RenderComponent() error {
	if err := c.renderComponent(); err != nil {
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeRendered, metav1.ConditionFalse, conurev1alpha1.ComponentReasonFailed, err.Error())
		c.rollupReady()
		if flushErr := c.flushStatus(); flushErr != nil {
			return flushErr
		}
		return err
	}
	_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeRendered, metav1.ConditionTrue, conurev1alpha1.ComponentReasonSucceeded, "Component rendered successfully")

	if err := c.applyResources(); err != nil {
		_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeDeployed, metav1.ConditionFalse, conurev1alpha1.ComponentReasonFailed, err.Error())
		c.rollupReady()
		if flushErr := c.flushStatus(); flushErr != nil {
			return flushErr
		}
		return err
	}
	_ = c.bufferCondition(conurev1alpha1.ComponentConditionTypeDeployed, metav1.ConditionTrue, conurev1alpha1.ComponentReasonSucceeded, "Component deployed successfully")
	c.rollupReady()
	c.Component.Status.ObservedGeneration = c.Component.Generation
	c.Component.Status.ObservedRestartedAt = c.Component.GetAnnotations()[conurev1alpha1.RestartedAtAnnotation]
	return c.flushStatus()
}

// applyResources applies the resources in the applySet to the cluster only if they have changed since the last apply or if they are new.
// The configuration drift detection is done by the fluxcd/pkg/ssa package.
func (c *ComponentHandler) applyResources() error {
	sort.SliceStable(c.applySet, func(i, j int) bool {
		return orderMap[c.applySet[i].GetKind()] < orderMap[c.applySet[j].GetKind()]
	})

	gvk, err := apiutil.GVKForObject(c.Component, c.Reconciler.Scheme)
	if err != nil {
		return fmt.Errorf("failed to get GVK for component: %w", err)
	}
	ownerRef := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               c.Component.GetName(),
		UID:                c.Component.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}

	for _, resource := range c.applySet {
		resource.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
		if _, err := c.engine.ApplyObject(c.Ctx, resource, false); err != nil {
			c.Logger.Error(err, "failed to apply resource", "kind", resource.GetKind(), "name", resource.GetName(), "namespace", resource.GetNamespace())
			return fmt.Errorf("failed to apply %s/%s: %w", resource.GetKind(), resource.GetName(), err)
		}
	}
	// Clear the apply set
	c.applySet = nil
	return nil
}

func (c *ComponentHandler) ReconcileDeployedObjects() error {
	annotations := c.Component.GetAnnotations()
	rawAnn := annotations[conurev1alpha1.ApplySetsAnnotation]
	if rawAnn == "" {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(rawAnn)
	if err != nil {
		c.Logger.Error(err, "failed to decode apply sets annotation")
		return err
	}
	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		c.Logger.Error(err, "failed to decompress apply sets")
		return err
	}
	defer reader.Close()

	envelopeBytes, err := io.ReadAll(reader)
	if err != nil {
		c.Logger.Error(err, "failed to read decompressed apply sets")
		return err
	}

	engineName, payload, err := render.ParseEnvelope(envelopeBytes)
	if err != nil {
		c.Logger.Error(err, "failed to parse apply-set envelope")
		return err
	}

	// The engine field on the handler may already be set (tests pre-populate
	// it to inject a mock). When not set, dispatch on the envelope-declared
	// engine so the drift sweep applies through the same adapter that
	// originally rendered the cached set.
	if c.engine == nil {
		builder, err := c.Reconciler.SelectBuilderByEngine(engineName)
		if err != nil {
			return err
		}
		eng, err := builder.BuildForApply(c.Ctx, c.Component)
		if err != nil {
			c.Logger.Error(err, "failed to initialize engine for drift sweep", "engine", engineName)
			return err
		}
		c.engine = eng
	}

	sets, err := c.engine.UnmarshalApplySets(payload)
	if err != nil {
		c.Logger.Error(err, "failed to unmarshal apply sets")
		return err
	}
	for _, set := range sets {
		c.applySet = append(c.applySet, set.Objects...)
	}
	if c.applySet == nil {
		return nil
	}

	if err := c.applyResources(); err != nil {
		// Reflect the apply failure in the status; the caller (Reconcile) then
		// requeues without invoking RenderComponent.
		if bufErr := c.bufferCondition(conurev1alpha1.ComponentConditionTypeDeployed, metav1.ConditionFalse, conurev1alpha1.ComponentReasonFailed, err.Error()); bufErr != nil {
			return bufErr
		}
		c.rollupReady()
		if flushErr := c.flushStatus(); flushErr != nil {
			return flushErr
		}
		return err
	}
	return nil
}
