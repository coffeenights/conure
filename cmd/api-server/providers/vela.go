package providers

import (
	"context"
	"errors"
	"fmt"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"log"
)

type VelaComponent struct {
	ComponentSpec   *common.ApplicationComponent
	ComponentStatus *common.ApplicationComponentStatus
}

type WorkloadName string

const (
	Deployment  WorkloadName = "Deployment"
	StatefulSet WorkloadName = "StatefulSet"
)

const (
	ApplicationIDLabel  = "conure.io/application-id"
	OrganizationIDLabel = "conure.io/organization-id"
	EnvironmentLabel    = "conure.io/environment"
	CreatedByLabel      = "conure.io/created-by"
	NamespaceLabel      = "conure.io/namespace"
)

type ProviderStatusVela struct {
	OrganizationID  string
	ApplicationID   string
	Namespace       string
	VelaApplication *v1beta1.Application
}

func NewProviderStatusVela(organizationID string, applicationID string, namespace string) (*ProviderStatusVela, error) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return nil, err
	}
	filter := map[string]string{
		OrganizationIDLabel: organizationID,
		ApplicationIDLabel:  applicationID,
	}

	velaApplication, err := k8sUtils.GetApplicationByLabels(clientset, namespace, filter)
	if err != nil {
		return nil, err
	}

	return &ProviderStatusVela{
		OrganizationID:  organizationID,
		ApplicationID:   applicationID,
		Namespace:       namespace,
		VelaApplication: velaApplication,
	}, nil
}

func (p *ProviderStatusVela) GetApplicationStatus() (string, error) {
	return string(p.VelaApplication.Status.Phase), nil
}

func (p *ProviderStatusVela) getVelaComponent(componentID string) (*VelaComponent, error) {
	velaComponent := &VelaComponent{}
	for _, componentSpec := range p.VelaApplication.Spec.Components {
		if componentSpec.Name == componentID {
			velaComponent.ComponentSpec = &componentSpec
			break
		}
	}
	if velaComponent.ComponentSpec == nil {
		return nil, ErrComponentNotFound
	}
	for _, componentStatus := range p.VelaApplication.Status.Services {
		if componentStatus.Name == componentID {
			velaComponent.ComponentStatus = &componentStatus
			break
		}
	}
	if velaComponent.ComponentStatus == nil {
		return nil, ErrComponentNotFound
	}
	return velaComponent, nil
}

func (p *ProviderStatusVela) GetNetworkProperties(componentID string) (*NetworkProperties, error) {
	var properties NetworkProperties
	velaComponent, err := p.getVelaComponent(componentID)
	if err != nil {
		return nil, err
	}
	// Information from trait
	for _, trait := range velaComponent.ComponentSpec.Traits {
		if trait.Type == "expose" {
			err := getExposeTraitProperties(&trait, &properties)
			if err != nil {
				return nil, err
			}
		}
	}

	// Information from Service
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return nil, err
	}
	filter := map[string]string{
		OrganizationIDLabel: p.OrganizationID,
		ApplicationIDLabel:  p.ApplicationID,
	}
	err = getNetworkPropertiesFromService(clientset, p.Namespace, filter, &properties)
	if err != nil {
		switch {
		case !errors.Is(err, k8sUtils.ErrServiceNotFound):
			return nil, err
		}
	}
	return &properties, nil
}

func (p *ProviderStatusVela) GetResourcesProperties(componentID string) (*ResourcesProperties, error) {
	var resources ResourcesProperties
	velaComponent, err := p.getVelaComponent(componentID)
	if err != nil {
		return nil, err
	}
	// Information from trait
	for _, trait := range velaComponent.ComponentSpec.Traits {
		if trait.Type == "scaler" {
			traitsData, err := k8sUtils.ExtractMapFromRawExtension(trait.Properties)
			if err != nil {
				return nil, err
			}
			if replicas, ok := traitsData["replicas"].(interface{}); ok {
				resources.Replicas = int32(replicas.(float64))
			}
		}
	}
	propertiesData, err := k8sUtils.ExtractMapFromRawExtension(velaComponent.ComponentSpec.Properties)
	if err != nil {
		return nil, err
	}
	if propertiesData["cpu"] == nil || propertiesData["memory"] == nil {
		return nil, fmt.Errorf("cpu or memory not found in properties")
	}
	resources.CPU = propertiesData["cpu"].(string)
	resources.Memory = propertiesData["memory"].(string)
	return &resources, nil
}

func (p *ProviderStatusVela) GetStorageProperties(componentID string) (*StorageProperties, error) {
	return nil, nil
}

func (p *ProviderStatusVela) GetSourceProperties(componentID string) (*SourceProperties, error) {
	var source SourceProperties
	velaComponent, err := p.getVelaComponent(componentID)
	if err != nil {
		return nil, err
	}
	propertiesData, err := k8sUtils.ExtractMapFromRawExtension(velaComponent.ComponentSpec.Properties)
	if err != nil {
		return nil, err
	}
	source.ContainerImage = ""
	if image, ok := propertiesData["image"].(string); ok {
		source.ContainerImage = image
	}
	return &source, nil
}

func getNetworkPropertiesFromService(clientset *k8sUtils.GenericClientset, namespace string, labels map[string]string, properties *NetworkProperties) error {
	services, err := k8sUtils.GetServicesByLabels(clientset, namespace, labels)
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}
	if len(services) == 0 {
		return fmt.Errorf("no services found with labels: %v", labels)
	}
	service := services[0]
	properties.IP = service.Spec.ClusterIP
	properties.ExternalIP = ""
	if service.Spec.Type == "LoadBalancer" {
		if len(service.Status.LoadBalancer.Ingress) != 0 {
			properties.ExternalIP = service.Status.LoadBalancer.Ingress[0].IP
		}
	}
	return nil
}

func getExposeTraitProperties(trait *common.ApplicationTrait, properties *NetworkProperties) error {
	traitsData, err := k8sUtils.ExtractMapFromRawExtension(trait.Properties)
	if err != nil {
		return err
	}
	if ports, ok := traitsData["port"].([]interface{}); ok {
		for _, p := range ports {
			properties.Ports = append(properties.Ports, int32(p.(float64)))
		}
	}
	return nil
}

type ProviderDispatcherVela struct {
	OrganizationID string
	ApplicationID  string
	Namespace      string
	Environment    string
}

func (p *ProviderDispatcherVela) createNamespace(clientset *k8sUtils.GenericClientset) error {
	options := metav1.CreateOptions{}
	namespaceManifest := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.Namespace,
			Labels: map[string]string{
				ApplicationIDLabel:  p.ApplicationID,
				OrganizationIDLabel: p.OrganizationID,
				EnvironmentLabel:    p.Environment,
			},
		},
	}
	_, err := clientset.K8s.CoreV1().Namespaces().Create(context.Background(), &namespaceManifest, options)
	if err != nil {
		return err
	}
	return nil
}

func (p *ProviderDispatcherVela) DeployApplication(manifest map[string]interface{}) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return err
	}
	// Create namespace if necessary
	err = p.createNamespace(clientset)

	deploymentRes := schema.GroupVersionResource{Group: "core.oam.dev", Version: "v1beta1", Resource: "applications"}
	deployment := &unstructured.Unstructured{
		Object: manifest,
	}
	result, err := clientset.Dynamic.Resource(deploymentRes).Namespace(p.Namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	var statusError *k8sErrors.StatusError
	if errors.As(err, &statusError) {
		if statusError.ErrStatus.Code == 409 {
			log.Printf("Application already exists\n")
			return ErrApplicationExists
		}
	} else if err != nil {
		return err
	}
	log.Printf("Created deployment %q.\n", result.GetName())
	return nil
}
