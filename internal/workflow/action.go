package workflow

import (
	"context"
	"fmt"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/k8s"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/stefanprodan/timoni/pkg/module"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ConureSystemNamespace = "conure-system"
)

type ActionsHandler struct {
	Ctx                context.Context
	Clientset          *k8sUtils.GenericClientset
	Actions            []coreconureiov1alpha1.Action
	OCIRepoCredentials string
	Namespace          string
}

func NewActionsHandler(ctx context.Context, Namespace string) (*ActionsHandler, error) {
	clientset, err := k8sUtils.GetClientset()
	if err != nil {
		return nil, err
	}
	return &ActionsHandler{
		Ctx:                ctx,
		Clientset:          clientset,
		OCIRepoCredentials: "", // TODO: Take it from integrations
		Namespace:          Namespace,
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
	logger := log.FromContext(a.Ctx)
	logger.Info("Running action", "action", action.Name)
	values, err := k8s.ExtractValuesFromRawExtension(action.Values)
	if err != nil {
		return err
	}
	modManager, err := module.NewManager(a.Ctx, actionDefinition.Name, actionDefinition.Spec.OCIRepository, actionDefinition.Spec.OCITag, a.Namespace, a.OCIRepoCredentials, values)
	if err != nil {
		return err
	}
	err = modManager.Apply()
	if err != nil {
		return err
	}
	return nil
}
