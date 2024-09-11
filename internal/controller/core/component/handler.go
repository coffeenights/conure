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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type ComponentHandler struct {
	Component         *conurev1alpha1.Component
	Reconciler        *ComponentReconciler
	Ctx               context.Context
	Logger            logr.Logger
	ComponentTemplate *module.Manager
	applySet          []*unstructured.Unstructured
}

func NewComponentHandler(ctx context.Context, component *conurev1alpha1.Component, reconciler *ComponentReconciler) *ComponentHandler {
	var handler ComponentHandler
	handler.Logger = log.FromContext(ctx)
	handler.Component = component
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	return &handler
}
func (c *ComponentHandler) reconcileResources(ctx context.Context, component *conurev1alpha1.Component, namespace string) error {
	if err := c.renderComponent(ctx, component, namespace); err != nil {
		return err
	}
	var reconcile []*unstructured.Unstructured
	for _, o := range c.applySet {
		var existingResource unstructured.Unstructured
		if err := c.Reconciler.Get(ctx, client.ObjectKey{Namespace: namespace, Name: o.GetName()}, &existingResource); err != nil {
			if errors.IsNotFound(err) {
				reconcile = append(reconcile, o)
			} else if err != nil {
				return err
			} else {
				//if c.hasResourceChanged(o, &existingResource) {
				//	reconcile = append(reconcile, o)
				//}
			}
		}
	}
	return nil
}

func (c *ComponentHandler) renderComponent(ctx context.Context, component *conurev1alpha1.Component, namespace string) error {
	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Spec.Values)
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
	componentTemplate, err := module.NewManager(ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, namespace, "", values.Get())
	if err != nil {
		return err
	}
	sets, err := componentTemplate.GetApplySets()
	if err != nil {
		return err
	}
	for _, set := range sets {
		for _, o := range set.Objects {
			c.applySet = append(c.applySet, o)
		}
	}
	return nil
}

func (c *ComponentHandler) setConditionReady(ctx context.Context, component *conurev1alpha1.Component, reason conurev1alpha1.ComponentConditionReason, message string) error {
	status := metav1.ConditionFalse
	if reason == conurev1alpha1.ComponentReadyRunningReason {
		status = metav1.ConditionTrue
	}
	component.Status.Conditions = common.SetCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String(), status, reason.String(), message)
	return c.Reconciler.Status().Update(ctx, component)
}

func (c *ComponentHandler) ReconcileComponent() error {
	return nil
}
