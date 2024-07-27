package settings

type CreateIntegrationRequest struct {
	Name            string `json:"name" validate:"required"`
	IntegrationType string `json:"integration_type" validate:"required"`
	// Integration value could be a dictionary or a string depending on the integration type
	IntegrationValue interface{} `json:"integration_value" validate:"required"`
}
