package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// Component describe functional units that may be instantiated as part of a larger distributed application
// ref: https://github.com/oam-dev/spec/blob/master/3.component_model.md

type Component struct {
	Name       string                `json:"name"`
	Type       ComponentType         `json:"type"`
	Replicas   int32                 `json:"replicas"`
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}

type ComponentType string

// Basic component types available to be deployed
const (
	Service         ComponentType = "service"
	Worker          ComponentType = "worker"
	CronTask        ComponentType = "cron_task"
	StatefulService ComponentType = "stateful_service"
)

type ServiceComponentProperties struct {
	Image      string   `json:"image"`
	Command    []string `json:"cmd,omitempty"`
	Port       int32    `json:"port"`
	TargetPort int32    `json:"targetPort"`
}

type Environment struct {
	Name  string
	Value string
}
