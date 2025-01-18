package component

import (
	"context"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const RequeueAfter = time.Minute * 3

type ComponentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/finalizers,verbs=update

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var component conurev1alpha1.Component
	if err := r.Get(ctx, req.NamespacedName, &component); err != nil {
		logger.V(1).Info("Component resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile deployed objects
	componentHandler := NewComponentHandler(ctx, &component, r)
	if err := componentHandler.ReconcileDeployedObjects(); err != nil {
		return ctrl.Result{}, err
	}

	if err := componentHandler.RenderComponent(); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&conurev1alpha1.Component{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := ComponentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}
