package api

import "time"

// ----- Organizations -------------------------------------------------------

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	AccountID string    `json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

type OrganizationListResponse struct {
	Organizations []Organization `json:"organizations"`
}

type OrganizationResponse struct {
	Organization
}

type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required"`
}

// ----- Applications --------------------------------------------------------

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Application struct {
	ID              string        `json:"id"`
	OrganizationID  string        `json:"organization_id"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	TotalComponents int64         `json:"total_components,omitempty"`
	CreatedBy       string        `json:"created_by,omitempty"`
	AccountID       string        `json:"account_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	Environments    []Environment `json:"environments,omitempty"`
}

type ApplicationListResponse struct {
	Organization Organization  `json:"organization"`
	Applications []Application `json:"applications"`
}

type CreateApplicationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CreateEnvironmentRequest struct {
	Name string `json:"name" binding:"required"`
}

// ----- Component identity (app-wide) --------------------------------------

type EnvironmentPresence struct {
	EnvironmentID         string `json:"environment_id"`
	EnvironmentName       string `json:"environment_name"`
	Active                bool   `json:"active"`
	HasDraft              bool   `json:"has_draft"`
	LatestDraftVersion    int    `json:"latest_draft_version,omitempty"`
	LatestDeployedVersion int    `json:"latest_deployed_version,omitempty"`
	Drifted               bool   `json:"drifted"`
}

type Component struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	Description   string                `json:"description,omitempty"`
	ApplicationID string                `json:"application_id"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Environments  []EnvironmentPresence `json:"environments,omitempty"`
}

type ComponentListResponse struct {
	Components []Component `json:"components"`
}

type CreateComponentRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Type        string                 `json:"type" binding:"required"`
	Description string                 `json:"description,omitempty"`
	Environment string                 `json:"environment" binding:"required"`
	Values      map[string]interface{} `json:"values,omitempty"`
}

type CreateComponentResponse struct {
	Component Component         `json:"component"`
	Revision  ComponentRevision `json:"revision"`
}

// ----- Component revisions -------------------------------------------------

type ComponentRevision struct {
	ID            string                 `json:"id"`
	ComponentID   string                 `json:"component_id"`
	EnvironmentID string                 `json:"environment_id"`
	Version       int                    `json:"version"`
	Values        map[string]interface{} `json:"values"`
	Status        string                 `json:"status"` // draft | deployed
	DeployedAt    *time.Time             `json:"deployed_at,omitempty"`
	CreatedBy     string                 `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

type ComponentRevisionListResponse struct {
	Revisions []ComponentRevision `json:"revisions"`
}

type CreateRevisionRequest struct {
	Values map[string]interface{} `json:"values"`
}

type UpdateRevisionRequest struct {
	Values map[string]interface{} `json:"values"`
}

// ----- Env-scoped component view ------------------------------------------

type DriftEntry struct {
	Path     string      `json:"path"`
	Live     interface{} `json:"live"`
	Deployed interface{} `json:"deployed"`
}

type ComponentInEnvResponse struct {
	ComponentID      string                 `json:"component_id"`
	Name             string                 `json:"name"`
	EnvironmentID    string                 `json:"environment_id"`
	EnvironmentName  string                 `json:"environment_name"`
	LiveValues       map[string]interface{} `json:"live_values,omitempty"`
	DeployedRevision *ComponentRevision     `json:"deployed_revision,omitempty"`
	LatestDraft      *ComponentRevision     `json:"latest_draft,omitempty"`
	Drifted          bool                   `json:"drifted"`
	Diff             []DriftEntry           `json:"diff,omitempty"`
	HealthCondition  string                 `json:"health_condition,omitempty"`
	HealthStatus     string                 `json:"health_status,omitempty"`
	HealthMessage    string                 `json:"health_message,omitempty"`
}

type ComponentInEnvListResponse struct {
	Components []ComponentInEnvResponse `json:"components"`
}

// ----- Promote / bulk deploy ----------------------------------------------

type PromoteRequest struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}

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

// ----- Component definitions (templates) ----------------------------------

type ComponentDefinition struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	OCIRepository string  `json:"oci_repository"`
	OCITag        string  `json:"oci_tag"`
	IconURL       *string `json:"icon_url"`
}

type ComponentDefinitionListResponse struct {
	Definitions []ComponentDefinition `json:"definitions"`
}
