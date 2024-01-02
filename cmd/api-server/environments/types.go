package environments

import (
	"time"
)

type EnvironmentResponse struct {
	ResourceID    string    `json:"resource_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	EnvironmentId string    `json:"environment_id"`
	AccountId     uint64    `json:"account_id"`
	Created       time.Time `json:"created"`
}

//func (r *EnvironmentResponse) FromClientsetToResponse(item *metav1) {
//	r.ResourceID = string(item.ObjectMeta.UID)
//	r.Name = item.ObjectMeta.Name
//	r.Description = item.ObjectMeta.Namespace
//	r.EnvironmentId = ""
//	r.AccountId = 0
//	r.Created = item.ObjectMeta.CreationTimestamp.UTC()
//}
