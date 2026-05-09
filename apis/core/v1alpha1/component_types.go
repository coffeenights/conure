package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
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

// IsValid reports whether this reason is one of the known
// ComponentConditionReason constants. Used at the boundary in
// setConditionReady to reject typos.
func (t ComponentConditionReason) IsValid() bool {
	_, ok := knownComponentConditionReasons[t]
	return ok
}

// Component status uses three typed conditions following the K8s convention
// (multiple typed conditions, each with its own Status/Reason/Message):
//
//   - Rendered: True when the last template render succeeded
//   - Deployed: True when the last apply succeeded
//   - Ready:    True when both Rendered and Deployed are True (rollup)
//
// `kubectl get component` then shows useful per-step diagnostics, and
// Ready=True/False is a single meaningful boolean for clients.
const (
	ComponentConditionTypeReady    ComponentConditionType = "Ready"
	ComponentConditionTypeRendered ComponentConditionType = "Rendered"
	ComponentConditionTypeDeployed ComponentConditionType = "Deployed"

	// Reasons shared by Rendered and Deployed.
	ComponentReasonInProgress ComponentConditionReason = "InProgress"
	ComponentReasonSucceeded  ComponentConditionReason = "Succeeded"
	ComponentReasonFailed     ComponentConditionReason = "Failed"

	// Reasons used by the Ready rollup.
	ComponentReasonReady            ComponentConditionReason = "Ready"
	ComponentReasonRenderingFailed  ComponentConditionReason = "RenderingFailed"
	ComponentReasonDeploymentFailed ComponentConditionReason = "DeploymentFailed"
)

var knownComponentConditionReasons = map[ComponentConditionReason]struct{}{
	ComponentReasonInProgress:       {},
	ComponentReasonSucceeded:        {},
	ComponentReasonFailed:           {},
	ComponentReasonReady:            {},
	ComponentReasonRenderingFailed:  {},
	ComponentReasonDeploymentFailed: {},
}

type ComponentSpec struct {
	ComponentType string                `json:"type"`
	Values        *runtime.RawExtension `json:"values"`
}

type ComponentStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
	// ObservedGeneration is the spec generation that was last successfully
	// rendered and deployed. The reconciler skips re-rendering when
	// metadata.generation matches this value and the component is in a
	// terminal-success condition.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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
	// RegistrySecretRef references a Secret of type kubernetes.io/dockerconfigjson
	// in the controller's namespace, used to authenticate with the OCI registry
	// when pulling the module artifact. Optional; omit for public registries.
	RegistrySecretRef *corev1.LocalObjectReference `json:"registrySecretRef,omitempty"`
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
