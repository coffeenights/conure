package providers

type NetworkProperties struct {
	IP         string  `json:"ip"`
	ExternalIP string  `json:"external_ip"`
	Host       string  `json:"host"`
	Ports      []int32 `json:"port"`
}

type ResourcesProperties struct {
	Replicas int32  `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

type StorageProperties struct {
	Size string `json:"size"`
}

type SourceProperties struct {
	ContainerImage string `json:"container_image"`
}
