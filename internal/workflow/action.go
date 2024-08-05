package workflow

import (
	"context"
	"fmt"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConureSystemNamespace = "conure-system"
)

type ActionsHandler struct {
	Ctx       context.Context
	Clientset *k8sUtils.GenericClientset
	Actions   []coreconureiov1alpha1.Action
}

func NewActionsHandler(ctx context.Context) (*ActionsHandler, error) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, err
	}
	return &ActionsHandler{
		Ctx:       ctx,
		Clientset: clientset,
	}, nil
}

func (a *ActionsHandler) GetActions(serviceType string) error {
	workflow, err := a.Clientset.Conure.CoreV1alpha1().Workflows(ConureSystemNamespace).Get(a.Ctx, serviceType, metav1.GetOptions{})
	if err != nil {
		return err
	}
	a.Actions = workflow.Spec.Actions
	return nil
}

func (a *ActionsHandler) RunActions() error {
	for _, action := range a.Actions {
		err := a.RunAction(&action)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *ActionsHandler) RunAction(action *coreconureiov1alpha1.Action) error {
	fmt.Printf("Running action %s", action.Name)
	actionDefinition, err := a.Clientset.Conure.CoreV1alpha1().ActionDefinitions(ConureSystemNamespace).Get(a.Ctx, action.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	fmt.Println(actionDefinition)
	return nil
}
