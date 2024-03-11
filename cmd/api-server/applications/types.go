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

type Application struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organization_id"`
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

func NewApplication(organizationID string, applicationID string, environment string) *Application {
	return &Application{
		OrganizationID: organizationID,
		ID:             applicationID,
		Environment:    environment,
	}
}

func (a *Application) getNamespace() string {
	return a.OrganizationID + "-" + a.ID + "-" + a.Environment
}

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
	LastUpdated     time.Time `json:"last_updated"`
}

func (r *ApplicationResponse) FromVelaClientsetToResponse(item *v1beta1.Application, revision *v1beta1.ApplicationRevision) {
	r.Name = item.ObjectMeta.Name
	r.ID = item.ObjectMeta.Labels["conure.io/application-id"]
	r.Description = item.ObjectMeta.Annotations["conure.io/description"]
	r.Environment = item.ObjectMeta.Labels["conure.io/environment"]
	r.CreatedBy = item.ObjectMeta.Labels["conure.io/created-by"]
	r.AccountID = item.ObjectMeta.Labels["conure.io/account-id"]
	r.Created = item.ObjectMeta.CreationTimestamp.UTC()
	r.Status = AppStatus(item.Status.Phase)
	r.Revision = item.Status.LatestRevision.Revision
	r.LastUpdated = revision.CreationTimestamp.UTC()
	r.TotalComponents = len(item.Spec.Components)
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

type NetworkProperties struct {
	IP         string  `json:"ip"`
	ExternalIP string  `json:"external_ip"`
	Host       string  `json:"host"`
	Ports      []int32 `json:"port"`
}

type ResourcesProperties struct {
	Replicas int32  `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

type StorageProperties struct {
	Size string `json:"size"`
}

type SourceProperties struct {
	ContainerImage string `json:"container_image"`
}

type ComponentProperties struct {
	Name                string               `json:"name"`
	Type                string               `json:"type"`
	Description         string               `json:"description"`
	NetworkProperties   *NetworkProperties   `json:"network"`
	ResourcesProperties *ResourcesProperties `json:"resources"`
	StorageProperties   *StorageProperties   `json:"storage"`
	SourceProperties    *SourceProperties    `json:"source"`
}
