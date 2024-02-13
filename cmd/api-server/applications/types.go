package applications

import (
	"encoding/json"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	"time"

	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/v1beta1"
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
	Name           string `json:"name"`
	Replicas       int32  `json:"replicas"`
	ContainerImage string `json:"container_image"`
	ContainerPort  int32  `json:"container_port"`
	Status         string `json:"status"`
	CPU            string `json:"cpu"`
	Memory         string `json:"memory"`
}

func (r *ServiceComponentResponse) FromClientsetToResponse(component common.ApplicationComponent, status common.ApplicationComponentStatus) {
	r.Name = component.Name
	propertiesData, err := extractMapFromRawExtension(component.Properties)
	if err != nil {
		log.Fatal(err)
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
	for _, trait := range component.Traits {
		traitsData, err := extractMapFromRawExtension(trait.Properties)
		if err != nil {
			log.Fatal(err)
		}
		if trait.Type == "scaler" {
			r.Replicas = int32(traitsData["replicas"].(float64))
		}
		if trait.Type == "expose" {
			r.ContainerPort = int32(traitsData["port"].([]interface{})[0].(float64))
		}
	}

	r.CPU = propertiesData["cpu"].(string)
	r.Memory = propertiesData["memory"].(string)
	r.Status = status.Message
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
