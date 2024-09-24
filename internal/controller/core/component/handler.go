package component

import (
	"context"
	"encoding/json"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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
	for _, set := range sets {
		for _, o := range set.Objects {
			hash := common.GetHashForSpec(o.Object["spec"].(map[string]interface{}))
			labels := common.SetHashToLabels(o.GetLabels(), hash)
			o.SetLabels(labels)
			c.applySet = append(c.applySet, o)
		}
	}
	return nil
}

func (c *ComponentHandler) hasResourceChanged(resource *unstructured.Unstructured) bool {
	var obj unstructured.Unstructured
	if err := c.Reconciler.Get(c.Ctx, types.NamespacedName{Namespace: c.Component.Namespace, Name: resource.GetName()}, &obj); err != nil {
		if errors.IsNotFound(err) {
			return true
		}
	}
	existingHash := common.GetHashFromLabels(obj.GetLabels())
	newHash := common.GetHashFromLabels(resource.GetLabels())
	return existingHash != newHash
}

func (c *ComponentHandler) setConditionReady(reason conurev1alpha1.ComponentConditionReason, message string) error {
	status := metav1.ConditionFalse
	if reason == conurev1alpha1.ComponentReadyRunningReason {
		status = metav1.ConditionTrue
	}
	c.Component.Status.Conditions = common.SetCondition(c.Component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String(), status, reason.String(), message)
	return c.Reconciler.Status().Update(c.Ctx, c.Component)
}

func (c *ComponentHandler) GetConditionReady() *metav1.Condition {
	index, exists := common.ContainsCondition(c.Component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String())
	if exists {
		return &c.Component.Status.Conditions[index]
	}
	return nil
}

func (c *ComponentHandler) ReconcileComponent() error {
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

func (c *ComponentHandler) applyResources() error {
	sort.SliceStable(c.applySet, func(i, j int) bool {
		return orderMap[c.applySet[i].GetKind()] < orderMap[c.applySet[j].GetKind()]
	})
	if err := c.setConditionReady(conurev1alpha1.ComponentReadyDeployingReason, "Deploying Component"); err != nil {
		return err
	}
	for _, resource := range c.applySet {
		if c.hasResourceChanged(resource) {
			_, err := c.componentTemplate.ApplyObject(resource, false)
			if err != nil {
				if err2 := c.setConditionReady(conurev1alpha1.ComponentReadyDeployingFailedReason, "Component failed to deploy"); err2 != nil {
					return err2
				}
				return err
			}
		}
	}
	if err := c.setConditionReady(conurev1alpha1.ComponentReadyDeployingSucceedReason, "Component deployed succesfully"); err != nil {
		return err
	}
	return nil
}
