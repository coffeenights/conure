package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const ComponentKind = "Component"
const (
	ApplySetsAnnotation = "conure.io/apply-sets"
)

type ComponentConditionType string

func (t ComponentConditionType) String() string {
	return string(t)
}

type ComponentConditionReason string

func (t ComponentConditionReason) String() string {
	return string(t)
}

const (
	ComponentConditionTypeReady          ComponentConditionType   = "Ready"
	ComponentReadyPendingReason          ComponentConditionReason = "Pending"
	ComponentReadyRenderingReason        ComponentConditionReason = "Rendering"
	ComponentReadyRenderingFailedReason  ComponentConditionReason = "RenderingFailed"
	ComponentReadyRenderingSucceedReason ComponentConditionReason = "RenderingSucceed"
	ComponentReadyDeployingReason        ComponentConditionReason = "Deploying"
	ComponentReadyDeployingFailedReason  ComponentConditionReason = "DeployingFailed"
	ComponentReadyDeployingSucceedReason ComponentConditionReason = "DeployingSucceed"
	ComponentReadyRunningReason          ComponentConditionReason = "Running"
)

type ComponentSpec struct {
	ComponentType string                `json:"type"`
	Values        *runtime.RawExtension `json:"values"`
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

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+genclient

// ComponentDefinition is the Schema for the componentdefinitions API
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentDefinitionSpec `json:"spec,omitempty"`
}

type ComponentDefinitionSpec struct {
	ComponentType string `json:"type"`
	Description   string `json:"description"`
	OCIRepository string `json:"ociRepository"`
	OCITag        string `json:"ociTag"`
	OCIDigest     string `json:"ociDigest"`
	OCIRegistry   string `json:"ociRegistry,omitempty"`
}

//+kubebuilder:object:root=true

type ComponentDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{}, &ComponentDefinition{}, &ComponentDefinitionList{})
}
