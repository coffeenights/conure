package component

import (
	"context"
	"encoding/json"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/stefanprodan/timoni/pkg/module"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
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
	var component coreconureiov1alpha1.Component
	if err := r.Get(ctx, req.NamespacedName, &component); err != nil {
		logger.Info("Component resource not found.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//conditions := common.ConureConditions{}
	//conditions = append(conditions, metav1.Condition{
	//	Type:               common.TypeRunning.String(),
	//	Status:             metav1.ConditionTrue,
	//	Reason:             common.ReasonFailedApply.String(),
	//	LastTransitionTime: metav1.Time{Time: time.Now()},
	//	Message:            "Component is running",
	//})
	//component.Status.Conditions = conditions
	//err := r.Status().Update(ctx, &component)
	//if err != nil {
	//	return ctrl.Result{}, err
	//}

	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Spec.Values)
	if err != nil {
		return ctrl.Result{}, err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	err = d.Decode(&values)
	if err != nil {
		return ctrl.Result{}, err
	}
	componentTemplate, err := module.NewManager(ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, req.Namespace, "", values.Get())
	if err != nil {
		return ctrl.Result{}, err
	}
	_ = componentTemplate
	_, err = componentTemplate.Build()
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coreconureiov1alpha1.Component{}).
		Owns(&coreconureiov1alpha1.WorkflowRun{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager) error {
	reconciler := ComponentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	return reconciler.SetupWithManager(mgr)
}
