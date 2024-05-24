package providers

import "time"

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

type VolumeProperties struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size string `json:"size"`
}

type StorageProperties struct {
	Volumes []VolumeProperties `json:"volumes"`
	Healthy bool               `json:"health"`
}

type SourceProperties struct {
	ContainerImage string `json:"container_image"`
	Command        string `json:"command"`
}

type ComponentStatusHealth struct {
	Healthy bool      `json:"healthy"`
	Message string    `json:"message"`
	Updated time.Time `json:"updated"`
}

type LogStream struct {
	Stream chan string
	Done   chan bool
	Error  chan error
}

func NewLogStream() *LogStream {
	return &LogStream{
		Stream: make(chan string),
		Error:  make(chan error),
	}
}

func (l *LogStream) Close() {
	close(l.Stream)
	close(l.Error)
}
