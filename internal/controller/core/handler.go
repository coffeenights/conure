package controller

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/workflow"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	actionsHandler, err := workflow.NewActionsHandler(a.Ctx, a.Application.Namespace)
	if err != nil {
		return err
	}
	err = actionsHandler.GetActions(component.ComponentType)
	if err != nil {
		return err
	}
	err = actionsHandler.RunActions()
	if err != nil {
		return err
	}
	return nil
}
