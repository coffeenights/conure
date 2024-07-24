package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// Component A component is a part of an application and represents a single unit of deployment.
type Component struct {
	Name          string                `json:"name"`
	ComponentType string                `json:"type"`
	OCRRepository string                `json:"ocrRepository"`
	OCRTag        string                `json:"ocrTag"`
	Values        *runtime.RawExtension `json:"values"`
}
