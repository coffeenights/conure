package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const ComponentKind = "Component"

type ComponentSpec struct {
	ComponentType string `json:"type"`
	OCIRepository string `json:"ociRepository"`
	OCITag        string `json:"ociTag"`
	Values        Values `json:"values"`
}

type ComponentStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient

// Component A component is a part of an application and represents a single unit of deployment.
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

// ComponentTemplate is simply a template for adding inline components into an application.
type ComponentTemplate struct {
	ComponentTemplateMetadata `json:"metadata"`
	Spec                      ComponentSpec `json:"spec,omitempty"`
}

// ComponentTemplateMetadata is the metadata for a ComponentTemplate (Used this in replacement of metav1.ObjectMeta as it wasn't working from some reason).
type ComponentTemplateMetadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

//+kubebuilder:object:root=true

type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
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
