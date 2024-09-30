package application

import (
	"context"
	"encoding/json"
	"fmt"
	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/go-logr/logr"
	"github.com/stefanprodan/timoni/pkg/module"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

type ApplicationHandler struct {
	Application *conurev1alpha1.Application
	Reconciler  *ApplicationReconciler
	Ctx         context.Context
	Logger      logr.Logger
}

func NewApplicationHandler(ctx context.Context, application *conurev1alpha1.Application, reconciler *ApplicationReconciler) (*ApplicationHandler, error) {
	var handler ApplicationHandler
	handler.Logger = log.FromContext(ctx)
	handler.Application = application
	handler.Ctx = ctx
	handler.Reconciler = reconciler
	return &handler, nil
}

func (a *ApplicationHandler) ReconcileComponents() error {
	for _, component := range a.Application.Spec.Components {
		err := a.ReconcileComponent(&component)
		if err != nil {
			return err
		}
	}
	readyComponents := 0
	for _, component := range a.Application.Status.Components {
		if component.Reason == conurev1alpha1.ComponentReadyRunningReason {
			readyComponents++
		}
	}
	a.Application.Status.ReadyComponents = readyComponents
	a.Application.Status.TotalComponents = len(a.Application.Spec.Components)
	return common.ApplyStatus(a.Ctx, a.Application, a.Reconciler.Client)
}

func (a *ApplicationHandler) ReconcileComponent(componentTemp *conurev1alpha1.ComponentTemplate) error {
	var (
		wflr              conurev1alpha1.WorkflowRun
		component         conurev1alpha1.Component
		existingComponent conurev1alpha1.Component
	)
	// Build the component object
	component.ObjectMeta = metav1.ObjectMeta{
		Name:        componentTemp.Name,
		Annotations: componentTemp.Annotations,
		Namespace:   a.Application.Namespace,
	}
	component.Spec = componentTemp.Spec
	component.TypeMeta = metav1.TypeMeta{
		Kind:       conurev1alpha1.ComponentKind,
		APIVersion: a.Application.APIVersion,
	}
	specHashTarget := common.GetHashForSpec(&component.Spec)
	component.Labels = common.SetHashToLabels(componentTemp.Labels, specHashTarget)
	if err := ctrl.SetControllerReference(a.Application, &component, a.Reconciler.Scheme); err != nil {
		return err
	}

	// Find an existing component
	err := a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: component.Name}, &existingComponent)

	// If the component does not exist, create it and run its workflow
	if apierrors.IsNotFound(err) {
		a.Logger.Info("Creating component", "component", component.Name)
		if err = a.setRenderingComponentStatus(component.Name); err != nil {
			return err
		}
		if err = a.createComponent(&component); err != nil {
			if err2 := a.setRenderingComponentFailedStatus(component.Name); err2 != nil {
				return err2
			}
			return err
		} else {
			return nil
		}
	} else if err != nil {
		return err
	}

	// If the component exists, update it
	if err = a.updateComponent(&component, &existingComponent, specHashTarget); err != nil {
		return err
	}

	// Find workflow runs associated with the component if the status is running
	condition := a.getConditionWorkflow(&existingComponent)
	if condition.Reason == conurev1alpha1.ComponentWorkflowRunningReason.String() || condition.Reason == conurev1alpha1.ComponentWorkflowTriggeredReason.String() {
		wflrName := existingComponent.ObjectMeta.Labels[conurev1alpha1.WorkflowRunNamelabel]
		err = a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: wflrName}, &wflr)
		if apierrors.IsNotFound(err) {
			a.Logger.V(1).Info("Workflow run not found", "component", component.Name)
		} else if err != nil {
			return err
		} else {
			if err = a.updateWorkflowRunConditions(&wflr, &existingComponent); err != nil {
				return err
			}
		}
	}
	// Update the component's status in the application
	conditionReady := a.getConditionReady(&existingComponent)
	var reason conurev1alpha1.ComponentConditionReason
	if conditionReady.Reason == "" {
		reason = conurev1alpha1.ComponentReadyPendingReason
	} else {
		reason = conurev1alpha1.ComponentConditionReason(conditionReady.Reason)
	}
	componentStatus := conurev1alpha1.ApplicationComponentStatus{
		ComponentName: existingComponent.Name,
		ComponentType: existingComponent.Spec.ComponentType,
		Reason:        reason,
	}
	for i, comp := range a.Application.Status.Components {
		if comp.ComponentName == existingComponent.Name {
			a.Application.Status.Components[i] = componentStatus
			return nil
		}
	}
	a.Application.Status.Components = append(a.Application.Status.Components, componentStatus)
	return nil
}

func (a *ApplicationHandler) updateWorkflowRunConditions(wflr *conurev1alpha1.WorkflowRun, existingComponent *conurev1alpha1.Component) error {
	index, exists := common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeRunningAction.String())
	if exists {
		if wflr.Status.Conditions[index].Status == metav1.ConditionTrue {
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowRunningReason, fmt.Sprintf("Workflow %s is running", wflr.Name)); err != nil {
				return err
			}
		} else {
			a.Logger.V(1).Info("Workflow failed", "component", existingComponent.Name)
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkFlowFailedReason, fmt.Sprintf("Workflow %s failed", wflr.Name)); err != nil {
				return err
			}
		}
	}
	index, exists = common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeFinished.String())
	if exists {
		if wflr.Status.Conditions[index].Status == metav1.ConditionTrue {
			a.Logger.V(1).Info("Workflow finished", "component", existingComponent.Name)
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkFlowSucceedReason, fmt.Sprintf("Workflow %s finished", wflr.Name)); err != nil {
				return err
			}
			if err := a.setConditionReady(existingComponent, conurev1alpha1.ComponentReadyPendingReason, "Component is pending a deployment"); err != nil {
				return err
			}
		} else {
			a.Logger.V(1).Info("Workflow failed", "component", existingComponent.Name)
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkFlowFailedReason, fmt.Sprintf("Workflow %s failed", wflr.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *ApplicationHandler) createComponent(component *conurev1alpha1.Component) error {
	if err := a.Reconciler.Create(a.Ctx, component); err != nil {
		return err
	}
	if err := a.setComponentWorkflow(component); err != nil {
		return err
	}
	if err := a.setConditionWorkflow(component, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow was triggered"); err != nil {
		return err
	}
	wflr, err := a.runComponentWorkflow(component)
	if apierrors.IsNotFound(err) {
		a.Logger.V(1).Info("Workflow not found", "component", component.Name)
		return a.Reconciler.Update(a.Ctx, component)
	} else if err != nil {
		if err2 := a.setConditionWorkflow(component, metav1.ConditionFalse, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow failed to trigger"); err2 != nil {
			return err2
		}
		return err
	}
	labels := component.GetLabels()
	labels[conurev1alpha1.WorkflowRunNamelabel] = wflr.Name
	component.SetLabels(labels)
	return a.Reconciler.Update(a.Ctx, component)
}

func (a *ApplicationHandler) updateComponent(component *conurev1alpha1.Component, existingComponent *conurev1alpha1.Component, targetHash string) error {
	// Check differences between the existing component and the new component
	specHashActual := common.GetHashForSpec(existingComponent.Spec)
	if specHashActual != targetHash {
		a.Logger.V(1).Info("Updating component", "component", component.Name)
		// Run workflow if the source has changed
		if !reflect.DeepEqual(existingComponent.Spec.Values.Source, component.Spec.Values.Source) {
			if err := a.setRenderingComponentStatus(component.Name); err != nil {
				return err
			}
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow was triggered"); err != nil {
				return err
			}
			wflr, err := a.runComponentWorkflow(component)
			if err != nil {
				if err2 := a.setConditionWorkflow(existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow failed to trigger"); err2 != nil {
					return err2
				}
				if err3 := a.setRenderingComponentFailedStatus(component.Name); err3 != nil {
					return err3
				}
				return err
			}
			labels := existingComponent.GetLabels()
			labels[conurev1alpha1.WorkflowRunNamelabel] = wflr.Name
			existingComponent.SetLabels(labels)
		}
		existingComponent.Spec = *component.Spec.DeepCopy()
		if err := a.Reconciler.Update(a.Ctx, existingComponent); err != nil {
			a.Logger.Error(err, "Unable to update the component for application", "component", component.Name)
			return err
		}
	}
	return nil
}

func (a *ApplicationHandler) setComponentWorkflow(component *conurev1alpha1.Component) error {
	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Spec.Values)
	if err != nil {
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	if err = d.Decode(&values); err != nil {
		return err
	}
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, a.Application.Namespace, "", true, values.Get())
	if err != nil {
		return err
	}
	sets, err := componentTemplate.GetApplySets()
	if err != nil {
		return err
	}
	// find the workflow and apply it
	for _, set := range sets {
		for _, obj := range set.Objects {
			if obj.GetKind() == "Workflow" {
				_, err = componentTemplate.ApplyObject(obj, false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *ApplicationHandler) runComponentWorkflow(component *conurev1alpha1.Component) (*conurev1alpha1.WorkflowRun, error) {
	var wfl conurev1alpha1.Workflow
	if err := a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: component.Name}, &wfl); err != nil {
		return nil, err
	}
	workflowRun := conurev1alpha1.WorkflowRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: component.Name + "-",
			Namespace:    a.Application.Namespace,
			Labels: map[string]string{
				k8sUtils.ApplicationNameLabel: a.Application.Name,
				k8sUtils.ComponentNameLabel:   component.Name,
			},
		},
		Spec: conurev1alpha1.WorkflowRunSpec{
			ApplicationName: a.Application.Name,
			ComponentName:   component.Name,
			WorkflowName:    wfl.Name,
		},
	}
	if err := ctrl.SetControllerReference(a.Application, &workflowRun, a.Reconciler.Scheme); err != nil {
		return nil, err
	}
	if err := a.Reconciler.Create(a.Ctx, &workflowRun); err != nil {
		return nil, err
	}
	return &workflowRun, nil
}

func (a *ApplicationHandler) setConditionWorkflow(component *conurev1alpha1.Component, status metav1.ConditionStatus, reason conurev1alpha1.ComponentConditionReason, message string) error {
	component.Status.Conditions = common.SetCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeWorkflow.String(), status, reason.String(), message)
	return common.ApplyStatus(a.Ctx, component, a.Reconciler.Client)
}

func (a *ApplicationHandler) getConditionWorkflow(component *conurev1alpha1.Component) metav1.Condition {
	index, exists := common.ContainsCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeWorkflow.String())
	if exists {
		return component.Status.Conditions[index]
	}
	return metav1.Condition{}
}

func (a *ApplicationHandler) getConditionReady(component *conurev1alpha1.Component) metav1.Condition {
	index, exists := common.ContainsCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String())
	if exists {
		return component.Status.Conditions[index]
	}
	return metav1.Condition{}
}

func (a *ApplicationHandler) setConditionReady(component *conurev1alpha1.Component, reason conurev1alpha1.ComponentConditionReason, message string) error {
	status := metav1.ConditionFalse
	if reason == conurev1alpha1.ComponentReadyRunningReason {
		status = metav1.ConditionTrue
	}
	component.Status.Conditions = common.SetCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeReady.String(), status, reason.String(), message)
	return common.ApplyStatus(a.Ctx, component, a.Reconciler.Client)
}

func (a *ApplicationHandler) setRenderingComponentStatus(componentName string) error {
	message := fmt.Sprintf("Component %s is being rendered", componentName)
	a.Application.Status.Conditions = common.SetCondition(a.Application.Status.Conditions, conurev1alpha1.ApplicationConditionTypeStatus.String(), metav1.ConditionTrue, conurev1alpha1.ApplicationStatusReasonRendering.String(), message)
	return common.ApplyStatus(a.Ctx, a.Application, a.Reconciler.Client)
}

func (a *ApplicationHandler) setRenderingComponentFailedStatus(componentName string) error {
	message := fmt.Sprintf("Component %s failed to render", componentName)
	a.Application.Status.Conditions = common.SetCondition(a.Application.Status.Conditions, conurev1alpha1.ApplicationConditionTypeStatus.String(), metav1.ConditionFalse, conurev1alpha1.ApplicationStatusReasonRenderingFailed.String(), message)
	return common.ApplyStatus(a.Ctx, a.Application, a.Reconciler.Client)
}

func (a *ApplicationHandler) setDeployedStatus() error {
	a.Application.Status.Conditions = common.SetCondition(a.Application.Status.Conditions, conurev1alpha1.ApplicationConditionTypeStatus.String(), metav1.ConditionTrue, conurev1alpha1.ApplicationStatusReasonDeployed.String(), "Components have been rendered and deployed")
	return common.ApplyStatus(a.Ctx, a.Application, a.Reconciler.Client)
}
