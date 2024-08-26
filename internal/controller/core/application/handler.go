package application

import (
	"context"
	"encoding/json"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type ApplicationHandler struct {
	Application *coreconureiov1alpha1.Application
	Reconciler  *ApplicationReconciler
	Ctx         context.Context
	Logger      logr.Logger
}

func NewApplicationHandler(ctx context.Context, application *coreconureiov1alpha1.Application, reconciler *ApplicationReconciler) (*ApplicationHandler, error) {
	var handler ApplicationHandler
	handler.Logger = log.FromContext(ctx)
	handler.Application = application
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	return &handler, nil
}

func (a *ApplicationHandler) ReconcileComponents() error {

	for _, component := range a.Application.Spec.Components {
		err := a.ReconcileComponent(&component)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *ApplicationHandler) ReconcileComponent(componentTemp *coreconureiov1alpha1.ComponentTemplate) error {
	logger := log.FromContext(a.Ctx)
	logger.Info("Reconciling component", "component", componentTemp.Name)

	var (
		metadata          metav1.ObjectMeta
		component         coreconureiov1alpha1.Component
		existingComponent coreconureiov1alpha1.Component
	)

	metadata.Name = componentTemp.Name
	metadata.Labels = componentTemp.Labels
	metadata.Annotations = componentTemp.Annotations
	component.ObjectMeta = metadata
	component.ObjectMeta.Namespace = a.Application.Namespace
	component.Spec = componentTemp.Spec
	component.TypeMeta = metav1.TypeMeta{
		Kind:       "Component",
		APIVersion: a.Application.APIVersion,
	}
	err := a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: metadata.Name}, &existingComponent)
	if apierrors.IsNotFound(err) {
		logger.Info("Creating component", "component", component.Name)
		err = a.Reconciler.Create(a.Ctx, &component)
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	logger.Info("Updating component", "component", component.Name)
	rv := existingComponent.GetResourceVersion()
	component.SetResourceVersion(rv)
	err = a.Reconciler.Update(a.Ctx, &component)
	if err != nil {
		return err
	}
	return nil
}

func (a *ApplicationHandler) ReconcileComponentOld(component *coreconureiov1alpha1.ComponentTemplate) error {
	logger := log.FromContext(a.Ctx)
	logger.Info("Reconciling component", "component", component.Name)

	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Spec.Values)
	if err != nil {
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	err = d.Decode(&values)
	if err != nil {
		return err
	}
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, a.Application.Namespace, "", values.Get())
	if err != nil {
		return err
	}
	_, err = componentTemplate.Build()
	if err != nil {
		return err
	}
	err = componentTemplate.Apply()
	if err != nil {
		return err
	}
	return nil
}
