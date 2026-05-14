// Package render defines the engine-agnostic interface that the component
// controller uses to render and apply manifests. Engine-specific code (Timoni
// CUE modules, Helm charts) lives in sibling packages and is wired in at
// controller startup.
package render

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ResourceSet is a named group of rendered Kubernetes objects. Timoni emits
// one ResourceSet per top-level CUE field; Helm renders a single set named
// after the chart. JSON tags match the legacy Timoni shape so apply-set
// annotations written before the multi-engine refactor remain decodable.
type ResourceSet struct {
	Name    string                       `json:"name"`
	Objects []*unstructured.Unstructured `json:"objects,omitempty"`
}
