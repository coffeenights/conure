package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/coffeenights/conure/apis/vela"
	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/mitchellh/mapstructure"
	"io"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"log"
)

type VelaComponent struct {
	ComponentSpec   *vela.ApplicationComponent
	ComponentStatus *vela.ApplicationComponentStatus
}

type PVCStorageTrait struct {
	PVC []struct {
		Name      string
		MountPath string
		Resources struct {
			Requests struct {
				Storage string
			}
		}
	}
}

type WorkloadName string

const (
	Deployment  WorkloadName = "Deployment"
	StatefulSet WorkloadName = "StatefulSet"
)

const (
	ApplicationIDLabel   = "conure.io/application-id"
	OrganizationIDLabel  = "conure.io/organization-id"
	EnvironmentLabel     = "conure.io/environment"
	CreatedByLabel       = "conure.io/created-by"
	NamespaceLabel       = "conure.io/namespace"
	ComponentNameLabel   = "app.oam.dev/component"
	ApplicationNameLabel = "app.oam.dev/name"
	ComponentIDLabel     = "conure.io/component-id"
)

type ProviderStatusVela struct {
	OrganizationID  string
	ApplicationID   string
	Namespace       string
	VelaApplication *vela.Application
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

// TODO: Migrate this function to use the dynamic client
//func (p *ProviderStatusVela) WatchApplicationStatus() error {
//	clientset, err := k8sUtils.GetClientset()
//	if err != nil {
//		return err
//	}
//	watchInterface, err := clientset.Vela.CoreV1beta1().Applications(p.Namespace).Watch(context.Background(), metav1.ListOptions{})
//	if err != nil {
//		log.Fatalf("Error opening watch: %v", err)
//	}
//	defer watchInterface.Stop()
//	for event := range watchInterface.ResultChan() {
//		switch event.Type {
//		case watch.Added:
//			fmt.Println("Application added:", event.Object)
//		case watch.Modified:
//			fmt.Println("Application modified:", event.Object)
//		case watch.Deleted:
//			fmt.Println("Application deleted:", event.Object)
//		case watch.Error:
//			fmt.Println("Error:", event.Object)
//		}
//	}
//	return nil
//}

func (p *ProviderStatusVela) GetComponentStatus(componentName string) (*ComponentStatusHealth, error) {
	comp, err := p.getVelaComponent(componentName)
	if err != nil {
		return nil, err
	}
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		ApplicationIDLabel:  p.ApplicationID,
		OrganizationIDLabel: p.OrganizationID,
		NamespaceLabel:      p.Namespace,
		ComponentNameLabel:  comp.ComponentSpec.Name,
	}
	deployments, err := k8sUtils.GetDeploymentsByLabels(clientset.K8s, p.Namespace, labels)
	if err != nil {
		return nil, err
	}

	if len(deployments) == 0 {
		return nil, conureerrors.ErrComponentNotFound
	}
	deployment := deployments[0]

	status := &ComponentStatusHealth{
		Healthy: comp.ComponentStatus.Healthy,
		Message: comp.ComponentStatus.Message,
		Updated: deployment.ObjectMeta.CreationTimestamp.UTC(),
	}
	return status, nil
}

func (p *ProviderStatusVela) getVelaComponent(componentName string) (*VelaComponent, error) {
	velaComponent := &VelaComponent{}
	for _, componentSpec := range p.VelaApplication.Spec.Components {
		if componentSpec.Name == componentName {
			velaComponent.ComponentSpec = &componentSpec
			break
		}
	}
	if velaComponent.ComponentSpec == nil {
		return nil, conureerrors.ErrComponentNotFound
	}
	for _, componentStatus := range p.VelaApplication.Status.Services {
		if componentStatus.Name == componentName {
			velaComponent.ComponentStatus = &componentStatus
			break
		}
	}
	if velaComponent.ComponentStatus == nil {
		return nil, conureerrors.ErrComponentNotFound
	}
	return velaComponent, nil
}

func (p *ProviderStatusVela) GetNetworkProperties(componentName string) (*NetworkProperties, error) {
	var properties NetworkProperties
	velaComponent, err := p.getVelaComponent(componentName)
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

func (p *ProviderStatusVela) GetResourcesProperties(componentName string) (*ResourcesProperties, error) {
	var resources ResourcesProperties
	velaComponent, err := p.getVelaComponent(componentName)
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
			resources.Replicas = int32(traitsData["replicas"].(float64))
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

func (p *ProviderStatusVela) GetStorageProperties(componentName string) (*StorageProperties, error) {
	var storages StorageProperties
	storages.Volumes = []VolumeProperties{}
	velaComponent, err := p.getVelaComponent(componentName)
	if err != nil {
		return nil, err
	}
	for _, trait := range velaComponent.ComponentSpec.Traits {
		if trait.Type == "storage" {
			traitsData, err := k8sUtils.ExtractMapFromRawExtension(trait.Properties)
			if err != nil {
				return nil, err
			}
			var pvcTrait PVCStorageTrait
			err = mapstructure.Decode(traitsData, &pvcTrait)
			if err != nil {
				return nil, err
			}
			for _, pvc := range pvcTrait.PVC {
				size := "8Gi" // Default value per kubevela
				if pvc.Resources.Requests.Storage != "" {
					size = pvc.Resources.Requests.Storage
				}
				volume := VolumeProperties{
					Name: pvc.Name,
					Path: pvc.MountPath,
					Size: size,
				}
				storages.Volumes = append(storages.Volumes, volume)
			}
		}
	}
	for _, status := range velaComponent.ComponentStatus.Traits {
		if status.Type == "storage" {
			storages.Healthy = status.Healthy
		}
	}
	return &storages, nil
}

func (p *ProviderStatusVela) GetSourceProperties(componentName string) (*SourceProperties, error) {
	var source SourceProperties
	velaComponent, err := p.getVelaComponent(componentName)
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
	if cmd, ok := propertiesData["cmd"].([]interface{}); ok {
		var cmdStr string
		for i, c := range cmd {
			if i == len(cmd)-1 {
				cmdStr += c.(string)
				break
			}
			cmdStr += c.(string) + " "
		}
		source.Command = cmdStr
	}
	return &source, nil
}

func (p *ProviderStatusVela) GetActivity(componentID string) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}
	labels := map[string]string{
		ApplicationIDLabel:  p.ApplicationID,
		OrganizationIDLabel: p.OrganizationID,
		NamespaceLabel:      p.Namespace,
		ComponentIDLabel:    componentID,
	}
	deployments, err := k8sUtils.GetDeploymentsByLabels(clientset.K8s, p.Namespace, labels)
	if err != nil {
		return err
	}
	deploymentSelector := fields.SelectorFromSet(fields.Set{
		"involvedObject.kind": "Deployment",
		"involvedObject.name": componentID,
		"involvedObject.uid":  string(deployments[0].UID),
	})
	listOptions := metav1.ListOptions{
		FieldSelector: deploymentSelector.String(),
	}
	events, err := clientset.K8s.CoreV1().Events(p.Namespace).List(context.Background(), listOptions)
	if err != nil {
		return err
	}
	_ = events
	return nil
}

func (p *ProviderStatusVela) GetPodList(componentName string) ([]Pod, error) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, err
	}

	podSelector := fields.SelectorFromSet(fields.Set{
		ApplicationNameLabel: p.VelaApplication.Name,
		ComponentNameLabel:   componentName,
	})

	listOptions := metav1.ListOptions{
		LabelSelector: podSelector.String(),
	}
	pods, err := clientset.K8s.CoreV1().Pods(p.Namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}
	var podList []Pod
	for _, k8sPod := range pods.Items {
		pod := Pod{
			Name:  k8sPod.Name,
			Phase: string(k8sPod.Status.Phase),
		}
		for _, k8sCondition := range k8sPod.Status.Conditions {
			cond := PodCondition{
				Type:    string(k8sCondition.Type),
				Status:  string(k8sCondition.Status),
				Reason:  k8sCondition.Reason,
				Message: k8sCondition.Message,
			}
			pod.Conditions = append(pod.Conditions, cond)
		}
		podList = append(podList, pod)
	}
	return podList, nil
}

func (p *ProviderStatusVela) StreamLogs(c context.Context, podName string, logStream *LogStream, linesBuffer int) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		logStream.Error <- err
		return
	}
	podLogOpts := corev1.PodLogOptions{
		Follow: true,
	}

	req := clientset.K8s.CoreV1().Pods(p.Namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(c)
	var statusError *k8sErrors.StatusError
	if err != nil {
		if errors.As(err, &statusError) {
			if statusError.ErrStatus.Code == 404 {
				logStream.Error <- conureerrors.ErrPodNotFound
			}
		} else {
			logStream.Error <- err
		}
		return
	}

	reader := bufio.NewReader(podLogs)
	lines := make([]string, linesBuffer)
	for {
		var str string
		for i := 0; i < len(lines); i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				logStream.Error <- err
				return
			}
			str = fmt.Sprintf("%s: %s", podName, line)
			logStream.Stream <- str
		}
	}
}

func getNetworkPropertiesFromService(clientset *k8sUtils.GenericClientset, namespace string, labels map[string]string, properties *NetworkProperties) error {
	services, err := k8sUtils.GetServicesByLabels(clientset, namespace, labels)
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}
	if len(services) == 0 {
		return k8sUtils.ErrServiceNotFound
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

func getExposeTraitProperties(trait *vela.ApplicationTrait, properties *NetworkProperties) error {
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
	OrganizationID  string
	ApplicationID   string
	ApplicationName string
	Namespace       string
	Environment     string
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
	var statusError *k8sErrors.StatusError

	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return err
	}
	// Create namespace if necessary
	err = p.createNamespace(clientset)
	if errors.As(err, &statusError) {
		if statusError.ErrStatus.Code == 409 {
			log.Printf("Namespace already exists, reusing it\n")
		} else {
			return err
		}
	} else if err != nil {
		return err
	}

	deploymentRes := schema.GroupVersionResource{Group: "core.oam.dev", Version: "v1beta1", Resource: "applications"}
	deployment := &unstructured.Unstructured{
		Object: manifest,
	}
	result, err := clientset.Dynamic.Resource(deploymentRes).Namespace(p.Namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		if errors.As(err, &statusError) {
			if statusError.ErrStatus.Code == 409 {
				log.Printf("Application already exists\n")
				return conureerrors.ErrApplicationExists
			}
		}
		return err
	}
	log.Printf("Created deployment %q.\n", result.GetName())
	return nil
}

func (p *ProviderDispatcherVela) UpdateApplication(manifest map[string]interface{}) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		return err
	}
	deploymentRes := schema.GroupVersionResource{Group: "core.oam.dev", Version: "v1beta1", Resource: "applications"}
	resource, err := clientset.Dynamic.Resource(deploymentRes).Namespace(p.Namespace).Get(context.Background(), p.ApplicationName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	deployment := &unstructured.Unstructured{
		Object: manifest,
	}
	deployment.SetResourceVersion(resource.GetResourceVersion())
	result, err := clientset.Dynamic.Resource(deploymentRes).Namespace(p.Namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	log.Printf("Updated deployment %q.\n", result.GetName())
	return nil
}
