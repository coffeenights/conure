package api

import "time"

// Organization

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
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

// Application

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ApplicationRevision struct {
	RevisionNumber int       `json:"revision_number"`
	CreatedAt      time.Time `json:"created_at"`
}

type Application struct {
	ID              string                `json:"id"`
	OrganizationID  string                `json:"organization_id"`
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	TotalComponents int64                 `json:"total_components"`
	Revisions       []ApplicationRevision `json:"revisions,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	Environments    []Environment         `json:"environments,omitempty"`
}

type ApplicationListResponse struct {
	Organization Organization  `json:"organization"`
	Applications []Application `json:"applications"`
}

type ApplicationResponse struct {
	Application
}

type ApplicationStatusResponse struct {
	Status string `json:"status"`
}

type CreateApplicationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CreateEnvironmentRequest struct {
	Name string `json:"name" binding:"required"`
}

// Component

type ComponentSettings struct {
	Resources ResourcesSettings `json:"resources_settings"`
	Source    SourceSettings    `json:"source_settings"`
	Network   NetworkSettings   `json:"network_settings"`
	Storage   []StorageSettings `json:"storage_settings"`
}

type ResourcesSettings struct {
	Replicas int    `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

type SourceSettings struct {
	Repository string `json:"repository"`
	Command    string `json:"command"`
}

type NetworkSettings struct {
	Exposed bool           `json:"exposed"`
	Type    string         `json:"type"`
	Ports   []PortSettings `json:"ports"`
}

type PortSettings struct {
	HostPort   int    `json:"host_port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`
}

type StorageSettings struct {
	Size      float32 `json:"size"`
	Name      string  `json:"name"`
	MountPath string  `json:"mount_path"`
}

type Component struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Description   string            `json:"description"`
	ApplicationID string            `json:"application_id"`
	Settings      ComponentSettings `json:"settings"`
	CreatedAt     time.Time         `json:"created_at"`
}

type ComponentListResponse struct {
	Components []Component `json:"components"`
}

type ComponentResponse struct {
	Component
}

type CreateComponentRequest struct {
	Name        string            `json:"name" binding:"required"`
	Type        string            `json:"type" binding:"required"`
	Description string            `json:"description"`
	Settings    ComponentSettings `json:"settings"`
}

// Component Status

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

type VolumeProperties struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size string `json:"size"`
}

type StorageProperties struct {
	Volumes []VolumeProperties `json:"volumes"`
	Healthy bool               `json:"health"`
}

type SourceProperties struct {
	ContainerImage string `json:"container_image"`
	Command        string `json:"command"`
}

type ComponentStatusHealth struct {
	Healthy bool      `json:"healthy"`
	Message string    `json:"message"`
	Updated time.Time `json:"updated"`
}

type ComponentProperties struct {
	Network   *NetworkProperties     `json:"network"`
	Resources *ResourcesProperties   `json:"resources"`
	Storage   *StorageProperties     `json:"storage"`
	Source    *SourceProperties      `json:"source"`
	Health    *ComponentStatusHealth `json:"health"`
}

type ComponentStatusResponse struct {
	Component  Component           `json:"component"`
	Properties ComponentProperties `json:"properties"`
}

// Pods

type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Pod struct {
	Name       string         `json:"name"`
	Phase      string         `json:"phase"`
	Conditions []PodCondition `json:"conditions"`
}

type ComponentPodsResponse struct {
	Pods []Pod `json:"pods"`
}

// Service Component Status

type ServiceComponentStatusResponse struct {
	UpdatedReplicas      int32     `json:"updated_replicas"`
	ReadyReplicas        int32     `json:"ready_replicas"`
	AvailableReplicas    int32     `json:"available_replicas"`
	ConditionAvailable   string    `json:"condition_available"`
	ConditionProgressing string    `json:"condition_progressing"`
	Created              time.Time `json:"created"`
	Updated              time.Time `json:"updated"`
}
