package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	k8sV1 "k8s.io/api/apps/v1"
	"time"
)

type ApplicationResponse struct {
	*models.Application
}

type ApplicationListResponse struct {
	Organization OrganizationResponse  `json:"organization"`
	Applications []ApplicationResponse `json:"applications"`
}

type CreateApplicationRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
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

type ComponentResponse struct {
	*models.Component
}

type ComponentListResponse struct {
	Components []ComponentResponse `json:"components"`
}

type ComponentProperties struct {
	NetworkProperties   *providers.NetworkProperties   `json:"network"`
	ResourcesProperties *providers.ResourcesProperties `json:"resources"`
	StorageProperties   *providers.StorageProperties   `json:"storage"`
	SourceProperties    *providers.SourceProperties    `json:"source"`
}

type ComponentStatusResponse struct {
	Component  ComponentResponse   `json:"component"`
	Properties ComponentProperties `json:"properties"`
}

type CreateComponentRequest struct {
	ID          string                   `json:"id" validate:"required"`
	Type        string                   `json:"type" validate:"required"`
	Properties  map[string]interface{}   `json:"properties" validate:"required"`
	Traits      []map[string]interface{} `json:"traits"`
	Description string                   `json:"description"`
}

type CreateOrganizationRequest struct {
	Name      string `json:"name" validate:"required"`
	AccountID string `json:"account_id" validate:"required"`
}

func (r *CreateOrganizationRequest) ParseRequestToModel() *models.Organization {
	return &models.Organization{
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

func (r *OrganizationResponse) ParseModelToResponse(organization *models.Organization) {
	r.ID = organization.ID.Hex()
	r.Name = organization.Name
	r.Status = string(organization.Status)
	r.CreatedAt = organization.CreatedAt
	r.AccountID = organization.AccountID
}

type CreateEnvironmentRequest struct {
	Name string `json:"name" validate:"required,regexp=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"`
}

type EnvironmentListResponse struct {
	Environments []EnvironmentResponse `json:"environments"`
}

type EnvironmentResponse struct {
	*models.Environment
}
