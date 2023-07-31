package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

type TraitType string

const (
	LoadBalancer TraitType = "load_balancer"
)

type Trait struct {
	Type       ComponentType         `json:"type"`
	Properties *runtime.RawExtension `json:"properties"`
}

type LoadBalancerTraitProperties struct {
}
