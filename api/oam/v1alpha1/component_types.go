package v1alpha1

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
)

// Component describe functional units that may be instantiated as part of a larger distributed application
// ref: https://github.com/oam-dev/spec/blob/master/3.component_model.md

type ComponentPropertiesInterface interface {
}

type Component struct {
	Name       string                `json:"name"`
	Type       ComponentType         `json:"type"`
	Properties *runtime.RawExtension `json:"properties,omitempty"`
}

func (c *Component) ComponentProperties() (*ComponentPropertiesInterface, error) {
	var componentProperties ComponentPropertiesInterface
	switch c.Type {
	case Service:
		componentProperties = ServiceComponentProperties{}
		if err := json.Unmarshal(c.Properties.Raw, &componentProperties); err != nil {
			return &componentProperties, err
		}
	}
	return &componentProperties, nil
}

type ComponentType string

const (
	Service ComponentType = "service"
	//Worker          ComponentType = "worker"
	//CronTask        ComponentType = "cron_task"
	//StatefulService ComponentType = "stateful_service"
)

type ServiceComponent struct {
	Component
	// Workload   *K8sDeploymentWorkloadType
	Properties *ServiceComponentProperties
}

type ServiceComponentProperties struct {
	Image   string   `json:"image"`
	Command []string `json:"cmd,omitempty"`
}

type Environment struct {
	Name  string
	Value string
}
