package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type WorkflowConditionType string

func (t WorkflowConditionType) String() string {
	return string(t)
}

type WorkflowConditionReason string

func (t WorkflowConditionReason) String() string {
	return string(t)
}

const (
	ConditionTypeRunningAction WorkflowConditionType = "RunningAction"

	RunningActionRenderingReason WorkflowConditionReason = "RunningActionRendering"
	RunningActionFailedReason    WorkflowConditionReason = "RunningActionFailed"
	RunningActionSucceedReason   WorkflowConditionReason = "RunningActionSucceed"
	RunningActionReason          WorkflowConditionReason = "RunningAction"

	ConditionTypeFinishedAction WorkflowConditionType   = "FinishedAction"
	FinishedActionReason        WorkflowConditionReason = "FinishedAction"

	ConditionTypeFinished     WorkflowConditionType   = "Finished"
	FinishedSuccesfullyReason WorkflowConditionReason = "FinishedSuccesfully"
	FinishedFailedReason      WorkflowConditionReason = "FinishedFailed"
)

type Action struct {
	Name   string                `json:"name"`
	Values *runtime.RawExtension `json:"values"`
}

// WorkflowSpec defines the desired state of Workflow
type WorkflowSpec struct {
	Actions []Action `json:"actions"`
}

// WorkflowStatus defines the observed state of Workflow
type WorkflowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient

// Workflow is the Schema for the workflows API
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkflowList contains a list of Workflow
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}

// WorkflowRunSpec defines the desired state of WorkflowRun
type WorkflowRunSpec struct {
	WorkflowName    string `json:"workflowName"`
	ApplicationName string `json:"applicationName"`
	ComponentName   string `json:"componentName"`
}

type WorkflowRunStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient

// WorkflowRun triggers and records the run of a workflow
type WorkflowRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowRunSpec   `json:"spec,omitempty"`
	Status WorkflowRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkflowRunList contains a list of WorkflowRun
type WorkflowRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkflowRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workflow{}, &WorkflowList{}, &WorkflowRun{}, &WorkflowRunList{})
}
