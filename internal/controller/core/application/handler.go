package application

import (
	"context"
	"fmt"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
