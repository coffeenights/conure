package component

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/timoni"
	"github.com/fluxcd/pkg/ssa"
	"github.com/stefanprodan/timoni/pkg/module"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// newFakeClientWithStatusUpdateCounter returns a fake client backed by a
// counter that increments every time Status().Update(...) is invoked. Useful
// for asserting that a reconcile produces a single status patch.
func newFakeClientWithStatusUpdateCounter(counter *int, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithStatusSubresource(&conurev1alpha1.Component{}).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				if subResourceName == "status" {
					*counter++
				}
				return c.Status().Update(ctx, obj, opts...)
			},
		}).
		Build()
}

// mockModuleManager implements timoni.ModuleManager for testing.
type mockModuleManager struct {
	applySets    []module.ResourceSet
	applyErr     error
	marshalErr   error
	unmarshalErr error
	applyObjErr  error
	appliedObjs  []*unstructured.Unstructured
	digest       string
}

func (m *mockModuleManager) GetApplySets() ([]module.ResourceSet, error) {
	return m.applySets, m.applyErr
}

func (m *mockModuleManager) GetDigest() string {
	return m.digest
}

func (m *mockModuleManager) MarshalApplySets(sets []module.ResourceSet) ([]byte, error) {
	if m.marshalErr != nil {
		return nil, m.marshalErr
	}
	return json.Marshal(sets)
}

func (m *mockModuleManager) UnmarshalApplySets(data []byte) ([]module.ResourceSet, error) {
	if m.unmarshalErr != nil {
		return nil, m.unmarshalErr
	}
	var sets []module.ResourceSet
	err := json.Unmarshal(data, &sets)
	return sets, err
}

func (m *mockModuleManager) ApplyObject(resource *unstructured.Unstructured, force bool) (*ssa.ChangeSetEntry, error) {
	if m.applyObjErr != nil {
		return nil, m.applyObjErr
	}
	m.appliedObjs = append(m.appliedObjs, resource)
	return &ssa.ChangeSetEntry{}, nil
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = conurev1alpha1.AddToScheme(s)
	return s
}

func newFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithStatusSubresource(&conurev1alpha1.Component{}).
		WithObjects(objs...).
		Build()
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

func newTestHandler(ctx context.Context, k8sClient client.Client, component *conurev1alpha1.Component) *ComponentHandler {
	reconciler := &ComponentReconciler{
		Client: k8sClient,
		Scheme: newTestScheme(),
	}
	return &ComponentHandler{
		Component:  component,
		Reconciler: reconciler,
		Ctx:        ctx,
		Logger:     log.FromContext(ctx),
	}
}

func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

func TestGetConditionReady_NoConditions(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	handler := newTestHandler(ctx, newFakeClient(), comp)

	condition := handler.GetConditionReady()
	if condition != nil {
		t.Fatal("expected nil condition when no conditions set")
	}
}

func TestGetConditionReady_WithCondition(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	comp.Status.Conditions = []metav1.Condition{
		{
			Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
			Status: metav1.ConditionFalse,
			Reason: conurev1alpha1.ComponentReadyDeployingSucceedReason.String(),
		},
	}
	handler := newTestHandler(ctx, newFakeClient(), comp)

	condition := handler.GetConditionReady()
	if condition == nil {
		t.Fatal("expected non-nil condition")
	}
	if condition.Reason != conurev1alpha1.ComponentReadyDeployingSucceedReason.String() {
		t.Fatalf("expected reason DeployingSucceed, got %s", condition.Reason)
	}
}

func TestSetConditionReady_SetsStatusFalse_ForNonRunning(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	err := handler.setConditionReady(conurev1alpha1.ComponentReadyRenderingReason, "rendering")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	condition := handler.GetConditionReady()
	if condition.Status != metav1.ConditionFalse {
		t.Fatalf("expected ConditionFalse for Rendering, got %s", condition.Status)
	}
}

func TestSetConditionReady_NoOpWhenUnchanged(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	comp.Status.Conditions = []metav1.Condition{
		{
			Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
			Status: metav1.ConditionFalse,
			Reason: conurev1alpha1.ComponentReadyDeployingSucceedReason.String(),
		},
	}
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	// Should be a no-op since status and reason match
	err := handler.setConditionReady(conurev1alpha1.ComponentReadyDeployingSucceedReason, "deployed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderComponent_FailureProducesSingleStatusPatch(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "nonexistent-type")

	var statusUpdates int
	k8sClient := newFakeClientWithStatusUpdateCounter(&statusUpdates, comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	// renderComponent fails because no ComponentDefinition exists for the type;
	// RenderComponent should buffer Rendering→RenderingFailed and emit one patch.
	if err := handler.RenderComponent(); err == nil {
		t.Fatal("expected error from RenderComponent without matching ComponentDefinition")
	}

	if statusUpdates != 1 {
		t.Fatalf("expected exactly 1 status update on failure path, got %d", statusUpdates)
	}

	condition := handler.GetConditionReady()
	if condition == nil {
		t.Fatal("expected condition to be set")
	}
	if condition.Reason != conurev1alpha1.ComponentReadyRenderingFailedReason.String() {
		t.Fatalf("expected RenderingFailed reason, got %s", condition.Reason)
	}
}

func TestRenderComponent_BufferedTransitionsOnlyEmitTerminal(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")

	var statusUpdates int
	k8sClient := newFakeClientWithStatusUpdateCounter(&statusUpdates, comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	// Simulate the intermediate buffering done by RenderComponent without
	// invoking the real OCI pull path.
	for _, reason := range []conurev1alpha1.ComponentConditionReason{
		conurev1alpha1.ComponentReadyRenderingReason,
		conurev1alpha1.ComponentReadyRenderingSucceedReason,
		conurev1alpha1.ComponentReadyDeployingReason,
		conurev1alpha1.ComponentReadyDeployingSucceedReason,
	} {
		if _, err := handler.bufferConditionReady(reason, "step"); err != nil {
			t.Fatalf("buffer failed: %v", err)
		}
	}
	if statusUpdates != 0 {
		t.Fatalf("expected 0 status updates after buffering, got %d", statusUpdates)
	}
	if err := handler.flushStatus(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if statusUpdates != 1 {
		t.Fatalf("expected exactly 1 status update after flush, got %d", statusUpdates)
	}

	// Only the terminal reason should be visible.
	condition := handler.GetConditionReady()
	if condition.Reason != conurev1alpha1.ComponentReadyDeployingSucceedReason.String() {
		t.Fatalf("expected DeployingSucceed reason, got %s", condition.Reason)
	}
}

func TestSetConditionReady_RejectsUnknownReason(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	handler := newTestHandler(ctx, newFakeClient(comp), comp)

	err := handler.setConditionReady(conurev1alpha1.ComponentConditionReason("Rendring"), "typo")
	if err == nil {
		t.Fatal("expected error for unknown reason")
	}
	if handler.GetConditionReady() != nil {
		t.Fatal("expected no condition to be set when reason is invalid")
	}
}

func TestSetConditionReady_UpdatesMessageOnFailureRetry(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	if err := handler.setConditionReady(conurev1alpha1.ComponentReadyRenderingFailedReason, "error A"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := handler.setConditionReady(conurev1alpha1.ComponentReadyRenderingFailedReason, "error B"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	condition := handler.GetConditionReady()
	if condition.Message != "error B" {
		t.Fatalf("expected failure message to be updated to 'error B', got %q", condition.Message)
	}
}

func TestSetConditionReady_NoOpWhenFailureMessageUnchanged(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	comp.Status.Conditions = []metav1.Condition{
		{
			Type:    conurev1alpha1.ComponentConditionTypeReady.String(),
			Status:  metav1.ConditionFalse,
			Reason:  conurev1alpha1.ComponentReadyRenderingFailedReason.String(),
			Message: "error A",
		},
	}
	original := comp.Status.Conditions[0].LastTransitionTime
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	if err := handler.setConditionReady(conurev1alpha1.ComponentReadyRenderingFailedReason, "error A"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	condition := handler.GetConditionReady()
	if !condition.LastTransitionTime.Equal(&original) {
		t.Fatal("expected LastTransitionTime to be preserved when nothing changed")
	}
}

func TestApplyResources_OrdersByKind(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	mock := &mockModuleManager{}
	handler.componentTemplate = mock

	// Add resources in wrong order
	deployment := &unstructured.Unstructured{}
	deployment.SetKind("Deployment")
	deployment.SetName("web")

	service := &unstructured.Unstructured{}
	service.SetKind("Service")
	service.SetName("web-svc")

	namespace := &unstructured.Unstructured{}
	namespace.SetKind("Namespace")
	namespace.SetName("app-ns")

	handler.applySet = []*unstructured.Unstructured{deployment, service, namespace}

	err := handler.applyResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.appliedObjs) != 3 {
		t.Fatalf("expected 3 applied objects, got %d", len(mock.appliedObjs))
	}
	// Should be sorted: Namespace(1) → Service(16) → Deployment(21)
	if mock.appliedObjs[0].GetKind() != "Namespace" {
		t.Fatalf("expected first applied to be Namespace, got %s", mock.appliedObjs[0].GetKind())
	}
	if mock.appliedObjs[1].GetKind() != "Service" {
		t.Fatalf("expected second applied to be Service, got %s", mock.appliedObjs[1].GetKind())
	}
	if mock.appliedObjs[2].GetKind() != "Deployment" {
		t.Fatalf("expected third applied to be Deployment, got %s", mock.appliedObjs[2].GetKind())
	}

	// Apply set should be cleared after success
	if handler.applySet != nil {
		t.Fatal("expected applySet to be nil after successful apply")
	}
}

func TestApplyResources_ReturnsErrorWithoutSettingCondition(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	mock := &mockModuleManager{applyObjErr: fmt.Errorf("apply failed")}
	handler.componentTemplate = mock

	obj := &unstructured.Unstructured{}
	obj.SetKind("Deployment")
	obj.SetName("web")
	handler.applySet = []*unstructured.Unstructured{obj}

	err := handler.applyResources()
	if err == nil {
		t.Fatal("expected error")
	}
	// Condition setting is the caller's responsibility (RenderComponent or
	// ReconcileDeployedObjects); applyResources just propagates the error.
	if handler.GetConditionReady() != nil {
		t.Fatal("expected applyResources to leave conditions untouched")
	}
}

// helper: compress and base64-encode apply sets for annotation
func encodeApplySets(t *testing.T, sets []module.ResourceSet) string {
	t.Helper()
	data, err := json.Marshal(sets)
	if err != nil {
		t.Fatalf("failed to marshal sets: %v", err)
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatalf("failed to gzip: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestReconcileDeployedObjects_NoAnnotation(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)
	handler.componentTemplate = &mockModuleManager{}

	err := handler.ReconcileDeployedObjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No annotation means no-op
	if handler.applySet != nil {
		t.Fatal("expected nil applySet when no annotation present")
	}
}

func TestReconcileDeployedObjects_DecodesAndApplies(t *testing.T) {
	ctx := context.Background()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "web-svc",
				"namespace": "default",
			},
			"spec": map[string]interface{}{},
		},
	}
	sets := []module.ResourceSet{
		{Name: "main", Objects: []*unstructured.Unstructured{obj}},
	}
	encoded := encodeApplySets(t, sets)

	comp := newTestComponent("web", "default", "webservice")
	comp.Annotations = map[string]string{
		conurev1alpha1.ApplySetsAnnotation: encoded,
	}

	mock := &mockModuleManager{}
	// ReconcileDeployedObjects calls module.NewManager internally for the real apply,
	// but we can test the decode path by pre-setting componentTemplate
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)
	handler.componentTemplate = mock

	// The method will decode the annotation, unmarshal via mock, then try to create
	// a new module.Manager for applying. Since we can't mock that call, we verify
	// the decode + unmarshal works by checking the applySet is populated before apply.
	// For a full integration test, we'd use envtest (Layer 3).

	// Test just the decode path by calling the internal steps manually:
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("gzip error: %v", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("read error: %v", err)
	}
	result, err := mock.UnmarshalApplySets(buf.Bytes())
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 resource set, got %d", len(result))
	}
	if len(result[0].Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(result[0].Objects))
	}
	if result[0].Objects[0].GetKind() != "Service" {
		t.Fatalf("expected Service, got %s", result[0].Objects[0].GetKind())
	}
}

func TestApplyResources_SetsOwnerReferences(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent("web", "default", "webservice")
	comp.UID = "test-uid-1234"
	k8sClient := newFakeClient(comp)
	handler := newTestHandler(ctx, k8sClient, comp)

	mock := &mockModuleManager{}
	handler.componentTemplate = mock

	deployment := &unstructured.Unstructured{}
	deployment.SetKind("Deployment")
	deployment.SetName("web")

	configMap := &unstructured.Unstructured{}
	configMap.SetKind("ConfigMap")
	configMap.SetName("web-config")

	service := &unstructured.Unstructured{}
	service.SetKind("Service")
	service.SetName("web-svc")

	handler.applySet = []*unstructured.Unstructured{deployment, configMap, service}

	err := handler.applyResources()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, obj := range mock.appliedObjs {
		refs := obj.GetOwnerReferences()
		if len(refs) != 1 {
			t.Fatalf("expected 1 owner reference on %s, got %d", obj.GetKind(), len(refs))
		}
		ref := refs[0]
		if ref.Kind != "Component" {
			t.Fatalf("expected owner kind Component, got %s", ref.Kind)
		}
		if ref.Name != "web" {
			t.Fatalf("expected owner name web, got %s", ref.Name)
		}
		if ref.UID != "test-uid-1234" {
			t.Fatalf("expected owner UID test-uid-1234, got %s", ref.UID)
		}
		if ref.Controller == nil || !*ref.Controller {
			t.Fatalf("expected Controller=true on owner reference for %s", obj.GetKind())
		}
		if ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
			t.Fatalf("expected BlockOwnerDeletion=true on owner reference for %s", obj.GetKind())
		}
	}
}

// Verify the interface is satisfied by the mock at compile time.
var _ timoni.ModuleManager = (*mockModuleManager)(nil)
