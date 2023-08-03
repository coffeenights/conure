package v1alpha1

import (
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
)

type MissingField struct {
	Field string
	Err   error
}

func (e *MissingField) Error() string { return "Missing field " + e.Field + e.Err.Error() }

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

func (s *ServiceComponentProperties) Parse(raw []byte) error {
	err := json.Unmarshal(raw, s)
	if err != nil {
		return err
	}
	if s.Port == 0 {
		return fmt.Errorf("missing 'port' field in the manifest")
	}
	if s.TargetPort == 0 {
		return fmt.Errorf("missing 'targetPort' field in the manifest")
	}
	if s.Image == "" {
		return fmt.Errorf("missing 'image' field in the manifest")
	}
	return nil
}

type Environment struct {
	Name  string
	Value string
}
