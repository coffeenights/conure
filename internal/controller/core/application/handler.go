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
	return nil
}

func (a *ApplicationHandler) ReconcileComponent(componentTemp *conurev1alpha1.ComponentTemplate) error {
	a.Logger.Info("Reconciling component", "component", componentTemp.Name)
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
	err := ctrl.SetControllerReference(a.Application, &component, a.Reconciler.Scheme)
	if err != nil {
		return err
	}

	// Find an existing component
	err = a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: component.Name}, &existingComponent)

	// If the component does not exist, create it and run its workflow
	if apierrors.IsNotFound(err) {
		a.Logger.Info("Creating component", "component", component.Name)
		if err = a.createComponent(&component); err != nil {
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

	// Find workflow runs associated with the component
	wflrName := existingComponent.ObjectMeta.Labels[conurev1alpha1.WorkflowRunNamelabel]
	err = a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: wflrName}, &wflr)
	if err != nil {
		return err
	}

	index, exists := common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeRunningAction.String())
	if exists {
		if wflr.Status.Conditions[index].Status == metav1.ConditionTrue {
			a.Logger.Info("Workflow is running, skipping the component", "component", component.Name)
			if err = a.setConditionWorkflow(&existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowRunningReason, fmt.Sprintf("Workflow %s is running", wflr.Name)); err != nil {
				return err
			}
		} else {
			a.Logger.Info("Workflow failed", "component", component.Name)
			if err = a.setConditionWorkflow(&existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkFlowFailedReason, fmt.Sprintf("Workflow %s failed", wflr.Name)); err != nil {
				return err
			}
		}
	}
	index, exists = common.ContainsCondition(wflr.Status.Conditions, conurev1alpha1.ConditionTypeFinished.String())
	if exists {
		if wflr.Status.Conditions[index].Status == metav1.ConditionTrue {
			a.Logger.Info("Workflow finished", "component", component.Name)
			if err = a.setConditionWorkflow(&existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkFlowSucceedReason, fmt.Sprintf("Workflow %s finished", wflr.Name)); err != nil {
				return err
			}
		} else {
			a.Logger.Info("Workflow failed", "component", component.Name)
			if err = a.setConditionWorkflow(&existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkFlowFailedReason, fmt.Sprintf("Workflow %s failed", wflr.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *ApplicationHandler) createComponent(component *conurev1alpha1.Component) error {
	err := a.Reconciler.Create(a.Ctx, component)
	if err != nil {
		return err
	}
	if err = a.setComponentWorkflow(component); err != nil {
		return err
	}
	if err = a.setConditionWorkflow(component, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow was triggered"); err != nil {
		return err
	}
	wflr, err := a.runComponentWorkflow(component)
	if apierrors.IsNotFound(err) {
		a.Logger.Info("Workflow run not found", "component", component.Name)
	} else if err != nil {
		if err = a.setConditionWorkflow(component, metav1.ConditionFalse, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow failed to trigger"); err != nil {
			return err
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
		a.Logger.Info("Updating component", "component", component.Name)
		// Run workflow if the source has changed
		if !reflect.DeepEqual(existingComponent.Spec.Values.Source, component.Spec.Values.Source) {
			if err := a.setConditionWorkflow(existingComponent, metav1.ConditionTrue, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow was triggered"); err != nil {
				return err
			}
			wflr, err := a.runComponentWorkflow(component)
			if err != nil {
				if err = a.setConditionWorkflow(existingComponent, metav1.ConditionFalse, conurev1alpha1.ComponentWorkflowTriggeredReason, "Workflow failed to trigger"); err != nil {
					return err
				}
				return err
			}
			labels := component.GetLabels()
			labels[conurev1alpha1.WorkflowRunNamelabel] = wflr.Name
			component.SetLabels(labels)
		}
		err := a.Reconciler.Status().Update(a.Ctx, existingComponent)
		if err != nil {
			return err
		}
		existingComponent.Spec = *component.Spec.DeepCopy()
		err = a.Reconciler.Update(a.Ctx, existingComponent)
		if err != nil {
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
	err = d.Decode(&values)
	if err != nil {
		return err
	}
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, a.Application.Namespace, "", values.Get())
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
	err := a.Reconciler.Get(a.Ctx, client.ObjectKey{Namespace: a.Application.Namespace, Name: component.Name}, &wfl)
	// If there is no workflow present, simply ignore the error
	if err != nil {
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
	err = a.Reconciler.Create(a.Ctx, &workflowRun)
	if err != nil {
		return nil, err
	}
	return &workflowRun, nil
}

func (a *ApplicationHandler) setConditionWorkflow(component *conurev1alpha1.Component, status metav1.ConditionStatus, reason conurev1alpha1.ComponentConditionReason, message string) error {
	component.Status.Conditions = common.SetCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeWorkflow.String(), status, reason.String(), message)
	return a.Reconciler.Status().Update(a.Ctx, component)
}

func (a *ApplicationHandler) getConditionWorkflow(component *conurev1alpha1.Component) metav1.Condition {
	index, exists := common.ContainsCondition(component.Status.Conditions, conurev1alpha1.ComponentConditionTypeWorkflow.String())
	if exists {
		return component.Status.Conditions[index]
	}
	return metav1.Condition{}
}

func (a *ApplicationHandler) ReconcileComponentOld(component *conurev1alpha1.ComponentTemplate) error {
	logger := log.FromContext(a.Ctx)
	logger.Info("Reconciling component", "component", component.Name)

	// Transform the values to a map
	valuesJSON, err := json.Marshal(component.Spec.Values)
	if err != nil {
		return err
	}
	values := timoni.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	// Turn numbers into strings, otherwise the decoder will take ints and turn them into floats
	d.UseNumber()
	err = d.Decode(&values)
	if err != nil {
		return err
	}
	componentTemplate, err := module.NewManager(a.Ctx, component.Name, component.Spec.OCIRepository, component.Spec.OCITag, a.Application.Namespace, "", values.Get())
	if err != nil {
		return err
	}
	_, err = componentTemplate.Build()
	if err != nil {
		return err
	}
	err = componentTemplate.Apply()
	if err != nil {
		return err
	}
	return nil
}
