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
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
	// Engine pins the rendering backend (timoni or helm). Required when
	// more than one ComponentDefinition shares the same type; otherwise the
	// API resolves it from the single matching definition.
	Engine      string                 `json:"engine,omitempty"`
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
	Comment       string                 `json:"comment,omitempty"`
	DeployedAt    *time.Time             `json:"deployed_at,omitempty"`
	CreatedBy     string                 `json:"created_by,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

type ComponentRevisionListResponse struct {
	Revisions []ComponentRevision `json:"revisions"`
}

type CreateRevisionRequest struct {
	Values  map[string]interface{} `json:"values"`
	Comment string                 `json:"comment,omitempty"`
}

type UpdateRevisionRequest struct {
	Values  map[string]interface{} `json:"values"`
	Comment string                 `json:"comment,omitempty"`
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

// ----- Pods & logs --------------------------------------------------------

type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type Pod struct {
	Name       string         `json:"name"`
	Phase      string         `json:"phase"`
	Ready      bool           `json:"ready"`
	Restarts   int32          `json:"restarts"`
	Containers []string       `json:"containers,omitempty"`
	Conditions []PodCondition `json:"conditions,omitempty"`
}

type ComponentPodsResponse struct {
	Pods []Pod `json:"pods"`
}

// ----- Component definitions (templates) ----------------------------------

type ComponentDefinition struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Engine        string  `json:"engine,omitempty"`
	Description   string  `json:"description"`
	OCIRepository string  `json:"oci_repository"`
	OCITag        string  `json:"oci_tag"`
	IconURL       *string `json:"icon_url"`
}

type ComponentDefinitionListResponse struct {
	Definitions []ComponentDefinition `json:"definitions"`
}

// ----- Variables (and secrets) -------------------------------------------
//
// The server stores all variables in one collection, distinguished by Type
// ("organization" | "environment" | "component") and the path-scoped IDs.
// IsEncrypted=true is what the UI/CLI surfaces as a "secret"; the value is
// AES-encrypted at rest by the API server but returned decrypted on read.

type Variable struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Value          string    `json:"value"`
	Type           string    `json:"type"`
	OrganizationID string    `json:"organization_id,omitempty"`
	ApplicationID  string    `json:"application_id,omitempty"`
	EnvironmentID  string    `json:"environment_id,omitempty"`
	ComponentID    string    `json:"component_id,omitempty"`
	IsEncrypted    bool      `json:"is_encrypted"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateVariableRequest struct {
	Name        string `json:"name" binding:"required"`
	Value       string `json:"value" binding:"required"`
	IsEncrypted bool   `json:"is_encrypted"`
}
