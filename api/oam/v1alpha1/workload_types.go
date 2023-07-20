package v1alpha1

import appsv1 "k8s.io/api/apps/v1"

type WorkloadType string

const (
	Deployment WorkloadType = "deployment"
)

type DeploymentType struct {
	SchemaRef *appsv1.Deployment
}
