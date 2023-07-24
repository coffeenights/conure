package workload

import (
	"fmt"
	oamconureiov1alpha1 "github.com/coffeenights/conure/api/oam/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ServiceWorkloadBuilder(application *oamconureiov1alpha1.Application, component *oamconureiov1alpha1.Component, properties *oamconureiov1alpha1.ServiceComponentProperties) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.Name,
			Namespace: application.Namespace,
			Annotations: map[string]string{
				"oam.conure.io/application.component": fmt.Sprintf("%s.%s", application.Name, component.Name),
			},
			Labels: map[string]string{
				"oam.conure.io/application": application.Name,
				"oam.conure.io/component":   component.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(component.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"application": application.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application": application.Name,
						"component":   component.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    component.Name,
							Image:   properties.Image,
							Command: properties.Command,
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{},
	}
	return deployment, nil
}

func int32Ptr(i int32) *int32 { return &i }
