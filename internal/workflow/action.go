package workflow

import (
	"context"
	"fmt"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConureSystemNamespace = "conure-system"
)

func GetActions(serviceType string) error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}
	workflow, err := clientset.Conure.CoreV1alpha1().Workflows(ConureSystemNamespace).Get(context.Background(), serviceType, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, action := range workflow.Spec.Actions {
		fmt.Println(action)
	}
	return nil
}

func RunAction() error {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return err
	}
	list, err := clientset.Conure.CoreV1alpha1().Workflows(ConureSystemNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, workflow := range list.Items {
		fmt.Println(workflow.Name)
	}
	return nil
}
