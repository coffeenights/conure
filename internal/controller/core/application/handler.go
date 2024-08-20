package application

import (
	"context"
	"encoding/json"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
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

func (a *ApplicationHandler) ReconcileComponent(component *coreconureiov1alpha1.Component) error {
	logger := log.FromContext(a.Ctx)
	logger.Info("Reconciling component", "component", component.Name)

	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Values)
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
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.OCIRepository, component.OCITag, a.Application.Namespace, "", values.Get())
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
