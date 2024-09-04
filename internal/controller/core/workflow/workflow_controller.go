package workflow

import (
	"context"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	var wflr conurev1alpha1.WorkflowRun
	if err := r.Get(ctx, req.NamespacedName, &wflr); err != nil {
		logger.Info("WorkflowRun resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var app conurev1alpha1.Application
	nsn := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      wflr.Spec.ApplicationName,
	}
	err := r.Get(ctx, nsn, &app)
	if err != nil {
		logger.Info("Application resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var wflw conurev1alpha1.Workflow
	nsn = types.NamespacedName{
		Namespace: req.Namespace,
		Name:      wflr.Spec.WorkflowName,
	}
	err = r.Get(ctx, nsn, &wflw)
	if err != nil {
		logger.Info("Workflow resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !r.isFinished(&wflr) {
		actionsHandler, err := NewActionsHandler(ctx, wflr.Namespace, &wflw, r)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.GetActions()
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeRunningAction, conurev1alpha1.RunningActionReason, "Running actions")
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.RunActions()
		if err != nil {
			condErr := r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeFinished, conurev1alpha1.FinishedFailedReason, "Failed to run actions")
			if condErr != nil {
				logger.Error(condErr, "Failed to set condition")
			}
			return ctrl.Result{}, err
		}
		err = r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeFinished, conurev1alpha1.FinishedSuccesfullyReason, "Finished")
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) setCondition(ctx context.Context, wflr *conurev1alpha1.WorkflowRun, conditionType conurev1alpha1.WorkflowConditionType, reason conurev1alpha1.WorkflowConditionReason, message string) error {
	var newConditions []metav1.Condition
	wflr.Status.Conditions = common.SetCondition(newConditions, string(conditionType), metav1.ConditionTrue, string(reason), message)
	err := r.Status().Update(ctx, wflr)
	if err != nil {
		return err
	}
	return nil
}

func (r *WorkflowReconciler) isFinished(wflr *conurev1alpha1.WorkflowRun) bool {
	index, exists := common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeFinished.String())
	if exists && wflr.Status.Conditions[index].Reason == conurev1alpha1.FinishedSuccesfullyReason.String() {
		return true
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&conurev1alpha1.WorkflowRun{}).
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
