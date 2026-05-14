package component

import (
	"context"
	"fmt"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"github.com/coffeenights/conure/internal/render"
	"github.com/coffeenights/conure/internal/render/apply"
	helmengine "github.com/coffeenights/conure/internal/render/helm"
	timoniengine "github.com/coffeenights/conure/internal/render/timoni"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const RequeueAfter = time.Minute * 3

type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Builders maps each supported engine to its render.Builder. Populated at
	// controller startup. The handler picks a builder via SelectBuilder (render
	// path) or SelectBuilderByEngine (drift path).
	Builders map[conurev1alpha1.ComponentEngine]render.Builder

	// newHandler builds the ComponentHandler used during Reconcile. Tests
	// override this to inject a mock engine so the OCI pull and SSA apply
	// paths can be exercised against a fake client.
	newHandler func(context.Context, *conurev1alpha1.Component, *ComponentReconciler) *ComponentHandler
}

// engineOf returns the engine label set on a ComponentDefinition, defaulting
// to EngineTimoni so ComponentDefinitions written before the multi-engine
// refactor continue to work.
func engineOf(def *conurev1alpha1.ComponentDefinition) conurev1alpha1.ComponentEngine {
	if def.Spec.Engine == "" {
		return conurev1alpha1.EngineTimoni
	}
	return def.Spec.Engine
}

// SelectBuilder returns the render.Builder configured for a ComponentDefinition.
func (r *ComponentReconciler) SelectBuilder(def *conurev1alpha1.ComponentDefinition) (render.Builder, error) {
	return r.SelectBuilderByEngine(engineOf(def))
}

// SelectBuilderByEngine returns the render.Builder for a given engine label.
// The drift sweep uses this with the engine label decoded from the apply-set
// envelope, since it doesn't load the ComponentDefinition.
func (r *ComponentReconciler) SelectBuilderByEngine(eng conurev1alpha1.ComponentEngine) (render.Builder, error) {
	b, ok := r.Builders[eng]
	if !ok {
		return nil, fmt.Errorf("no render.Builder registered for engine %q", eng)
	}
	return b, nil
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
// already been observed, the restart annotation matches the last observed
// value, and the Ready rollup is True. When this returns true, the reconciler
// can skip rendering and applying.
func isReconciledUpToDate(c *conurev1alpha1.Component) bool {
	if c.Status.ObservedGeneration != c.Generation {
		return false
	}
	if c.Status.ObservedRestartedAt != c.GetAnnotations()[conurev1alpha1.RestartedAtAnnotation] {
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

// restartAnnotationChangedPredicate fires only when the restart annotation
// value differs between old and new. Combined with GenerationChangedPredicate,
// it lets a metadata-only restart wake the reconciler without also reacting
// to every other annotation write (e.g. the controller's own ApplySets patch).
type restartAnnotationChangedPredicate struct {
	predicate.Funcs
}

func (restartAnnotationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return e.ObjectOld.GetAnnotations()[conurev1alpha1.RestartedAtAnnotation] !=
		e.ObjectNew.GetAnnotations()[conurev1alpha1.RestartedAtAnnotation]
}

// SetupWithManager sets up the controller with the Manager. Status-only
// updates are filtered via GenerationChangedPredicate so that the reconciler's
// own status patches don't trigger fresh reconciles. Restart-annotation
// changes are admitted through a sibling predicate so a metadata-only restart
// still wakes the reconciler.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&conurev1alpha1.Component{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			restartAnnotationChangedPredicate{},
		)).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	// The Helm engine applies through a shared SSA manager so its
	// drift-detection semantics match Timoni's, using a distinct
	// FieldManager identity so the two engines don't conflict in mixed
	// clusters. The Timoni adapter keeps its upstream apply path (and
	// its own FieldManager) for back-compat with already-deployed
	// components.
	helmApplier, err := apply.New(mgr.GetConfig(), mgr.GetScheme())
	if err != nil {
		return fmt.Errorf("building shared apply manager: %w", err)
	}
	reconciler := ComponentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Builders: map[conurev1alpha1.ComponentEngine]render.Builder{
			conurev1alpha1.EngineTimoni: timoniengine.NewBuilder(timoniCacheDir()),
			conurev1alpha1.EngineHelm:   helmengine.NewBuilder(helmApplier),
		},
	}
	return reconciler.SetupWithManager(mgr)
}
