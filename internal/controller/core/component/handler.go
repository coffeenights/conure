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
	"strings"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ComponentHandler struct {
	Component         *conurev1alpha1.Component
	Reconciler        *ComponentReconciler
	Ctx               context.Context
	Logger            logr.Logger
	componentTemplate timoni.ModuleManager
	applySet          []*unstructured.Unstructured
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
	// Look up the ComponentDefinition by the component's type
	compDefList := &conurev1alpha1.ComponentDefinitionList{}
	if err := c.Reconciler.List(c.Ctx, compDefList); err != nil {
		return fmt.Errorf("failed to list component definitions: %w", err)
	}

	var compDef *conurev1alpha1.ComponentDefinition
	for i := range compDefList.Items {
		if compDefList.Items[i].Spec.ComponentType == c.Component.Spec.ComponentType {
			compDef = &compDefList.Items[i]
			break
		}
	}
	if compDef == nil {
		return fmt.Errorf("component definition not found for type %q", c.Component.Spec.ComponentType)
	}

	// Transform the values to a map
	valuesJSON, err := json.Marshal(c.Component.Spec.Values)
	if err != nil {
		c.Logger.Error(err, "failed to marshal component values")
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))

	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	if err = d.Decode(&values); err != nil {
		c.Logger.Error(err, "failed to decode component values")
		return err
	}
	creds, err := resolveRegistryCredentials(c.Ctx, c.Reconciler.Client, compDef.Spec.RegistrySecretRef, compDef.Spec.OCIRepository)
	if err != nil {
		c.Logger.Error(err, "failed to resolve registry credentials", "ociRepository", compDef.Spec.OCIRepository)
		return err
	}
	if creds == "" {
		c.Logger.Info("no registrySecretRef set on ComponentDefinition, attempting anonymous pull", "componentDefinition", compDef.Name, "ociRepository", compDef.Spec.OCIRepository)
	}
	mgr, err := module.NewManager(c.Ctx, c.Component.Name, "oci://"+compDef.Spec.OCIRepository, compDef.Spec.OCITag, c.Component.Namespace, creds, false, values.Get())
	if err != nil {
		c.Logger.Error(err, "failed to initialize template manager", "ociRepository", compDef.Spec.OCIRepository, "ociTag", compDef.Spec.OCITag)
		return err
	}
	mgr.CacheDir = timoniCacheDir()
	c.componentTemplate = mgr
	sets, err := c.componentTemplate.GetApplySets()
	if err != nil {
		c.Logger.Error(err, "failed to get apply sets")
		return err
	}

	// Verify the OCI digest matches the expected value from the ComponentDefinition
	if compDef.Spec.OCIDigest != "" {
		resolvedDigest := c.componentTemplate.GetDigest()
		if resolvedDigest != compDef.Spec.OCIDigest {
			return fmt.Errorf("OCI digest mismatch for component type %q: expected %s, got %s", compDef.Spec.ComponentType, compDef.Spec.OCIDigest, resolvedDigest)
		}
	}

	// Add the spec hashes to every object and add them to the apply set
	for _, set := range sets {
		for _, o := range set.Objects {
			hash := common.GetHashForSpec(o.Object["spec"].(map[string]interface{}))
			labels := common.SetHashToLabels(o.GetLabels(), hash)
			o.SetLabels(labels)
			c.applySet = append(c.applySet, o)
		}
	}
	// Compress, encode and add the sets to the component annotations
	setsJSON, err := c.componentTemplate.MarshalApplySets(sets)
	if err != nil {
		c.Logger.Error(err, "failed to marshal apply sets")
		return err
	}
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err = gzipWriter.Write(setsJSON)
	if err != nil {
		c.Logger.Error(err, "failed to compress apply sets")
		return err
	}
	if err = gzipWriter.Close(); err != nil {
		c.Logger.Error(err, "failed to close gzip writer")
		return err
	}
	compressedData := buf.Bytes()
	setsBase64 := base64.StdEncoding.EncodeToString(compressedData)
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
	if err = c.Reconciler.Patch(c.Ctx, c.Component, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
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
		if _, err := c.componentTemplate.ApplyObject(resource, false); err != nil {
			c.Logger.Error(err, "failed to apply resource", "kind", resource.GetKind(), "name", resource.GetName(), "namespace", resource.GetNamespace())
			return fmt.Errorf("failed to apply %s/%s: %w", resource.GetKind(), resource.GetName(), err)
		}
	}
	// Clear the apply set
	c.applySet = nil
	return nil
}

func (c *ComponentHandler) ReconcileDeployedObjects() error {
	if c.componentTemplate == nil {
		manager, err := module.NewManager(c.Ctx, c.Component.Name, "", "", c.Component.Namespace, "", true, map[string]interface{}{})
		if err != nil {
			c.Logger.Error(err, "failed to initialize template manager for reconciliation")
			return err
		}
		c.componentTemplate = manager
	}

	annotations := c.Component.GetAnnotations()
	if annotations[conurev1alpha1.ApplySetsAnnotation] != "" {
		decoded, err := base64.StdEncoding.DecodeString(annotations[conurev1alpha1.ApplySetsAnnotation])
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

		setsJSON, err := io.ReadAll(reader)
		if err != nil {
			c.Logger.Error(err, "failed to read decompressed apply sets")
			return err
		}
		sets, err := c.componentTemplate.UnmarshalApplySets(setsJSON)
		if err != nil {
			c.Logger.Error(err, "failed to unmarshal apply sets")
			return err
		}
		for _, set := range sets {
			c.applySet = append(c.applySet, set.Objects...)
		}
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
