package applications

import (
	"time"

	"github.com/coffeenights/conure/api/oam/v1alpha1"
	"k8s.io/api/apps/v1"
)

type ApplicationResponse struct {
	ResourceID    string                     `json:"resource_id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	EnvironmentId string                     `json:"environment_id"`
	AccountId     uint64                     `json:"account_id"`
	Components    []ServiceComponentResponse `json:"components"`
	Created       time.Time                  `json:"created"`
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
	Name           string `json:"name"`
	Replicas       int32  `json:"replicas"`
	ContainerImage string `json:"container_image"`
	ContainerPort  int32  `json:"container_port"`
}

func (r *ServiceComponentResponse) FromClientsetToResponse(item *v1.Deployment) {
	r.Name = item.ObjectMeta.Name
	r.Replicas = *item.Spec.Replicas
	r.ContainerImage = item.Spec.Template.Spec.Containers[0].Image
	r.ContainerPort = 0
}
