package component

import (
	"context"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// newHandler builds the ComponentHandler used during Reconcile. Tests
	// override this to inject a mock componentTemplate so the OCI pull and
	// SSA apply paths can be exercised against a fake client.
	newHandler func(context.Context, *conurev1alpha1.Component, *ComponentReconciler) *ComponentHandler
}

func (r *ComponentReconciler) handlerFor(ctx context.Context, c *conurev1alpha1.Component) *ComponentHandler {
	if r.newHandler != nil {
		return r.newHandler(ctx, c, r)
	}
	return NewComponentHandler(ctx, c, r)
}

//+kubebuilder:rbac:groups=core.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/finalizers,verbs=update

// isReconciledUpToDate returns true when the component's spec.generation has
// already been observed and the Ready rollup is True. When this returns true,
// the reconciler can skip rendering and applying.
func isReconciledUpToDate(c *conurev1alpha1.Component) bool {
	if c.Status.ObservedGeneration != c.Generation {
		return false
	}
	idx, ok := common.ContainsCondition(c.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String())
	if !ok {
		return false
	}
	return c.Status.Conditions[idx].Status == metav1.ConditionTrue
}

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var component conurev1alpha1.Component
	if err := r.Get(ctx, req.NamespacedName, &component); err != nil {
		logger.V(1).Info("Component resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	componentHandler := r.handlerFor(ctx, &component)

	// Re-apply the cached apply-set on every reconcile. fluxcd/pkg/ssa
	// (via timoni's ApplyObject) diffs live state against each object and
	// recreates / reverts drift, so the periodic requeue is the drift-detection
	// loop. This path is cheap — no OCI pull, no Timoni render — so it runs
	// even when spec.generation hasn't changed.
	if err := componentHandler.ReconcileDeployedObjects(); err != nil {
		logger.Info("Failed to reconcile deployed objects", "error", err)
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	// Skip the expensive render path when spec.generation hasn't changed and
	// Ready is already True; drift was just handled above.
	if isReconciledUpToDate(&component) {
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
