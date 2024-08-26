package workflow

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkflowReconciler reconciles an WorkflowRun object
type WorkflowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *WorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var wflr coreconureiov1alpha1.WorkflowRun
	if err := r.Get(ctx, req.NamespacedName, &wflr); err != nil {
		logger.Info("WorkflowRun resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var app coreconureiov1alpha1.Application
	nsn := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      wflr.Spec.ApplicationName,
	}
	err := r.Get(ctx, nsn, &app)
	if err != nil {
		logger.Info("Application resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var wflw coreconureiov1alpha1.Workflow
	nsn = types.NamespacedName{
		Namespace: req.Namespace,
		Name:      wflr.Spec.WorkflowName,
	}
	err = r.Get(ctx, nsn, &wflw)
	if err != nil {
		logger.Info("Workflow resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !wflr.Status.Finished {
		actionsHandler, err := NewActionsHandler(ctx, r.Client, wflr.Namespace, &wflw)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.GetActions()
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.RunActions()
		if err != nil {
			return ctrl.Result{}, err
		}
		wflr.Status.Finished = true
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coreconureiov1alpha1.WorkflowRun{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := WorkflowReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}
