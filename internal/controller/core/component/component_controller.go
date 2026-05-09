package component

import (
	"context"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const RequeueAfter = time.Minute * 3

type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/finalizers,verbs=update

// isReconciledUpToDate returns true when the component's spec.generation has
// already been observed and the current condition indicates a terminal-success
// state. When this returns true, the reconciler can skip rendering and applying.
func isReconciledUpToDate(c *conurev1alpha1.Component) bool {
	if c.Status.ObservedGeneration != c.Generation {
		return false
	}
	idx, ok := common.ContainsCondition(c.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String())
	if !ok {
		return false
	}
	reason := c.Status.Conditions[idx].Reason
	return reason == conurev1alpha1.ComponentReadyDeployingSucceedReason.String() ||
		reason == conurev1alpha1.ComponentReadyRunningReason.String()
}

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var component conurev1alpha1.Component
	if err := r.Get(ctx, req.NamespacedName, &component); err != nil {
		logger.V(1).Info("Component resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isReconciledUpToDate(&component) {
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	componentHandler := NewComponentHandler(ctx, &component, r)

	if err := componentHandler.ReconcileDeployedObjects(); err != nil {
		logger.Info("Failed to reconcile deployed objects", "error", err)
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	if err := componentHandler.RenderComponent(); err != nil {
		logger.Info("Failed to render component", "error", err)
		// Condition is set with the error message — don't requeue, wait for spec change
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager. Status-only
// updates are filtered via GenerationChangedPredicate so that the reconciler's
// own status patches don't trigger fresh reconciles.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&conurev1alpha1.Component{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := ComponentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}
