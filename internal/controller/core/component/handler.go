package component

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ComponentHandler struct {
	Component  *coreconureiov1alpha1.Component
	Reconciler *ComponentReconciler
	Ctx        context.Context
	Logger     logr.Logger
}

func NewApplicationHandler(ctx context.Context, component *coreconureiov1alpha1.Component, reconciler *ComponentReconciler) (*ComponentHandler, error) {
	var handler ComponentHandler
	handler.Logger = log.FromContext(ctx)
	handler.Component = component
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	return &handler, nil
}

func (a *ComponentHandler) findWorkflow() error {
	//wflRaw := &unstructured.Unstructured{}
	//for _, applySet := range applySets {
	//	for _, obj := range applySet.Objects {
	//		if obj.GetKind() == "Workflow" {
	//			wflRaw = obj.DeepCopy()
	//		}
	//	}
	//}
	return nil
}

func (a *ComponentHandler) CompareWorkflow(componentTemplate *module.Manager) error {
	//applySets, err := componentTemplate.GetApplySets()
	//if err != nil {
	//	return err
	//}
	//
	//// Find the workflow manifest
	//wflRaw := &unstructured.Unstructured{}
	//for _, applySet := range applySets {
	//	for _, obj := range applySet.Objects {
	//		if obj.GetKind() == "Workflow" {
	//			wflRaw = obj.DeepCopy()
	//		}
	//	}
	//}
	return nil
}

func (a *ComponentHandler) ReconcileComponent() error {
	return nil
}
