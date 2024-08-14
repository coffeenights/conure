package k8s

import (
	"github.com/coffeenights/conure/pkg/client/core_conure"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type GenericClientset struct {
	Conure  *core_conure.Clientset
	K8s     *kubernetes.Clientset
	Dynamic *dynamic.DynamicClient
	Config  *rest.Config
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

	conure, err := core_conure.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &GenericClientset{Conure: conure, K8s: k8s, Dynamic: dynamicClient, Config: config}, nil
}
