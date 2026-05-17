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
	ComponentID string `json:"component_id"`
	Name        string `json:"name"`
	// Type/Engine identify the component's ComponentDefinition (join key:
	// type + optional engine). The CLI needs them here to resolve the
	// definition's fieldRoles from this single call.
	Type             string                 `json:"type"`
	Engine           string                 `json:"engine,omitempty"`
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
	// Buildable reports whether conure can build the component's image.
	// When false, `conure deploy` is promote-only for this type.
	Buildable bool `json:"buildable"`
	// FieldRoles maps conure's well-known field roles (sourceType,
	// image.repository, git.repository, …) to dotted paths into a
	// component's values. The CLI and server resolve image/build fields
	// through this instead of hardcoding the schema; there is no fallback.
	FieldRoles map[string]string `json:"field_roles,omitempty"`
	// CredentialRef is the logical name of the org registry credential used
	// to pull this definition's private OCI module (a name, not material).
	CredentialRef string `json:"credential_ref,omitempty"`
	// Source is "default" for a row inherited from the platform's shipped
	// defaults and "organization" for one this org created or overrode. The
	// admin CLI surfaces it so operators can tell inherited from customized.
	Source string `json:"source,omitempty"`
}

type ComponentDefinitionListResponse struct {
	Definitions []ComponentDefinition `json:"definitions"`
}

// ComponentDefinitionRequest is the create/override body for an org-scoped
// component definition (POST .../component-definitions). Type is required and,
// with Engine (optional, empty == timoni), is the lookup key. Posting for a
// (type, engine) the org already has updates it in place (and un-hides a
// tombstone); otherwise it creates a new org-owned override.
//
// Every non-key field is a pointer so the server can distinguish "omitted"
// from "set to zero": a nil field is left at whatever the org's resolved
// definition already has (inherited default or existing override), so a
// one-flag edit patches instead of wiping field_roles/buildable. The
// --from-file path fills every field, which naturally yields a full replace.
type ComponentDefinitionRequest struct {
	Type   string `json:"type"`
	Engine string `json:"engine,omitempty"`

	Description   *string            `json:"description,omitempty"`
	OCIRepository *string            `json:"oci_repository,omitempty"`
	OCITag        *string            `json:"oci_tag,omitempty"`
	OCIDigest     *string            `json:"oci_digest,omitempty"`
	OCIRegistry   *string            `json:"oci_registry,omitempty"`
	CredentialRef *string            `json:"credential_ref,omitempty"`
	Buildable     *bool              `json:"buildable,omitempty"`
	FieldRoles    *map[string]string `json:"field_roles,omitempty"`
	IconURL       *string            `json:"icon_url,omitempty"`
}

// HideComponentDefinitionRequest is the tombstone body (POST
// .../component-definitions/hide). It suppresses the inherited default for a
// (type, engine) within the org; reversible by deleting the resulting row.
type HideComponentDefinitionRequest struct {
	Type   string `json:"type"`
	Engine string `json:"engine,omitempty"`
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

// ----- Credentials --------------------------------------------------------
//
// Credentials are the org-scoped, AES-encrypted source of truth for registry
// and git auth. The server stores ciphertext; a deploy projects the resolved
// credential into a Kubernetes Secret. The secret material is write-only over
// the API: it is accepted on create but NEVER returned, so Credential (the
// read shape) has no secret field.

// Credential is the metadata-only read view. There is intentionally no field
// for the password/token: list/get can never leak material.
type Credential struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	RegistryURL    string    `json:"registry_url,omitempty"`
	Username       string    `json:"username,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateCredentialRequest is the write shape. Kind is "registry" or "git".
// Secret is the raw password/token; the CLI reads it from stdin so it never
// enters shell history. Posting an existing Name rotates it in place.
type CreateCredentialRequest struct {
	Name        string `json:"name" binding:"required"`
	Kind        string `json:"kind" binding:"required"`
	RegistryURL string `json:"registry_url,omitempty"`
	Username    string `json:"username,omitempty"`
	Secret      string `json:"secret" binding:"required"`
}

// ----- System info --------------------------------------------------------

// SystemInfo carries cluster-level metadata the CLI needs to decide how to
// build (cross-arch or native) and which target platform to push.
type SystemInfo struct {
	// Platform is "<os>/<arch>", e.g. "linux/amd64" — the dominant node
	// platform in the target cluster.
	Platform string `json:"platform"`
	// KubernetesVersion is the cluster's server GitVersion. Advisory.
	KubernetesVersion string `json:"kubernetes_version"`
}

// ----- Builds --------------------------------------------------------------

// BuildStatus matches the server enum: "pending" | "building" |
// "succeeded" | "failed".
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusBuilding  BuildStatus = "building"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
)

// TriggerBuildRequest is the body of POST /builds.
//
// Two flows on the server side:
//
//   - build_location == "local": the CLI has already built and pushed
//     image_ref. The server records the build as succeeded and rolls the
//     deploy forward synchronously.
//   - build_location == "remote": the server creates a BuildKit Job in
//     conure-system, clones git_repository@git_branch, builds with the
//     chosen frontend (dockerfile or railpack), pushes to image_ref, and
//     deploys when the Job succeeds. Railpack is rejected for remote builds.
type TriggerBuildRequest struct {
	BuildTool     string `json:"build_tool" binding:"required"`     // "railpack" | "dockerfile"
	BuildLocation string `json:"build_location" binding:"required"` // "local" | "remote"
	Platform      string `json:"platform,omitempty"`                // "linux/amd64" etc.
	GitRepository string `json:"git_repository,omitempty"`
	GitBranch     string `json:"git_branch,omitempty"`
	ImageRef      string `json:"image_ref" binding:"required"`
}

// Build is the wire shape returned by the build endpoints.
type Build struct {
	ID            string      `json:"id"`
	ComponentID   string      `json:"component_id"`
	ApplicationID string      `json:"application_id"`
	EnvironmentID string      `json:"environment_id"`
	Status        BuildStatus `json:"status"`
	BuildTool     string      `json:"build_tool"`
	BuildLocation string      `json:"build_location"`
	Platform      string      `json:"platform,omitempty"`
	GitRepository string      `json:"git_repository,omitempty"`
	GitBranch     string      `json:"git_branch,omitempty"`
	ImageRef      string      `json:"image_ref,omitempty"`
	JobName       string      `json:"job_name,omitempty"`
	JobNamespace  string      `json:"job_namespace,omitempty"`
	ErrorMessage  string      `json:"error_message,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	FinishedAt    *time.Time  `json:"finished_at,omitempty"`
}

type BuildListResponse struct {
	Builds []Build `json:"builds"`
}

// ----- Users (admin) and self-service account -----------------------------

type User struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	OrganizationID string     `json:"organization_id,omitempty"`
	IsActive       bool       `json:"is_active"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type UserListResponse struct {
	Users []User `json:"users"`
}

type CreateUserRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	Role           string `json:"role,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
}

type UpdateUserRequest struct {
	Email          *string `json:"email,omitempty"`
	Role           *string `json:"role,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

type ResetPasswordRequest struct {
	Password string `json:"password,omitempty"`
}

type ResetPasswordResponse struct {
	Password string `json:"password"`
}

type UpdateMeRequest struct {
	Email *string `json:"email,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	Password    string `json:"password"`
	Password2   string `json:"password2"`
}
