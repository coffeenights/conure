package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActionDefinitionSpec defines the desired state of ActionDefinition
type ActionDefinitionSpec struct {
	OCIRepository string `json:"ociRepository"`
	OCITag        string `json:"ociTag"`
	ConfigDocs    string `json:"configDocs"`
}

// ActionDefinitionStatus defines the observed state of ActionDefinition
type ActionDefinitionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true

// ActionDefinition is the Schema for the actionDefinition API
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +genclient
type ActionDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActionDefinitionSpec   `json:"spec,omitempty"`
	Status ActionDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ActionDefinitionList contains a list of ActionDefinitions
type ActionDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ActionDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ActionDefinition{}, &ActionDefinitionList{})
}
