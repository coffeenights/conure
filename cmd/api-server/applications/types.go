package applications

import (
	"time"

	"github.com/coffeenights/conure/api/oam/v1alpha1"
)

type ApplicationResponse struct {
	ResourceID    string        `json:"resource_id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	EnvironmentId string        `json:"environment_id"`
	AccountId     uint64        `json:"account_id"`
	Components    []interface{} `json:"components"`
	Created       time.Time     `json:"created"`
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
