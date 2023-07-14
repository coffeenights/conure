package v1alpha1

// Component describe functional units that may be instantiated as part of a larger distributed application
// ref: https://github.com/oam-dev/spec/blob/master/3.component_model.md

type Component struct {
	Name string        `json:"name"`
	Type ComponentType `json:"type"`
	// Properties *runtime.RawExtension `json:"properties,omitempty"`
}

type ComponentType string

const (
	Service         ComponentType = "service"
	Worker          ComponentType = "worker"
	CronTask        ComponentType = "cron_task"
	StatefulService ComponentType = "stateful_service"
)

type Environment struct {
	Name  string
	Value string
}

type ServiceComponent struct {
	Component
	Image string
	Port  int32
}
