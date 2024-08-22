package workflow

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
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
	if !wflr.Status.Finished {
		actionsHandler, err := NewActionsHandler(ctx, wflr.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.GetActions(wflr.Spec.WorkflowName)
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
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := WorkflowReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}
