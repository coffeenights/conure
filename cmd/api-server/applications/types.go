package applications

import (
	"time"

	"github.com/coffeenights/conure/api/oam/v1alpha1"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
)

type AppStatus string

const (
	AppReady    AppStatus = "Ready"
	AppNotReady AppStatus = "NotReady"
)

type ApplicationResponse struct {
	ResourceID      string                     `json:"resource_id"`
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	EnvironmentId   string                     `json:"environment_id"`
	AccountId       uint64                     `json:"account_id"`
	TotalComponents int                        `json:"total_components"`
	Components      []ServiceComponentResponse `json:"components"`
	Status          AppStatus                  `json:"status"`
	Created         time.Time                  `json:"created"`
}

func (r *ApplicationResponse) FromClientsetToResponse(item *v1alpha1.Application) {
	r.ResourceID = string(item.ObjectMeta.UID)
	r.Name = item.ObjectMeta.Name
	r.Description = item.ObjectMeta.Namespace
	r.EnvironmentId = ""
	r.AccountId = 0
	r.Created = item.ObjectMeta.CreationTimestamp.UTC()
}

type ServiceComponentResponse struct {
	Name           string    `json:"name"`
	Replicas       int32     `json:"replicas"`
	ContainerImage string    `json:"container_image"`
	ContainerPort  int32     `json:"container_port"`
	Status         AppStatus `json:"status"`
	Updated        time.Time `json:"updated"`
}

func (r *ServiceComponentResponse) FromClientsetToResponse(deployment appsV1.Deployment, services []coreV1.Service) {
	r.Name = deployment.ObjectMeta.Name
	r.Replicas = *deployment.Spec.Replicas
	r.ContainerImage = deployment.Spec.Template.Spec.Containers[0].Image
	r.Updated = deployment.CreationTimestamp.UTC()

	status := deployment.Status
	if status.Replicas != status.ReadyReplicas {
		r.Status = AppNotReady
	} else {
		r.Status = AppReady
	}

	// Extracting all ports from the service associated to the deployment
	r.ContainerPort = 0
	if len(services) > 0 {
		if len(services[0].Spec.Ports) > 0 {
			r.ContainerPort = services[0].Spec.Ports[0].Port
		}
	}
}
