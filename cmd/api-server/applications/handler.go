package applications

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

type ApiHandler struct {
	MongoDB    *database.MongoDB
	Config     *apiConfig.Config
	KeyStorage variables.SecretKeyStorage
	// Kube is the shared cluster client used by env-scoped read paths
	// (drift detection, orphan-CRD listing). May be nil in tests; handlers
	// that need it construct one lazily via k8sUtils.GetClientset().
	Kube *k8sUtils.GenericClientset
}

func NewApiHandler(config *apiConfig.Config, mongo *database.MongoDB, keyStorage variables.SecretKeyStorage, kube *k8sUtils.GenericClientset) *ApiHandler {
	return &ApiHandler{
		MongoDB:    mongo,
		Config:     config,
		KeyStorage: keyStorage,
		Kube:       kube,
	}
}

// kubeClient returns the handler's pre-wired clientset, or constructs a fresh
// one on demand. Lets handlers transparently work both in production (where
// the router pre-wires Kube) and in tests (where it's nil).
func (a *ApiHandler) kubeClient() (*k8sUtils.GenericClientset, error) {
	if a.Kube != nil {
		return a.Kube, nil
	}
	return k8sUtils.GetClientset()
}
