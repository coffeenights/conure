package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const ComponentKind = "Component"
const (
	ApplySetsAnnotation = "conure.io/apply-sets"
	// RestartedAtAnnotation, when set on a Component, is propagated into the
	// pod-template annotations of every rendered workload (Deployment,
	// StatefulSet, etc.). Bumping it triggers a rolling restart of pods —
	// component types without a pod template simply ignore it.
	RestartedAtAnnotation = "conure.io/restartedAt"
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
	ComponentType string `json:"type"`
	// Engine optionally narrows ComponentDefinition lookup to a specific
	// rendering backend. Required only when more than one ComponentDefinition
	// shares the same spec.type (e.g. a Timoni and a Helm implementation of
	// "webservice" deployed side-by-side). When empty, the lookup expects a
	// single matching ComponentDefinition for the type.
	// +kubebuilder:validation:Enum=timoni;helm
	// +optional
	Engine ComponentEngine       `json:"engine,omitempty"`
	Values *runtime.RawExtension `json:"values"`
}

type ComponentStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
	// ObservedGeneration is the spec generation that was last successfully
	// rendered and deployed. The reconciler skips re-rendering when
	// metadata.generation matches this value and the component is in a
	// terminal-success condition.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ObservedRestartedAt is the value of the conure.io/restartedAt
	// annotation at the time of the last successful render. A restart is a
	// metadata-only change (no spec/generation bump), so the reconciler
	// uses this field to detect that the annotation has moved and a fresh
	// render is needed to propagate it into pod templates.
	ObservedRestartedAt string `json:"observedRestartedAt,omitempty"`
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

// ComponentEngine selects which rendering backend handles a ComponentDefinition.
// Empty defaults to EngineTimoni so existing ComponentDefinitions continue to
// work without modification.
type ComponentEngine string

const (
	EngineTimoni ComponentEngine = "timoni"
	EngineHelm   ComponentEngine = "helm"
)

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+genclient
//+genclient:nonNamespaced

// ComponentDefinition is the Schema for the componentdefinitions API
type ComponentDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentDefinitionSpec `json:"spec,omitempty"`
}

type ComponentDefinitionSpec struct {
	ComponentType string `json:"type"`
	Description   string `json:"description"`
	// Engine selects the rendering backend. Empty defaults to timoni.
	// +kubebuilder:validation:Enum=timoni;helm
	// +optional
	Engine        ComponentEngine `json:"engine,omitempty"`
	OCIRepository string          `json:"ociRepository"`
	OCITag        string          `json:"ociTag"`
	OCIDigest     string          `json:"ociDigest"`
	OCIRegistry   string          `json:"ociRegistry,omitempty"`
	// RegistrySecretName is the name of a kubernetes.io/dockerconfigjson
	// Secret in the controller's namespace used to authenticate the OCI
	// module pull. It is NOT user-authored: the API server resolves the
	// org-scoped logical credential (models.ComponentDefinition.CredentialRef)
	// to a concrete projected Secret and stamps that name here at deploy time.
	// Empty means anonymous pull (public registry). The controller consumes
	// this plain name and never resolves credentials itself.
	// +optional
	RegistrySecretName string `json:"registrySecretName,omitempty"`
	// Helm carries engine-specific configuration when Engine=helm. Ignored
	// otherwise. Helm charts do not have CUE-style schema validation, so
	// values typing falls back to whatever values.schema.json the chart ships.
	// +optional
	Helm *HelmEngineSpec `json:"helm,omitempty"`
	// Buildable declares that conure can build the component's container
	// image (vs. only deploying a prebuilt one). When false (or unset),
	// conure never attempts a build for components of this type and
	// `conure deploy` is promote-only. When true, the per-component
	// discriminator at FieldRoles["sourceType"] decides whether a given
	// component instance builds ("git") or deploys a prebuilt image ("oci").
	// +optional
	Buildable bool `json:"buildable,omitempty"`
	// FieldRoles maps conure's well-known field roles to dotted paths into
	// the component's values. conure owns the role vocabulary; the
	// definition owns where each role lives in its own #Config schema.
	// There is no default/fallback: a role read when it is needed but
	// absent here is a hard error (this platform is pre-1.0; definitions
	// must declare the roles their components use).
	//
	// Reserved roles:
	//   sourceType        discriminator; value is "git" (build) or "oci"
	//                     (deploy prebuilt). conure fixes this vocabulary.
	//   image.repository  application image repository (built or pulled)
	//   image.tag         application image tag
	//   git.repository    git URL to clone for a remote build (sourceType=git)
	//   git.branch        git branch to build
	//   build.tool        "dockerfile" | "railpack"
	//   build.location    "remote" | "local"
	//   build.dockerfile  path to the Dockerfile within the build context
	//
	// image.* is required whenever Buildable is true. git.*/build.* are
	// read only when a component's sourceType resolves to "git".
	// +optional
	FieldRoles map[string]string `json:"fieldRoles,omitempty"`
}

// HelmEngineSpec carries Helm-specific render options. All fields are optional.
type HelmEngineSpec struct {
	// ReleaseName overrides the value used for .Release.Name during template
	// rendering. Defaults to the Component name.
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`
	// Namespace overrides .Release.Namespace. Defaults to the Component's
	// namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// KubeVersion overrides .Capabilities.KubeVersion (e.g. "v1.29.0"). When
	// empty, the controller's discovery client supplies the live cluster
	// version.
	// +optional
	KubeVersion string `json:"kubeVersion,omitempty"`
	// APIVersions adds entries to .Capabilities.APIVersions. The controller's
	// discovery client supplies the base set; entries here are appended.
	// +optional
	APIVersions []string `json:"apiVersions,omitempty"`
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
