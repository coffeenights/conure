package workflow

import (
	"context"
	coreconureiov1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/stefanprodan/timoni/pkg/module"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ConureSystemNamespace = "conure-system"
)

type ActionsHandler struct {
	Ctx                context.Context
	Reconciler         *WorkflowReconciler
	Actions            []coreconureiov1alpha1.Action
	Workflow           *coreconureiov1alpha1.Workflow
	WorkflowRun        *coreconureiov1alpha1.WorkflowRun
	OCIRepoCredentials string
	Namespace          string
	ID                 string
}

func NewActionsHandler(ctx context.Context, Namespace string, wflw *coreconureiov1alpha1.Workflow, wflr *coreconureiov1alpha1.WorkflowRun, reconciler *WorkflowReconciler) *ActionsHandler {
	return &ActionsHandler{
		Ctx:                ctx,
		OCIRepoCredentials: "", // TODO: Take it from integrations
		Namespace:          Namespace,
		ID:                 k8sUtils.Generate8DigitHash(),
		Workflow:           wflw,
		WorkflowRun:        wflr,
		Reconciler:         reconciler,
	}
}

func (a *ActionsHandler) GetActions() []coreconureiov1alpha1.Action {
	a.Actions = a.Workflow.Spec.Actions
	return a.Actions
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
	logger.Info("Retrieving action definition", "action", action.Type)
	var actionDefinition coreconureiov1alpha1.ActionDefinition
	err := a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: ConureSystemNamespace, Name: action.Type}, &actionDefinition)
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
	sets, err := modManager.GetApplySets()
	if err != nil {
		return err
	}

	gvk, err := apiutil.GVKForObject(a.WorkflowRun, a.Reconciler.Scheme)
	if err != nil {
		return err
	}
	for _, set := range sets {
		for _, obj := range set.Objects {
			ownerRefs := []metav1.OwnerReference{
				{
					APIVersion:         gvk.GroupVersion().String(),
					Kind:               gvk.Kind,
					Name:               a.WorkflowRun.GetName(),
					UID:                a.WorkflowRun.GetUID(),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			}
			obj.SetOwnerReferences(ownerRefs)
			// Inject the action name as a label
			labels := obj.GetLabels()
			labels[coreconureiov1alpha1.WorkflowActionNamelabel] = action.Name
			obj.SetLabels(labels)
			_, err = modManager.ApplyObject(obj, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
