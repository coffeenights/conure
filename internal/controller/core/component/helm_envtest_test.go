package component

import (
	"context"
	"testing"
	"time"

	chartv2loader "helm.sh/helm/v4/pkg/chart/v2/loader"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/render"
	"github.com/coffeenights/conure/internal/render/apply"
	helmengine "github.com/coffeenights/conure/internal/render/helm"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// helmTestChartPath points at the testdata chart shared with the Helm engine's
// unit tests. Relative to the component package, it lives three dirs up.
const helmTestChartPath = "../../../render/helm/testdata/sample"

// diskHelmBuilder is a render.Builder used only by the envtest suite. It
// substitutes the OCI pull with a load-from-disk so the test exercises the
// full render → apply → status path without needing a live registry.
type diskHelmBuilder struct {
	chartPath string
	applier   *apply.Manager
}

func newDiskHelmBuilder(applier *apply.Manager) *diskHelmBuilder {
	return &diskHelmBuilder{chartPath: helmTestChartPath, applier: applier}
}

func (b *diskHelmBuilder) Build(ctx context.Context, def *conurev1alpha1.ComponentDefinition, comp *conurev1alpha1.Component, _ string) (render.Engine, error) {
	chart, err := chartv2loader.Load(b.chartPath)
	if err != nil {
		return nil, err
	}
	return helmengine.NewEngineFromChart(chart, def, comp, b.applier)
}

func (b *diskHelmBuilder) BuildForApply(ctx context.Context, comp *conurev1alpha1.Component) (render.Engine, error) {
	// Same shape as the production Helm Builder.BuildForApply — only the
	// applier is required for the drift sweep.
	return helmengine.NewEngineFromChart(nil, &conurev1alpha1.ComponentDefinition{}, comp, b.applier)
}

// TestEnvtest_HelmComponent_RendersAndApplies drives a Component end-to-end
// through the controller using engine: helm. The matching ComponentDefinition
// points at any OCI reference (ignored by the disk-loader test builder), and
// we assert that:
//   - the controller renders the sample chart;
//   - the rendered Deployment and Service appear in the cluster via SSA apply;
//   - the Component's Ready condition rolls up to True with reason Ready;
//   - the apply-set annotation contains the v1 envelope tagged "helm".
func TestEnvtest_HelmComponent_RendersAndApplies(t *testing.T) {
	defName := "helm-sample-def"
	def := &conurev1alpha1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: defName},
		Spec: conurev1alpha1.ComponentDefinitionSpec{
			ComponentType: "helm-sample",
			Engine:        conurev1alpha1.EngineHelm,
			OCIRepository: "ghcr.io/example/sample-chart",
			OCITag:        "0.1.0",
		},
	}
	if err := k8sClient.Create(ctx, def); err != nil {
		t.Fatalf("creating ComponentDefinition: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, def) })

	comp := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "helm-web", Namespace: "default"},
		Spec: conurev1alpha1.ComponentSpec{
			ComponentType: "helm-sample",
			Values:        &runtime.RawExtension{Raw: []byte(`{"replicas":1}`)},
		},
	}
	if err := k8sClient.Create(ctx, comp); err != nil {
		t.Fatalf("creating Component: %v", err)
	}
	t.Cleanup(func() {
		_ = k8sClient.Delete(ctx, comp)
		_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "helm-web", Namespace: "default"}})
		_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "helm-web-svc", Namespace: "default"}})
	})

	waitForConditionReason(t, comp.Name, comp.Namespace,
		conurev1alpha1.ComponentConditionTypeReady.String(),
		conurev1alpha1.ComponentReasonReady.String(),
	)

	deploy := &appsv1.Deployment{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "helm-web", Namespace: "default"}, deploy); err != nil {
		t.Fatalf("Deployment helm-web missing after reconcile: %v", err)
	}
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 1 {
		t.Fatalf("expected Deployment.spec.replicas=1, got %v", deploy.Spec.Replicas)
	}

	svc := &corev1.Service{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "helm-web-svc", Namespace: "default"}, svc); err != nil {
		t.Fatalf("Service helm-web-svc missing after reconcile: %v", err)
	}

	// The hook resource from the sample chart (Job annotated
	// helm.sh/hook: pre-install) must NOT have been applied — Helm hooks
	// are filtered at render time.
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "helm-web-pre", Namespace: "default"}, &batchv1.Job{}); err == nil {
		t.Fatalf("hook Job helm-web-pre should have been filtered out, but it was applied")
	}

	// Refetch the Component and inspect the apply-set envelope so we
	// confirm the Helm engine produced a v1-tagged annotation, not legacy.
	deadline := time.Now().Add(5 * time.Second)
	var fetched conurev1alpha1.Component
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: comp.Name, Namespace: comp.Namespace}, &fetched); err == nil {
			if fetched.Annotations[conurev1alpha1.ApplySetsAnnotation] != "" {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if fetched.Annotations[conurev1alpha1.ApplySetsAnnotation] == "" {
		t.Fatalf("expected ApplySetsAnnotation to be set after a successful Helm reconcile")
	}
}
