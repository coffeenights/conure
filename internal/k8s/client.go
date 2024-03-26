package k8s

import (
	"github.com/coffeenights/conure/pkg/client/oam_conure"
	coreOAMDevClientset "github.com/oam-dev/kubevela-core-api/pkg/generated/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type GenericClientset struct {
	Conure *oam_conure.Clientset
	K8s    *kubernetes.Clientset
	Vela   *coreOAMDevClientset.Clientset
	Config *rest.Config
}

func GetClientset() (*GenericClientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig :=
			clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	vela, err := coreOAMDevClientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	conure, err := oam_conure.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &GenericClientset{Conure: conure, K8s: k8s, Vela: vela, Config: config}, nil
}
