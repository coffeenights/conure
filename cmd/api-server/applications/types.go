package applications

import (
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// ApplicationStatus Indicate the current condition of the overall application
type ApplicationStatus string

const (
	ApplicationStarting           ApplicationStatus = "starting"
	ApplicationRendering          ApplicationStatus = "rendering"
	ApplicationPolicyGenerating   ApplicationStatus = "generatingPolicy"
	ApplicationRunningWorkflow    ApplicationStatus = "runningWorkflow"
	ApplicationWorkflowSuspending ApplicationStatus = "workflowSuspending"
	ApplicationWorkflowTerminated ApplicationStatus = "workflowTerminated"
	ApplicationWorkflowFailed     ApplicationStatus = "workflowFailed"
	ApplicationRunning            ApplicationStatus = "running"
	ApplicationUnhealthy          ApplicationStatus = "unhealthy"
	ApplicationDeleting           ApplicationStatus = "deleting"
)

type ApplicationResponse struct {
	*models.Application
	TotalComponents int64 `json:"total_components"`
}

type ApplicationListResponse struct {
	Organization OrganizationResponse  `json:"organization"`
	Applications []ApplicationResponse `json:"applications"`
}

type CreateApplicationRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

// EnvironmentPresence summarizes whether a component is active in a given
// environment, what its last-deployed revision is, whether a draft is
// outstanding, and whether live state has drifted from the last deploy.
type EnvironmentPresence struct {
	EnvironmentID         string `json:"environment_id"`
	EnvironmentName       string `json:"environment_name"`
	Active                bool   `json:"active"`
	HasDraft              bool   `json:"has_draft"`
	LatestDraftVersion    int    `json:"latest_draft_version,omitempty"`
	LatestDeployedVersion int    `json:"latest_deployed_version,omitempty"`
	Drifted               bool   `json:"drifted"`
}

// ComponentResponse is the app-wide identity view, with optional per-env
// presence rollup.
type ComponentResponse struct {
	*models.Component
	Environments []EnvironmentPresence `json:"environments,omitempty"`
}

type ComponentListResponse struct {
	Components []ComponentResponse `json:"components"`
}

// ComponentRevisionResponse is the JSON shape returned for any single revision.
type ComponentRevisionResponse struct {
	*models.ComponentRevision
}

type ComponentRevisionListResponse struct {
	Revisions []models.ComponentRevision `json:"revisions"`
}

// ComponentInEnvResponse is the env-scoped detail view: live K8s values, the
// last-deployed revision snapshot, and a structured drift diff.
type ComponentInEnvResponse struct {
	ComponentID      string                    `json:"component_id"`
	Name             string                    `json:"name"`
	EnvironmentID    string                    `json:"environment_id"`
	EnvironmentName  string                    `json:"environment_name"`
	LiveValues       map[string]interface{}    `json:"live_values,omitempty"`
	DeployedRevision *models.ComponentRevision `json:"deployed_revision,omitempty"`
	LatestDraft      *models.ComponentRevision `json:"latest_draft,omitempty"`
	Drifted          bool                      `json:"drifted"`
	Diff             []DriftEntry              `json:"diff,omitempty"`
	HealthCondition  string                    `json:"health_condition,omitempty"`
	HealthStatus     string                    `json:"health_status,omitempty"`
	HealthMessage    string                    `json:"health_message,omitempty"`
}

// ComponentInEnvListResponse is the env-scoped list shape returned by
// `GET /e/:env/c`. Each entry mirrors what the detail call returns.
type ComponentInEnvListResponse struct {
	Components []ComponentInEnvResponse `json:"components"`
}

type CreateComponentRequest struct {
	Name        string `json:"name" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Description string `json:"description"`
	// Environment is the target env for the first draft revision created
	// alongside the new component identity.
	Environment string                 `json:"environment" binding:"required"`
	Values      map[string]interface{} `json:"values"`
}

type CreateRevisionRequest struct {
	Values  map[string]interface{} `json:"values"`
	Comment string                 `json:"comment,omitempty"`
}

type UpdateRevisionRequest struct {
	Values  map[string]interface{} `json:"values"`
	Comment string                 `json:"comment,omitempty"`
}

type PromoteRequest struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}

// DeployBatchEntry reports the per-component outcome of a bulk deploy.
type DeployBatchEntry struct {
	ComponentID   string `json:"component_id"`
	ComponentName string `json:"component_name"`
	RevisionID    string `json:"revision_id,omitempty"`
	Version       int    `json:"version,omitempty"`
	Error         string `json:"error,omitempty"`
}

type DeployBatchResponse struct {
	Deployed []DeployBatchEntry `json:"deployed"`
	Failed   []DeployBatchEntry `json:"failed"`
}

type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required"`
}

func (r *CreateOrganizationRequest) ParseRequestToModel() *models.Organization {
	return &models.Organization{
		Name: r.Name,
	}
}

type OrganizationResponse struct {
	*models.Organization
}

type OrganizationListResponse struct {
	Organizations []OrganizationResponse `json:"organizations"`
}

type CreateEnvironmentRequest struct {
	Name string `json:"name" validate:"required,regexp=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"` // TODO: Validate this field with a regex, current implementation doesn't work
}

type EnvironmentListResponse struct {
	Environments []EnvironmentResponse `json:"environments"`
}

type EnvironmentResponse struct {
	*models.Environment
}

// PodConditionResponse mirrors the subset of corev1.PodCondition that's
// useful to a UI/CLI consumer.
type PodConditionResponse struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// PodResponse is the shape returned by ListComponentPods. Phase/Ready/Restarts
// are pre-computed kubectl-style so the CLI doesn't have to roll them up.
type PodResponse struct {
	Name       string                 `json:"name"`
	Phase      string                 `json:"phase"`
	Ready      bool                   `json:"ready"`
	Restarts   int32                  `json:"restarts"`
	Containers []string               `json:"containers,omitempty"`
	Conditions []PodConditionResponse `json:"conditions,omitempty"`
}

type ComponentPodsResponse struct {
	Pods []PodResponse `json:"pods"`
}
