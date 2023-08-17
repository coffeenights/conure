package applications

type ApplicationResponse struct {
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	EnvironmentId string        `json:"environment_id"`
	AccountId     uint64        `json:"account_id"`
	Components    []interface{} `json:"components"`
}

type ServiceComponentResponse struct {
	Name           string `json:"name"`
	Replicas       int32  `json:"replicas"`
	ContainerImage string `json:"container_image"`
	ContainerPort  int32  `json:"container_port"`
}
