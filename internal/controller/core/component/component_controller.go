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
		logger.Info("Component resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	componentHandler := NewComponentHandler(ctx, &component, r)
	if err := componentHandler.ReconcileComponent(); err != nil {
		return ctrl.Result{}, err
	}
	//index, exists := common.ContainsCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeWorkflow.String())
	//if exists && component.Status.Conditions[index].Reason == conurev1alpha1.ComponentWorkFlowSucceedReason.String() {
	//	if err := r.reconcileResources(ctx, &component, req.Namespace); err != nil {
	//		return ctrl.Result{}, err
	//	}
	//}
	return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
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
