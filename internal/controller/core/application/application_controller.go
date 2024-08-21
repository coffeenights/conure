package application

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.conure.io,resources=applications/finalizers,verbs=update

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var application coreconureiov1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		logger.Info("Application resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	handler, err := NewApplicationHandler(ctx, &application, r)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = handler.ReconcileComponents()
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &appsv1.Deployment{}, ".metadata.controller", func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		deployment := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deployment)
		if owner == nil {
			return nil
		}
		apiGVStr := coreconureiov1alpha1.GroupVersion.String()
		if owner.APIVersion != apiGVStr || owner.Kind != "Application" {
			return nil
		}

		// ...and if so, return it
		r := []string{owner.Name}
		return r
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&coreconureiov1alpha1.Application{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := ApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}