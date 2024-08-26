package workflow

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/stefanprodan/timoni/pkg/module"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ConureSystemNamespace = "conure-system"
)

type ActionsHandler struct {
	Ctx                context.Context
	Client             client.Client
	Actions            []coreconureiov1alpha1.Action
	Workflow           *coreconureiov1alpha1.Workflow
	OCIRepoCredentials string
	Namespace          string
	ID                 string
}

func NewActionsHandler(ctx context.Context, client client.Client, Namespace string, wflw *coreconureiov1alpha1.Workflow) (*ActionsHandler, error) {
	return &ActionsHandler{
		Ctx:                ctx,
		Client:             client,
		OCIRepoCredentials: "", // TODO: Take it from integrations
		Namespace:          Namespace,
		ID:                 k8sUtils.Generate8DigitHash(),
		Workflow:           wflw,
	}, nil
}

func (a *ActionsHandler) GetActions() error {
	a.Actions = a.Workflow.Spec.Actions
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
	var actionDefinition coreconureiov1alpha1.ActionDefinition
	err := a.Client.Get(a.Ctx, client.ObjectKey{Namespace: ConureSystemNamespace, Name: action.Name}, &actionDefinition)
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
