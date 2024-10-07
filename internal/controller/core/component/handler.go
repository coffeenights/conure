package component

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sort"
	"strings"
)

type ComponentHandler struct {
	Component         *conurev1alpha1.Component
	Reconciler        *ComponentReconciler
	Ctx               context.Context
	Logger            logr.Logger
	componentTemplate *module.Manager
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

func NewComponentHandler(ctx context.Context, component *conurev1alpha1.Component, reconciler *ComponentReconciler) *ComponentHandler {
	return &ComponentHandler{
		Component:  component,
		Reconciler: reconciler,
		Ctx:        ctx,
		Logger:     log.FromContext(ctx),
	}
}

func (c *ComponentHandler) renderComponent() error {
	// Transform the values to a map
	valuesJSON, err := json.Marshal(c.Component.Spec.Values)
	if err != nil {
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))

	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	if err = d.Decode(&values); err != nil {
		return err
	}
	c.componentTemplate, err = module.NewManager(c.Ctx, c.Component.Name, c.Component.Spec.OCIRepository, c.Component.Spec.OCITag, c.Component.Namespace, "", true, values.Get())
	if err != nil {
		return err
	}
	sets, err := c.componentTemplate.GetApplySets()
	if err != nil {
		return err
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
		return err
	}
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err = gzipWriter.Write(setsJSON)
	if err != nil {
		return err
	}
	if err = gzipWriter.Close(); err != nil {
		return err
	}
	compressedData := buf.Bytes()
	setsBase64 := base64.StdEncoding.EncodeToString(compressedData)
	// Create a patch with the new annotations
	c.Component.Annotations[conurev1alpha1.ApplySetsAnnotation] = setsBase64
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": c.Component.GetAnnotations(),
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	if err = c.Reconciler.Patch(c.Ctx, c.Component, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return err
	}
	return nil
}

func (c *ComponentHandler) setConditionReady(reason conurev1alpha1.ComponentConditionReason, message string) error {
	status := metav1.ConditionFalse
	if reason == conurev1alpha1.ComponentReadyRunningReason {
		status = metav1.ConditionTrue
	}
	currentCondition := c.GetConditionReady()
	if currentCondition != nil && currentCondition.Status == status && currentCondition.Reason == string(reason) && currentCondition.Message == message {
		return nil
	}
	c.Component.Status.Conditions = common.SetCondition(c.Component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String(), status, reason.String(), message)
	return common.ApplyStatus(c.Ctx, c.Component, c.Reconciler.Client)
}

func (c *ComponentHandler) GetConditionReady() *metav1.Condition {
	index, exists := common.ContainsCondition(c.Component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String())
	if exists {
		return &c.Component.Status.Conditions[index]
	}
	return nil
}

func (c *ComponentHandler) RenderComponent() error {
	if err := c.setConditionReady(conurev1alpha1.ComponentReadyRenderingReason, "Component is being rendered"); err != nil {
		return err
	}
	if err := c.renderComponent(); err != nil {
		if err2 := c.setConditionReady(conurev1alpha1.ComponentReadyRenderingFailedReason, "Component failed to render"); err2 != nil {
			return err2
		}
		return err
	}
	if err := c.setConditionReady(conurev1alpha1.ComponentReadyRenderingSucceedReason, "Component rendered successfully"); err != nil {
		return err
	}
	return c.applyResources()
}

// applyResources applies the resources in the applySet to the cluster only if they have changed since the last apply or if they are new.
// The configuration drift detection is done by the fluxcd/pkg/ssa package.
func (c *ComponentHandler) applyResources() error {
	sort.SliceStable(c.applySet, func(i, j int) bool {
		return orderMap[c.applySet[i].GetKind()] < orderMap[c.applySet[j].GetKind()]
	})
	for _, resource := range c.applySet {
		_, err := c.componentTemplate.ApplyObject(resource, false)
		if err != nil {
			if err2 := c.setConditionReady(conurev1alpha1.ComponentReadyDeployingFailedReason, "Component failed to deploy"); err2 != nil {
				return err2
			}
			return err
		}
	}
	// Clear the apply set
	c.applySet = nil
	return nil
}

func (c *ComponentHandler) ReconcileDeployedObjects() error {
	annotations := c.Component.GetAnnotations()
	if annotations[conurev1alpha1.ApplySetsAnnotation] != "" {
		decoded, err := base64.StdEncoding.DecodeString(annotations[conurev1alpha1.ApplySetsAnnotation])
		if err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return err
		}
		defer reader.Close()

		setsJSON, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		sets, err := c.componentTemplate.UnmarshalApplySets(setsJSON)
		if err != nil {
			return err
		}
		for _, set := range sets {
			c.applySet = append(c.applySet, set.Objects...)
		}
	}
	if c.applySet == nil {
		return nil
	}
	c.updateStatus()

	// Apply the resources
	manager, err := module.NewManager(c.Ctx, c.Component.Name, c.Component.Spec.OCIRepository, c.Component.Spec.OCITag, c.Component.Namespace, "", true, map[string]interface{}{})
	if err != nil {
		return err
	}
	c.componentTemplate = manager
	return c.applyResources()
}

func (c *ComponentHandler) updateStatus() error {
	// Update the status with the current objects
	for _, obj := range c.applySet {
		if obj.GetKind() == "Deployment" {
			deployment := &appsv1.Deployment{}
			if err := c.Reconciler.Get(c.Ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, deployment); err != nil {

			}
			if deployment.Status.ReadyReplicas == deployment.Status.Replicas {
				if err := c.setConditionReady(conurev1alpha1.ComponentReadyRunningReason, "Component is running"); err != nil {
					c.Logger.Error(err, "Failed to set condition ready")
					return err
				}
			}
		}
	}
	return nil
}
