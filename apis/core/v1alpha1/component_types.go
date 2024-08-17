package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// Component A component is a part of an application and represents a single unit of deployment.
type Component struct {
	Name          string `json:"name"`
	ComponentType string `json:"type"`
	OCIRepository string `json:"ociRepository"`
	OCITag        string `json:"ociTag"`
	Values        Values `json:"values"`
}

type Values struct {
	Resources Resources             `json:"resources"`
	Network   Network               `json:"network"`
	Source    Source                `json:"source"`
	Storage   []Storage             `json:"storage"`
	Advanced  *runtime.RawExtension `json:"advanced,omitempty"`
}

type Resources struct {
	Replicas int     `json:"replicas"`
	CPU      float32 `json:"cpu"`
	Memory   int     `json:"memory"`
}

type AccessType string

const (
	Public  AccessType = "public"
	Private AccessType = "private"
)

type Protocol string

const (
	TCP Protocol = "tcp"
	UDP Protocol = "udp"
)

type Port struct {
	HostPort   int      `json:"host_port"`
	TargetPort int      `json:"target_port"`
	Protocol   Protocol `json:"protocol"`
}

type Network struct {
	Exposed bool       `json:"exposed"`
	Type    AccessType `json:"type"`
	Ports   []Port     `json:"ports"`
}

type Source struct {
	Repository string `json:"repository"`
	Command    string `json:"command"`
}

type Storage struct {
	Size      float32 `json:"size"`
	Name      string  `json:"name"`
	MountPath string  `json:"mount_path"`
}
