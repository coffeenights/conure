package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
)

type K8sDeploymentWorkloadType struct {
	Name      string
	SchemaRef *appsv1.Deployment
}
