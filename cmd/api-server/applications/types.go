package applications

type ApplicationResponse struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	EnvironmentId string `json:"environment_id"`
	AccountId     uint64 `json:"account_id"`
}
