package applications

import (
	"encoding/json"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	"k8s.io/apimachinery/pkg/runtime"
	"time"

	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/v1beta1"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
)

type AppStatus string

const (
	AppReady    AppStatus = "Ready"
	AppNotReady AppStatus = "NotReady"
)

type ApplicationResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Environment     string    `json:"environment"`
	CreatedBy       string    `json:"created_by"`
	AccountID       string    `json:"account_id"`
	TotalComponents int       `json:"total_components"`
	Status          AppStatus `json:"status"`
	Created         time.Time `json:"created"`
	Revision        int64     `json:"revision"`
}

func (r *ApplicationResponse) FromVelaClientsetToResponse(item *v1beta1.Application) {
	r.Name = item.ObjectMeta.Name
	r.ID = item.ObjectMeta.Labels["conure.io/application-id"]
	r.Description = item.ObjectMeta.Annotations["conure.io/description"]
	r.Environment = item.ObjectMeta.Labels["conure.io/environment"]
	r.CreatedBy = item.ObjectMeta.Labels["conure.io/created-by"]
	r.AccountID = item.ObjectMeta.Labels["conure.io/account-id"]
	r.Created = item.ObjectMeta.CreationTimestamp.UTC()
	r.Status = AppStatus(item.Status.Phase)
	r.Revision = item.Status.LatestRevision.Revision
	r.TotalComponents = len(item.Spec.Components)
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

func (r *ServiceComponentResponse) FromVelaClientsetToResponse(deployment common.ApplicationComponent) {
	r.Name = deployment.Name
	propertiesData, err := extractMapFromRawExtension(deployment.Properties)
	if err != nil {
		panic(err)
	}
	r.ContainerImage = propertiesData["image"].(string)
	// check if the port is defined in the properties or its on the containerPort
	if propertiesData["port"] != nil {
		r.ContainerPort = int32(propertiesData["port"].(float64))
	} else {
		// go through the containers to find the port
		for _, container := range propertiesData["image"].([]map[string]interface{}) {
			if container != nil {
				r.ContainerPort = 1
			}
		}
	}

	// go through the traits to find the replicas and the ports
	for _, trait := range deployment.Traits {
		traitsData, err := extractMapFromRawExtension(trait.Properties)
		if err != nil {
			panic(err)
		}
		if trait.Type == "scaler" {
			//{"replicas":2}
			r.Replicas = int32(traitsData["replicas"].(float64))
		}
		if trait.Type == "expose" {
			//{"annotations":{"service":"backend"},"port":[8090],"type":"ClusterIP"}
			r.ContainerPort = int32(traitsData["port"].([]interface{})[0].(float64))
		}
	}
}

type ServiceComponentShortResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (r *ServiceComponentShortResponse) FromClientsetToResponse(component common.ApplicationComponent) {
	r.Name = component.Name
	r.Type = component.Type
}

type ServiceComponentListResponse struct {
	Application ApplicationResponse             `json:"application"`
	Components  []ServiceComponentShortResponse `json:"components"`
}

func extractMapFromRawExtension(data *runtime.RawExtension) (map[string]interface{}, error) {
	var result map[string]interface{}
	bytesData, err := data.MarshalJSON()
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytesData, &result)
	if err != nil {
		panic(err)
	}
	return result, err
}

type CreateOrganizationRequest struct {
	Name      string `json:"name" validate:"required"`
	AccountID string `json:"account_id" validate:"required"`
}

func (r *CreateOrganizationRequest) ParseRequestToModel() *Organization {
	return &Organization{
		Name:      r.Name,
		AccountID: r.AccountID,
	}
}

type OrganizationResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	AccountID string    `json:"account_id"`
}

func (r *OrganizationResponse) ParseModelToResponse(organization *Organization) {
	r.ID = organization.ID.Hex()
	r.Name = organization.Name
	r.Status = string(organization.Status)
	r.CreatedAt = organization.CreatedAt
	r.AccountID = organization.AccountID
}

type CreateEnvironmentRequest struct {
	Name           string `json:"name" validate:"required,regexp=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"`
	ApplicationID  string `json:"application_id" validate:"required"`
	OrganizationID string `json:"organization_id" validate:"required"`
}

type EnvironmentListResponse struct {
	Environments []EnvironmentResponse `json:"environments"`
}

type EnvironmentResponse struct {
	Name string `json:"name"`
}
