package applications

import (
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	k8sV1 "k8s.io/api/apps/v1"
	"time"
)

type ApplicationResponse struct {
	*Application
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

type ComponentResponse struct {
	*Component
}

type ComponentListResponse struct {
	Components []ComponentResponse `json:"components"`
}

type CreateComponentRequest struct {
	ID          string                 `json:"id" validate:"required"`
	Type        string                 `json:"type" validate:"required"`
	Properties  map[string]interface{} `json:"properties" validate:"required"`
	Description string                 `json:"description"`
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
	Name string `json:"name" validate:"required,regexp=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"`
}

type EnvironmentListResponse struct {
	Environments []EnvironmentResponse `json:"environments"`
}

type EnvironmentResponse struct {
	*Environment
}
