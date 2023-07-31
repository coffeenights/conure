package controller

import (
	"context"
	"encoding/json"
	oamconureiov1alpha1 "github.com/coffeenights/conure/api/oam/v1alpha1"
	"github.com/coffeenights/conure/internal/workload"
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
	Application *oamconureiov1alpha1.Application
	Reconciler  *ApplicationReconciler
	Ctx         context.Context
	Workloads   []Workload
}

func NewApplicationHandler(ctx context.Context, application *oamconureiov1alpha1.Application, reconciler *ApplicationReconciler) (*ApplicationHandler, error) {
	logger := log.FromContext(ctx)
	var handler ApplicationHandler
	handler.Application = application
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	for _, component := range application.Spec.Components {
		switch component.Type {
		case oamconureiov1alpha1.Service:
			componentProperties := oamconureiov1alpha1.ServiceComponentProperties{}
			err := json.Unmarshal(component.Properties.Raw, &componentProperties)
			if err != nil {
				return &handler, err
			}
			wld := workload.ServiceWorkload{
				Application: application,
				Component:   &component,
				Properties:  &componentProperties,
			}
			err = wld.Build()
			if err != nil {
				logger.Error(err, "unable to construct workload from Application template")
				return &handler, err
			}
			err = wld.SetControllerReference(reconciler.Scheme)
			if err != nil {
				logger.Error(err, "unable to set the reference back to the controller in the workload")
				return &handler, err
			}
			handler.Workloads = append(handler.Workloads, &wld)
		case oamconureiov1alpha1.StatefulService:
		case oamconureiov1alpha1.CronTask:
		case oamconureiov1alpha1.Worker:
		}
	}
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
