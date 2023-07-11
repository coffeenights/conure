/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	oamconureiov1alpha1 "github.com/coffeenights/conure/api/oam/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=oam.conure.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=oam.conure.io,resources=applications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=oam.conure.io,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var application oamconureiov1alpha1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		log.Error(err, "unable to fetch Application")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var deployments appsv1.DeploymentList

	if err := r.List(ctx, &deployments, client.InNamespace(req.Namespace), client.MatchingFields{".metadata.controller": req.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}
	var deployment appsv1.Deployment
	deploymentExists := false
	// Check if the deployment already exists
	for _, deployment = range deployments.Items {
		if deployment.ObjectMeta.Name == req.Name {
			deploymentExists = true
			break
		}
	}

	if deploymentExists {
		log.V(1).Info("Deployment for Application run", "deployment", deployment)
	} else {
		scheduledResult := ctrl.Result{RequeueAfter: time.Hour}
		deployment, err := constructDeployment(&application)
		if err != nil {
			log.Error(err, "unable to construct deployment from template")
			// don't bother requeuing until we get a change to the spec
			return scheduledResult, nil
		}

		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Unable to create deployment for application", "deployment", deployment)
			return ctrl.Result{}, err
		}
		log.V(1).Info("created Deployment for Application run", "deployment", deployment)
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
		// ...make sure it's a CronJob...
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
		Complete(r)
}

func constructDeployments(application *oamconureiov1alpha1.Application) ([]*appsv1.Deployment, error) {

	var deployments []*appsv1.Deployment
	for _, component := range application.Spec.Components {
		deployment := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name,
				Namespace: application.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"application": req.Name,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"application": req.Name,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  component.Name,
								Image: component.Properties[0].Image,
								Command: []string{
									"/bin/sh",
									"-c",
									"sleep 3600",
								},
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{},
		}
		deployments = append(deployments, deployment)
	}

	if err := ctrl.SetControllerReference(application, deployment, r.Scheme); err != nil {
		return nil, err
	}
	return deployment, nil
}

func int32Ptr(i int32) *int32 { return &i }
