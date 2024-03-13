package applications

import (
	"errors"
	"fmt"
	"github.com/coffeenights/conure/internal/k8s"
	"github.com/oam-dev/kubevela-core-api/apis/core.oam.dev/common"
	"log"
)

type BackendType string

const (
	Kubernetes BackendType = "kubernetes"
)

type WorkloadName string

const (
	Deployment  WorkloadName = "Deployment"
	StatefulSet WorkloadName = "StatefulSet"
)

// Workload is an interface that represents different kinds of workloads.
// It provides methods to extract various properties of a workload such as network, resources, storage, and source properties.
type Workload interface {
	GetNetworkProperties() (*NetworkProperties, error)
	GetResourcesProperties() (*ResourcesProperties, error)
	GetStorageProperties() (*StorageProperties, error)
	GetSourceProperties() (*SourceProperties, error)
}

type WorkloadProperties struct {
	Name    WorkloadName
	Backend BackendType
}

type K8sWorkloadProperties struct {
	ComponentSpec   *common.ApplicationComponent
	ComponentStatus *common.ApplicationComponentStatus
}

type ApplicationProperties struct {
	ApplicationID  string
	OrganizationID string
	Environment    string
}

type K8sDeploymentWorkload struct {
	WorkloadProperties
	K8sWorkloadProperties
	*Application
}

func getNetworkPropertiesFromService(clientset *k8s.GenericClientset, namespace string, labels map[string]string, properties *NetworkProperties) error {
	services, err := getServicesByLabels(clientset, namespace, labels)
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
	traitsData, err := extractMapFromRawExtension(trait.Properties)
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
func (w *K8sDeploymentWorkload) GetNetworkProperties() (*NetworkProperties, error) {
	var properties NetworkProperties

	// Information from trait
	for _, trait := range w.ComponentSpec.Traits {
		if trait.Type == "expose" {
			err := getExposeTraitProperties(&trait, &properties)
			if err != nil {
				return nil, err
			}
		}
	}

	// Information from Service
	clientset, err := k8s.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return nil, err
	}
	filter := map[string]string{
		OrganizationIDLabel: w.Application.OrganizationID,
		ApplicationIDLabel:  w.Application.ID.Hex(),
		EnvironmentLabel:    w.Application.Environment,
	}
	err = getNetworkPropertiesFromService(clientset, w.Application.GetNamespace(), filter, &properties)
	if err != nil {
		switch {
		case !errors.Is(err, ErrServiceNotFound):
			return nil, err
		}
	}
	return &properties, nil
}

func (w *K8sDeploymentWorkload) GetResourcesProperties() (*ResourcesProperties, error) {
	var resources ResourcesProperties
	// Information from trait
	for _, trait := range w.ComponentSpec.Traits {
		if trait.Type == "scaler" {
			traitsData, err := extractMapFromRawExtension(trait.Properties)
			if err != nil {
				return nil, err
			}
			if replicas, ok := traitsData["replicas"].(interface{}); ok {
				resources.Replicas = int32(replicas.(float64))
			}
		}
	}
	propertiesData, err := extractMapFromRawExtension(w.ComponentSpec.Properties)
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

func (w *K8sDeploymentWorkload) GetStorageProperties() (*StorageProperties, error) {
	return &StorageProperties{}, nil
}

func (w *K8sDeploymentWorkload) GetSourceProperties() (*SourceProperties, error) {
	var source SourceProperties
	propertiesData, err := extractMapFromRawExtension(w.ComponentSpec.Properties)
	if err != nil {
		return nil, err
	}
	source.ContainerImage = ""
	if image, ok := propertiesData["image"].(string); ok {
		source.ContainerImage = image
	}
	return &source, nil
}

type K8sStatefulSetWorkload struct {
	WorkloadProperties
	K8sWorkloadProperties
	*Application
}

func (w *K8sStatefulSetWorkload) GetNetworkProperties() (*NetworkProperties, error) {
	var properties NetworkProperties

	// Information from trait
	for _, trait := range w.ComponentSpec.Traits {
		if trait.Type == "expose" {
			err := getExposeTraitProperties(&trait, &properties)
			if err != nil {
				return nil, err
			}
		}
	}

	// Information from Service
	clientset, err := k8s.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return nil, err
	}
	filter := map[string]string{
		OrganizationIDLabel: w.Application.OrganizationID,
		ApplicationIDLabel:  w.Application.ID.Hex(),
		EnvironmentLabel:    w.Application.Environment,
	}
	err = getNetworkPropertiesFromService(clientset, w.Application.GetNamespace(), filter, &properties)
	if err != nil {
		switch {
		case !errors.Is(err, ErrServiceNotFound):
			return nil, err
		}
	}
	return &properties, nil
}

func (w *K8sStatefulSetWorkload) GetResourcesProperties() (*ResourcesProperties, error) {
	return &ResourcesProperties{}, nil
}

func (w *K8sStatefulSetWorkload) GetStorageProperties() (*StorageProperties, error) {
	return &StorageProperties{}, nil
}

func (w *K8sStatefulSetWorkload) GetSourceProperties() (*SourceProperties, error) {
	return &SourceProperties{}, nil
}

func NewK8sWorkload(app *Application, spec *common.ApplicationComponent, status *common.ApplicationComponentStatus) (Workload, error) {
	wln := WorkloadName(status.WorkloadDefinition.Kind)
	bt := Kubernetes
	properties := WorkloadProperties{
		Name:    wln,
		Backend: bt,
	}
	k8sProperties := K8sWorkloadProperties{
		ComponentSpec:   spec,
		ComponentStatus: status,
	}
	switch wln {
	case Deployment:
		return &K8sDeploymentWorkload{
			WorkloadProperties:    properties,
			K8sWorkloadProperties: k8sProperties,
			Application:           app,
		}, nil
	case StatefulSet:
		return &K8sStatefulSetWorkload{
			WorkloadProperties:    properties,
			K8sWorkloadProperties: k8sProperties,
			Application:           app,
		}, nil
	}
	return nil, fmt.Errorf("workload name %s not supported", wln)
}
