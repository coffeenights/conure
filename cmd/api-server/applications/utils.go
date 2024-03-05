package applications

import (
	"context"
	"fmt"
	k8sV1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
)

const (
	DeploymentResourceType  = "deployment"
	StatefulSetResourceType = "statefulset"
	ServiceResourceType     = "service"
)

func getDeploymentByLabels(clientset *kubernetes.Clientset, namespace string, labels map[string]string) ([]k8sV1.Deployment, error) {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	var labelSelector []string
	for key, value := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", key, value))
	}

	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	}

	deployments, err := deploymentsClient.List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	if len(deployments.Items) == 0 {
		return nil, fmt.Errorf("no deployment found with label selector: %s", labelSelector)
	}

	return deployments.Items, nil
}

func getStatefulSetByLabels(clientset *kubernetes.Clientset, namespace string, labels map[string]string) ([]k8sV1.StatefulSet, error) {
	statefulSetsClient := clientset.AppsV1().StatefulSets(namespace)
	var labelSelector []string
	for key, value := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", key, value))
	}

	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	}

	statefulsets, err := statefulSetsClient.List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	if len(statefulsets.Items) == 0 {
		return nil, fmt.Errorf("no statefulset found with label selector: %s", labelSelector)
	}

	return statefulsets.Items, nil
}

func getServicesByLabels(clientset *kubernetes.Clientset, namespace string, labels map[string]string) ([]corev1.Service, error) {
	servicesClient := clientset.CoreV1().Services(namespace)

	var labelSelector []string
	for key, value := range labels {
		labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", key, value))
	}

	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join(labelSelector, ","),
	}

	services, err := servicesClient.List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}

	return services.Items, nil
}

func GetResourceByLabel(resourceType string, clientset *kubernetes.Clientset, namespace string, labels map[string]string) (interface{}, error) {
	switch resourceType {
	case DeploymentResourceType:
		return getDeploymentByLabels(clientset, namespace, labels)
	case StatefulSetResourceType:
		return getStatefulSetByLabels(clientset, namespace, labels)
	case ServiceResourceType:
		return getServicesByLabels(clientset, namespace, labels)
	}
	return nil, fmt.Errorf("resource type %s not supported", resourceType)
}
