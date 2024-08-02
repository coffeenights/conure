package workload

import (
	"context"
	"fmt"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ServiceWorkload struct {
	Deployment  *appsv1.Deployment
	Service     *corev1.Service
	Application *coreconureiov1alpha1.Application
	Component   *coreconureiov1alpha1.Component
	Properties  *coreconureiov1alpha1.ServiceComponentProperties
}

func (s *ServiceWorkload) Build() error {
	s.Deployment = s.buildDeployment()
	s.Service = s.buildService()
	return nil
}

func (s *ServiceWorkload) SetControllerReference(scheme *runtime.Scheme) error {
	err := ctrl.SetControllerReference(s.Application, s.Deployment, scheme)
	if err != nil {
		return err
	}
	err = ctrl.SetControllerReference(s.Application, s.Service, scheme)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServiceWorkload) buildDeployment() *appsv1.Deployment {
	replicas := s.Component.Replicas
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", s.Application.Name, s.Component.Name),
			Namespace: s.Application.Namespace,
			Annotations: map[string]string{
				"core.conure.io/application.component": fmt.Sprintf("%s.%s", s.Application.Name, s.Component.Name),
			},
			Labels: map[string]string{
				"core.conure.io/application":   s.Application.Name,
				"core.conure.io/component":     s.Component.Name,
				"app.kubernetes.io/name":       s.Component.Name,
				"app.kubernetes.io/part-of":    s.Application.Name,
				"app.kubernetes.io/managed-by": "Conure",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"core.conure.io/application": s.Application.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"core.conure.io/application":   s.Application.Name,
						"core.conure.io/component":     s.Component.Name,
						"app.kubernetes.io/name":       s.Component.Name,
						"app.kubernetes.io/part-of":    s.Application.Name,
						"app.kubernetes.io/managed-by": "Conure",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    s.Component.Name,
							Image:   s.Properties.Image,
							Command: s.Properties.Command,
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{},
	}
	return &deployment
}

func (s *ServiceWorkload) buildService() *corev1.Service {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", s.Application.Name, s.Component.Name),
			Namespace: s.Application.Namespace,
			Annotations: map[string]string{
				"core.conure.io/application.component": fmt.Sprintf("%s.%s", s.Application.Name, s.Component.Name),
			},
			Labels: map[string]string{
				"core.conure.io/application":   s.Application.Name,
				"core.conure.io/component":     s.Component.Name,
				"app.kubernetes.io/name":       s.Component.Name,
				"app.kubernetes.io/part-of":    s.Application.Name,
				"app.kubernetes.io/managed-by": "Conure",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:     s.Properties.Port,
				Protocol: "TCP",
				TargetPort: intstr.IntOrString{
					IntVal: s.Properties.TargetPort,
				},
			}},
			Type: "LoadBalancer",
			Selector: map[string]string{
				"core.conure.io/application": s.Application.Name,
				"core.conure.io/component":   s.Component.Name,
			},
		},
		Status: corev1.ServiceStatus{},
	}
	return service
}

func (s *ServiceWorkload) reconcileDeployment(ctx context.Context, c client.Client) error {
	// Check if the deployment exists
	var existingDeployment appsv1.Deployment
	logger := log.FromContext(ctx)
	objectKey := client.ObjectKeyFromObject(s.Deployment)
	if err := c.Get(ctx, objectKey, &existingDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			err = c.Create(ctx, s.Deployment)
			if err != nil {
				logger.Error(err, "Unable to create deployment for application", "deployment", s.Deployment)
				return err
			}
			logger.V(1).Info("Created Deployment for Application run", "deployment", s.Deployment)
		} else {
			return err
		}
	} else {
		specHashTarget := GetHashForSpec(s.Deployment.Spec)
		specHashActual := GetHashFromLabels(existingDeployment.Labels)
		if specHashActual != specHashTarget {
			s.Deployment.Labels = SetHashToLabels(s.Deployment.Labels, specHashTarget)
			err = c.Update(ctx, s.Deployment)
			if err != nil {
				logger.Error(err, "Unable to create deployment for application", "deployment", s.Deployment)
				return err
			}
			logger.V(1).Info("Updated Deployment for Application run", "deployment", s.Deployment)
		}
	}
	return nil
}

func (s *ServiceWorkload) reconcileService(ctx context.Context, c client.Client) error {
	var existingService corev1.Service
	logger := log.FromContext(ctx)
	objectKey := client.ObjectKeyFromObject(s.Service)
	if err := c.Get(ctx, objectKey, &existingService); err != nil {
		if apierrors.IsNotFound(err) {
			err = c.Create(ctx, s.Service)
			if err != nil {
				logger.Error(err, "Unable to create service for application", "service", s.Service)
				return err
			}
			logger.V(1).Info("Created Service for Application run", "service", s.Service)
		} else {
			return err
		}
	} else {
		specHashTarget := GetHashForSpec(s.Service.Spec)
		specHashActual := GetHashFromLabels(existingService.Labels)
		if specHashActual != specHashTarget {
			s.Deployment.Labels = SetHashToLabels(s.Service.Labels, specHashTarget)
			err = c.Update(ctx, s.Service)
			if err != nil {
				logger.Error(err, "Unable to create service for application", "service", s.Service)
				return err
			}
			logger.V(1).Info("Updated Service for Application run", "service", s.Service)
		}
	}
	return nil
}

func (s *ServiceWorkload) Reconcile(ctx context.Context, c client.Client) error {
	err := s.reconcileDeployment(ctx, c)
	if err != nil {
		return err
	}

	err = s.reconcileService(ctx, c)
	if err != nil {
		return err
	}

	return nil
}
