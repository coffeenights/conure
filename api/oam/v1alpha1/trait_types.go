package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

type TraitType string

const (
	Gateway TraitType = "gateway"
)

type Trait struct {
	Type       ComponentType         `json:"type"`
	Properties *runtime.RawExtension `json:"properties"`
}

type GatewayTraitProperties struct {
	Rules []map[string]string `json:"rules"`
}
