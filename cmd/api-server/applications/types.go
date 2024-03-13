package applications

import (
	"encoding/json"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	k8sV1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"time"

	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/v1beta1"
)

type AppStatus string

const (
	AppReady    AppStatus = "Ready"
	AppNotReady AppStatus = "NotReady"
)

const (
	ApplicationIDLabel  = "conure.io/application-id"
	OrganizationIDLabel = "conure.io/organization-id"
	EnvironmentLabel    = "conure.io/environment"
	CreatedByLabel      = "conure.io/created-by"
	NamespaceLabel      = "conure.io/namespace"
)

type ApplicationResponse struct {
	*Application
}

type ApplicationListResponse struct {
	Organization OrganizationResponse  `json:"organization"`
	Applications []ApplicationResponse `json:"applications"`
}

type ApplicationResponseOld struct {
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
	LastUpdated     time.Time `json:"last_updated"`
}

func (r *ApplicationResponseOld) FromVelaClientsetToResponse(item *v1beta1.Application, revision *v1beta1.ApplicationRevision) {
	r.Name = item.ObjectMeta.Name
	r.Description = item.ObjectMeta.Annotations["conure.io/description"]
	r.CreatedBy = item.ObjectMeta.Labels["conure.io/account-id"]
	r.AccountID = item.ObjectMeta.Labels["conure.io/account-id"]
}

type ApplicationDetailsResponse struct {
	Application ApplicationResponse `json:"application"`
}

type ServiceComponentResponse struct {
	ComponentProperties
}

type ServiceComponentStatusResponse struct {
	UpdatedReplicas      int32     `json:"updated_replicas"`
	ReadyReplicas        int32     `json:"ready_replicas"`
	AvailableReplicas    int32     `json:"available_replicas"`
	ConditionAvailable   string    `json:"condition_available"`
	ConditionProgressing string    `json:"condition_progressing"`
	Created              time.Time `json:"created"`
	Updated              time.Time `json:"updated"`
}

func (r *ServiceComponentStatusResponse) FromClientsetToResponse(deployment k8sV1.Deployment) {
	r.UpdatedReplicas = deployment.Status.UpdatedReplicas
	r.ReadyReplicas = deployment.Status.ReadyReplicas
	r.AvailableReplicas = deployment.Status.AvailableReplicas
	r.Created = deployment.ObjectMeta.CreationTimestamp.UTC()
	r.Updated = deployment.ObjectMeta.CreationTimestamp.UTC()

	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Available" {
			r.ConditionAvailable = string(condition.Status)
		} else if condition.Type == "Progressing" {
			r.ConditionProgressing = string(condition.Status)
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
	Components []ServiceComponentShortResponse `json:"components"`
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
