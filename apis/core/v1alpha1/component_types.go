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
	Replicas int    `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
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
	HostPort   int      `json:"hostPort"`
	TargetPort int      `json:"targetPort"`
	Protocol   Protocol `json:"protocol"`
}

type Network struct {
	Exposed bool       `json:"exposed"`
	Type    AccessType `json:"type"`
	Ports   []Port     `json:"ports"`
}

type Source struct {
	SourceType           string   `json:"sourceType"`
	GitRepository        string   `json:"gitRepository,omitempty"`
	GitBranch            string   `json:"gitBranch,omitempty"`
	BuildTool            string   `json:"buildTool,omitempty"`
	DockerfilePath       string   `json:"dockerfilePath,omitempty"`
	NixpackPath          string   `json:"nixpackPath,omitempty"`
	OCIRepository        string   `json:"ociRepository,omitempty"`
	Tag                  string   `json:"tag,omitempty"`
	Command              []string `json:"command"`
	WorkingDir           string   `json:"workingDir"`
	ImagePullSecretsName string   `json:"imagePullSecretsName"`
}

type Storage struct {
	Size      string `json:"size"`
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}
