package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationConditionType string

func (t ApplicationConditionType) String() string {
	return string(t)
}

type ApplicationConditionReason string

func (t ApplicationConditionReason) String() string {
	return string(t)
}

const (
	ApplicationConditionTypeStatus         ApplicationConditionType   = "Status"
	ApplicationStatusReasonRendering       ApplicationConditionReason = "RenderingComponent"
	ApplicationStatusReasonRenderingFailed ApplicationConditionReason = "RenderingComponentFailed"
	ApplicationStatusReasonDeployed        ApplicationConditionReason = "Deployed"
)

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	Components []ComponentTemplate `json:"components"`
}

type ApplicationComponentStatus struct {
	ComponentName string                   `json:"componentName"`
	ComponentType string                   `json:"componentType"`
	Reason        ComponentConditionReason `json:"reason"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	Conditions      []metav1.Condition           `json:"conditions,omitempty"`
	Components      []ApplicationComponentStatus `json:"components,omitempty"`
	ReadyComponents int                          `json:"readyComponents,omitempty"`
	TotalComponents int                          `json:"totalComponents,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
