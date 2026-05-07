package application

import (
	"context"
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = conurev1alpha1.AddToScheme(s)
	return s
}

func newFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithStatusSubresource(&conurev1alpha1.Application{}, &conurev1alpha1.Component{}).
		WithObjects(objs...).
		Build()
}

func newTestApplication(name, namespace string) *conurev1alpha1.Application {
	return &conurev1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newTestComponent(name, namespace, componentType string) *conurev1alpha1.Component {
	return &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: conurev1alpha1.ComponentSpec{
			ComponentType: componentType,
		},
	}
}

func newTestHandler(ctx context.Context, k8sClient client.Client, app *conurev1alpha1.Application) *ApplicationHandler {
	reconciler := &ApplicationReconciler{
		Client: k8sClient,
		Scheme: newTestScheme(),
	}
	return &ApplicationHandler{
		Application: app,
		Reconciler:  reconciler,
		Ctx:         ctx,
		Logger:      log.FromContext(ctx),
	}
}

func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

func TestSetRenderingComponentStatus(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	handler := newTestHandler(ctx, k8sClient, app)

	err := handler.setRenderingComponentStatus("web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(app.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(app.Status.Conditions))
	}
	c := app.Status.Conditions[0]
	if c.Type != conurev1alpha1.ApplicationConditionTypeStatus.String() {
		t.Fatalf("expected type Status, got %s", c.Type)
	}
	if c.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue, got %s", c.Status)
	}
	if c.Reason != conurev1alpha1.ApplicationStatusReasonRendering.String() {
		t.Fatalf("expected reason RenderingComponent, got %s", c.Reason)
	}
}

func TestSetRenderingComponentFailedStatus(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	handler := newTestHandler(ctx, k8sClient, app)

	err := handler.setRenderingComponentFailedStatus("web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := app.Status.Conditions[0]
	if c.Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse, got %s", c.Status)
	}
	if c.Reason != conurev1alpha1.ApplicationStatusReasonRenderingFailed.String() {
		t.Fatalf("expected reason RenderingComponentFailed, got %s", c.Reason)
	}
}

func TestSetDeployedStatus(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	handler := newTestHandler(ctx, k8sClient, app)

	err := handler.setDeployedStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := app.Status.Conditions[0]
	if c.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue, got %s", c.Status)
	}
	if c.Reason != conurev1alpha1.ApplicationStatusReasonDeployed.String() {
		t.Fatalf("expected reason Deployed, got %s", c.Reason)
	}
}

func TestStatusTransition_RenderingToDeployed(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	handler := newTestHandler(ctx, k8sClient, app)

	// Simulate: rendering → deployed
	if err := handler.setRenderingComponentStatus("web"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := handler.setDeployedStatus(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still be 1 condition (updated in place)
	if len(app.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(app.Status.Conditions))
	}
	if app.Status.Conditions[0].Reason != conurev1alpha1.ApplicationStatusReasonDeployed.String() {
		t.Fatalf("expected Deployed, got %s", app.Status.Conditions[0].Reason)
	}
}

func TestStatusTransition_RenderingToFailed(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	handler := newTestHandler(ctx, k8sClient, app)

	if err := handler.setRenderingComponentStatus("web"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := handler.setRenderingComponentFailedStatus("web"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(app.Status.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(app.Status.Conditions))
	}
	if app.Status.Conditions[0].Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse after failure, got %s", app.Status.Conditions[0].Status)
	}
}

func TestGetConditionReady_Component(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	comp := newTestComponent("web", "default", "webservice")
	comp.Status.Conditions = []metav1.Condition{
		{
			Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
			Status: metav1.ConditionTrue,
			Reason: conurev1alpha1.ComponentReadyRunningReason.String(),
		},
	}
	k8sClient := newFakeClient(app, comp)
	handler := newTestHandler(ctx, k8sClient, app)

	condition := handler.getConditionReady(comp)
	if condition.Reason != conurev1alpha1.ComponentReadyRunningReason.String() {
		t.Fatalf("expected Running, got %s", condition.Reason)
	}
}

func TestGetConditionReady_ComponentNoCondition(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(app, comp)
	handler := newTestHandler(ctx, k8sClient, app)

	condition := handler.getConditionReady(comp)
	if condition.Reason != "" {
		t.Fatalf("expected empty reason for missing condition, got %s", condition.Reason)
	}
}

func TestSetConditionReady_Component_Running(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(app, comp)
	handler := newTestHandler(ctx, k8sClient, app)

	err := handler.setConditionReady(comp, conurev1alpha1.ComponentReadyRunningReason, "running")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	condition := handler.getConditionReady(comp)
	if condition.Status != metav1.ConditionTrue {
		t.Fatalf("expected ConditionTrue for Running, got %s", condition.Status)
	}
}

func TestSetConditionReady_Component_Pending(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(app, comp)
	handler := newTestHandler(ctx, k8sClient, app)

	err := handler.setConditionReady(comp, conurev1alpha1.ComponentReadyPendingReason, "waiting")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	condition := handler.getConditionReady(comp)
	if condition.Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse for Pending, got %s", condition.Status)
	}
}

func TestNewApplicationHandler(t *testing.T) {
	ctx := context.Background()
	app := newTestApplication("myapp", "default")
	k8sClient := newFakeClient(app)
	reconciler := &ApplicationReconciler{
		Client: k8sClient,
		Scheme: newTestScheme(),
	}

	handler, err := NewApplicationHandler(ctx, app, reconciler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.Application != app {
		t.Fatal("expected handler to reference the application")
	}
	if handler.Reconciler != reconciler {
		t.Fatal("expected handler to reference the reconciler")
	}
}
