package workflow

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/coffeenights/conure/internal/timoni"
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
	ID                 string
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
		ID:                 k8sUtils.Generate8DigitHash(),
	}, nil
}

func (a *ActionsHandler) GetActions(workflowName string) error {
	workflow, err := a.Clientset.Conure.CoreV1alpha1().Workflows(a.Namespace).Get(a.Ctx, workflowName, metav1.GetOptions{})
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
	logger := log.FromContext(a.Ctx)
	logger.Info("Retrieving action definition", "action", action.Name)
	actionDefinition, err := a.Clientset.Conure.CoreV1alpha1().ActionDefinitions(ConureSystemNamespace).Get(a.Ctx, action.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	logger.Info("Running action", "action", action.Name)
	values := timoni.Values{}
	if err = values.ExtractFromRawExtension(action.Values); err != nil {
		return err
	}
	values["nameSuffix"] = a.ID
	modManager, err := module.NewManager(a.Ctx, actionDefinition.Name, actionDefinition.Spec.OCIRepository, actionDefinition.Spec.OCITag, a.Namespace, a.OCIRepoCredentials, values.Get())
	if err != nil {
		return err
	}
	err = modManager.Apply()
	if err != nil {
		return err
	}
	return nil
}
