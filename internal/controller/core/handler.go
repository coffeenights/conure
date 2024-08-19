package controller

import (
	"context"
	"encoding/json"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	// Check if the application exists
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

	// Pull the component template
	//advancedValues := timoni.Values{}
	//if component.Values.Advanced != nil {
	//	if err := advancedValues.ExtractFromRawExtension(component.Values.Advanced); err != nil {
	//		return err
	//	}
	//}

	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Values)
	if err != nil {
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	d.UseNumber()
	err = d.Decode(&values)
	if err != nil {
		return err
	}
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.OCIRepository, component.OCITag, a.Application.Namespace, "", values.Get())
	if err != nil {
		return err
	}

	applySets, err := componentTemplate.GetApplySets()
	if err != nil {
		return err
	}
	wflw := &unstructured.Unstructured{}
	logger.Info("Applying workflow", "workflow", component.Name)
	for _, applySet := range applySets {
		for _, obj := range applySet.Objects {
			if obj.GetKind() == "Workflow" {
				wflw = obj
				break
			}
		}
	}
	// Update workflow manifest
	_, err = componentTemplate.ApplyObject(wflw, false)
	if err != nil {
		return err
	}

	// Determine if the workflow should run
	//actionsHandler, err := workflow.NewActionsHandler(a.Ctx, a.Application.Namespace)
	//if err != nil {
	//	return err
	//}
	//err = actionsHandler.GetActions(component.Name)
	//if err != nil {
	//	return err
	//}
	//err = actionsHandler.RunActions()
	//if err != nil {
	//	return err
	//}
	return nil
}
