package workflow

import (
	"context"
	"fmt"
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

const (
	OwnerKey = ".metadata.controller"
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
	if err := r.Get(ctx, nsn, &app); err != nil {
		logger.Info("Application resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	var wflw conurev1alpha1.Workflow
	nsn = types.NamespacedName{
		Namespace: req.Namespace,
		Name:      wflr.Spec.WorkflowName,
	}
	if err := r.Get(ctx, nsn, &wflw); err != nil {
		logger.Info("Workflow resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if r.isFinished(&wflr) {
		return ctrl.Result{}, nil
	}

	actionsHandler := NewActionsHandler(ctx, wflr.Namespace, &wflw, &wflr, r)
	actions := actionsHandler.GetActions()
	// Get the last action
	lastAction := actions[len(actions)-1]

	// Find any action job running
	var childJobs batchv1.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{OwnerKey: req.Name}); err != nil {
		logger.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}
	for _, job := range childJobs.Items {
		labels := job.GetLabels()

		// If there is still an active job, mark as running and return
		if job.Status.Active > 0 {
			message := fmt.Sprintf("Running action %s", job.GetLabels()[conurev1alpha1.WorkflowActionNamelabel])
			if err := r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeRunningAction, metav1.ConditionTrue, conurev1alpha1.RunningActionReason, message); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		for _, condition := range job.Status.Conditions {
			// If the job is completed, check if it was the last action and set the condition
			if condition.Type == batchv1.JobComplete && labels[conurev1alpha1.WorkflowActionNamelabel] == lastAction.Name {
				if err := r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeFinished, metav1.ConditionTrue, conurev1alpha1.FinishedSuccesfullyReason, "Finished"); err != nil {
					return ctrl.Result{}, err
				}
			} else if condition.Type == batchv1.JobFailed {
				if err := r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeFinished, metav1.ConditionFalse, conurev1alpha1.FinishedFailedReason, "Failed"); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	// If there are no active jobs and the run is not marked as finished, run the actions
	if !r.isFinished(&wflr) && !r.isRunning(&wflr) {
		err := r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeRunningAction, metav1.ConditionTrue, conurev1alpha1.RunningActionReason, "Running actions")
		if err != nil {
			return ctrl.Result{}, err
		}
		err = actionsHandler.RunActions()
		if err != nil {
			if err = r.setCondition(ctx, &wflr, conurev1alpha1.ConditionTypeFinished, metav1.ConditionFalse, conurev1alpha1.FinishedFailedReason, "Failed to run actions"); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *WorkflowReconciler) setCondition(ctx context.Context, wflr *conurev1alpha1.WorkflowRun, conditionType conurev1alpha1.WorkflowConditionType, status metav1.ConditionStatus, reason conurev1alpha1.WorkflowConditionReason, message string) error {
	var newConditions []metav1.Condition
	wflr.Status.Conditions = common.SetCondition(newConditions, string(conditionType), status, string(reason), message)
	err := r.Status().Update(ctx, wflr)
	if err != nil {
		return err
	}
	return nil
}

func (r *WorkflowReconciler) isFinished(wflr *conurev1alpha1.WorkflowRun) bool {
	_, exists := common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeFinished.String())
	return exists
}
func (r *WorkflowReconciler) isRunning(wflr *conurev1alpha1.WorkflowRun) bool {
	_, exists := common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeRunningAction.String())
	return exists
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &batchv1.Job{}, OwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*batchv1.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		apiGVStr := conurev1alpha1.GroupVersion.String()
		if owner.APIVersion != apiGVStr || owner.Kind != "WorkflowRun" {
			return nil
		}
		r := []string{owner.Name}
		return r
	}); err != nil {
		return err
	}

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
