package applications

import (
	"crypto/rand"
	"encoding/hex"
	"os"

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
	// WatcherID identifies this API replica when acquiring build leases.
	// Generated once per process at NewApiHandler. Tests may override.
	WatcherID string
}

func NewApiHandler(config *apiConfig.Config, mongo *database.MongoDB, keyStorage variables.SecretKeyStorage, kube *k8sUtils.GenericClientset) *ApiHandler {
	return &ApiHandler{
		MongoDB:    mongo,
		Config:     config,
		KeyStorage: keyStorage,
		Kube:       kube,
		WatcherID:  newWatcherID(),
	}
}

// newWatcherID returns a process-unique identifier used as the lease owner
// when watching build Jobs. Uniqueness across replicas is what makes the
// lease's conditional update safe under HA. Format is opaque; hostname is
// included only as a debugging aid.
func newWatcherID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "api"
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return host + "-" + hex.EncodeToString(b)
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
