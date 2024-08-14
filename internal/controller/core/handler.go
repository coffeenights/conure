package controller

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Workload interface {
	Build() error
	SetControllerReference(scheme *runtime.Scheme) error
	Reconcile(context.Context, client.Client) error
}

type ApplicationHandler struct {
	Application *coreconureiov1alpha1.Application
	Reconciler  *ApplicationReconciler
	Ctx         context.Context
	Logger      logr.Logger
	Workloads   []Workload
}

func NewApplicationHandler(ctx context.Context, application *coreconureiov1alpha1.Application, reconciler *ApplicationReconciler) (*ApplicationHandler, error) {
	var handler ApplicationHandler
	handler.Logger = log.FromContext(ctx)
	handler.Application = application
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	return &handler, nil
}

func (a *ApplicationHandler) ReconcileWorkloads() error {
	for _, wld := range a.Workloads {
		err := wld.Reconcile(a.Ctx, a.Reconciler.Client)
		if err != nil {
			return err
		}
	}
	return nil
}
