package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// Component describe functional units that may be instantiated as part of a larger distributed application
// ref: https://github.com/oam-dev/spec/blob/master/3.component_model.md
type Component struct {
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}
