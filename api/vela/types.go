package vela

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ApplicationPhase is a label for the condition of an application at the current time
type ApplicationPhase string

const (
	// ApplicationStarting means the app is preparing for reconcile
	ApplicationStarting ApplicationPhase = "starting"
	// ApplicationRendering means the app is rendering
	ApplicationRendering ApplicationPhase = "rendering"
	// ApplicationPolicyGenerating means the app is generating policies
	ApplicationPolicyGenerating ApplicationPhase = "generatingPolicy"
	// ApplicationRunningWorkflow means the app is running workflow
	ApplicationRunningWorkflow ApplicationPhase = "runningWorkflow"
	// ApplicationWorkflowSuspending means the app's workflow is suspending
	ApplicationWorkflowSuspending ApplicationPhase = "workflowSuspending"
	// ApplicationWorkflowTerminated means the app's workflow is terminated
	ApplicationWorkflowTerminated ApplicationPhase = "workflowTerminated"
	// ApplicationWorkflowFailed means the app's workflow is failed
	ApplicationWorkflowFailed ApplicationPhase = "workflowFailed"
	// ApplicationRunning means the app finished rendering and applied result to the cluster
	ApplicationRunning ApplicationPhase = "running"
	// ApplicationUnhealthy means the app finished rendering and applied result to the cluster, but still unhealthy
	ApplicationUnhealthy ApplicationPhase = "unhealthy"
	// ApplicationDeleting means application is being deleted
	ApplicationDeleting ApplicationPhase = "deleting"
)

// ApplicationTrait defines the trait of application
type ApplicationTrait struct {
	Type string `json:"type"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}

// ApplicationComponent describe the component of application
type ApplicationComponent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// ExternalRevision specified the component revisionName
	ExternalRevision string `json:"externalRevision,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties *runtime.RawExtension `json:"properties,omitempty"`

	DependsOn []string `json:"dependsOn,omitempty"`

	// Traits define the trait of one component, the type must be array to keep the order.
	Traits []ApplicationTrait `json:"traits,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// scopes in ApplicationComponent defines the component-level scopes
	// the format is <scope-type:scope-instance-name> pairs, the key represents type of `ScopeDefinition` while the value represent the name of scope instance.
	Scopes map[string]string `json:"scopes,omitempty"`

	// ReplicaKey is not empty means the component is replicated. This field is designed so that it can't be specified in application directly.
	// So we set the json tag as "-". Instead, this will be filled when using replication policy.
	ReplicaKey string `json:"-"`
}

// ApplicationComponentStatus record the health status of App component
type ApplicationComponentStatus struct {
	Name      string                   `json:"name"`
	Namespace string                   `json:"namespace,omitempty"`
	Cluster   string                   `json:"cluster,omitempty"`
	Env       string                   `json:"env,omitempty"`
	Healthy   bool                     `json:"healthy"`
	Message   string                   `json:"message,omitempty"`
	Traits    []ApplicationTraitStatus `json:"traits,omitempty"`
	Scopes    []corev1.ObjectReference `json:"scopes,omitempty"`
}

type ApplicationTraitStatus struct {
	Type    string `json:"type"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type ApplicationSpec struct {
	Components []ApplicationComponent `json:"components"`
}

// Revision has name and revision number
type Revision struct {
	Name     string `json:"name"`
	Revision int64  `json:"revision"`

	// RevisionHash record the hash value of the spec of ApplicationRevision object.
	RevisionHash string `json:"revisionHash,omitempty"`
}

type AppStatus struct {
	ConditionedStatus `json:",inline"`

	// The generation observed by the application controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	Phase ApplicationPhase `json:"status,omitempty"`

	// Components record the related Components created by Application Controller
	Components []corev1.ObjectReference `json:"components,omitempty"`

	// Services record the status of the application services
	Services []ApplicationComponentStatus `json:"services,omitempty"`

	LatestRevision *Revision `json:"latestRevision,omitempty"`
}

type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec `json:"spec,omitempty"`
	Status AppStatus       `json:"status,omitempty"`
}

type ConditionedStatus struct {
	// Conditions of the resource.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

type Condition struct {
	// Type of this condition. At most one of each condition type may apply to
	// a resource at any point in time.
	Type ConditionType `json:"type"`

	// Status of this condition; is it currently True, False, or Unknown?
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time this condition transitioned from one
	// status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// A Reason for this condition's last transition from one status to another.
	Reason ConditionReason `json:"reason"`

	// A Message containing details about this condition's last transition from
	// one status to another, if any.
	// +optional
	Message string `json:"message,omitempty"`
}

type ConditionType string

// Condition types.
const (
	// TypeReady resources are believed to be ready to handle work.
	TypeReady ConditionType = "Ready"

	// TypeSynced resources are believed to be in sync with the
	// Kubernetes resources that manage their lifecycle.
	TypeSynced ConditionType = "Synced"
)

// A ConditionReason represents the reason a resource is in a condition.
// nolint
type ConditionReason string

// Reasons a resource is or is not ready.
const (
	ReasonAvailable   ConditionReason = "Available"
	ReasonUnavailable ConditionReason = "Unavailable"
	ReasonCreating    ConditionReason = "Creating"
	ReasonDeleting    ConditionReason = "Deleting"
)

// Reasons a resource is or is not synced.
const (
	ReasonReconcileSuccess ConditionReason = "ReconcileSuccess"
	ReasonReconcileError   ConditionReason = "ReconcileError"
)
