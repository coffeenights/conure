package controller

import (
	"context"
	"encoding/json"
	"github.com/coffeenights/conure/internal/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oamconureiov1alpha1 "github.com/coffeenights/conure/api/oam/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=oam.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oam.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oam.conure.io,resources=applications/finalizers,verbs=update

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var application oamconureiov1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		logger.Error(err, "unable to fetch Application")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, component := range application.Spec.Components {
		switch component.Type {
		case oamconureiov1alpha1.Service:
			componentProperties := oamconureiov1alpha1.ServiceComponentProperties{}
			err := json.Unmarshal(component.Properties.Raw, &componentProperties)
			if err != nil {
				return ctrl.Result{}, err
			}

			deployment, err := workload.ServiceWorkloadBuilder(&application, &component, &componentProperties)
			if err != nil {
				logger.Error(err, "unable to construct deployment from template")
				scheduledResult := ctrl.Result{RequeueAfter: time.Hour}
				return scheduledResult, err
			}
			err = ctrl.SetControllerReference(&application, deployment, r.Scheme)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Check if the deployment exists
			var existingDeployment appsv1.Deployment
			objectKey := client.ObjectKeyFromObject(deployment)
			if err = r.Get(ctx, objectKey, &existingDeployment); err != nil {
				if apierrors.IsNotFound(err) {
					err = r.Create(ctx, deployment)
					if err != nil {
						logger.Error(err, "Unable to create deployment for application", "deployment", deployment)
						return ctrl.Result{}, err
					}
					logger.V(1).Info("Created Deployment for Application run", "deployment", deployment)
				} else {
					return ctrl.Result{}, err
				}
			} else {
				specHashTarget := GetHashForSpec(deployment.Spec)
				specHashActual := GetHashFromLabels(existingDeployment.Labels)
				if specHashActual != specHashTarget {
					deployment.Labels = SetHashToLabels(deployment.Labels, specHashTarget)
					err = r.Update(ctx, deployment)
					if err != nil {
						logger.Error(err, "Unable to create deployment for application", "deployment", deployment)
						return ctrl.Result{}, err
					}
					logger.V(1).Info("Updated Deployment for Application run", "deployment", deployment)
				}
			}

		case oamconureiov1alpha1.StatefulService:
		case oamconureiov1alpha1.CronTask:
		case oamconureiov1alpha1.Worker:
		}

	}

	//var deployments appsv1.DeploymentList
	//if err := r.List(ctx, &deployments, client.InNamespace(req.Namespace), client.MatchingFields{".metadata.controller": req.Name}); err != nil {
	//	log.Error(err, "unable to list child Jobs")
	//	return ctrl.Result{}, err
	//}

	//deploymentExists := false
	//var currentDeployment appsv1.Deployment
	//// Check if the deployment already exists
	//for _, currentDeployment = range deployments.Items {
	//	if currentDeployment.ObjectMeta.Name == req.Name {
	//		deploymentExists = true
	//		break
	//	}
	//}

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
		apiGVStr := oamconureiov1alpha1.GroupVersion.String()
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
		For(&oamconureiov1alpha1.Application{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
