package component

import (
	"testing"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func waitForConditionReason(t *testing.T, name, namespace, conditionType, expectedReason string) {
	t.Helper()
	key := types.NamespacedName{Name: name, Namespace: namespace}
	deadline := time.Now().Add(10 * time.Second)
	var lastReason string
	for time.Now().Before(deadline) {
		fetched := &conurev1alpha1.Component{}
		if err := k8sClient.Get(ctx, key, fetched); err == nil {
			idx, exists := common.ContainsCondition(fetched.Status.Conditions, conditionType)
			if exists {
				lastReason = fetched.Status.Conditions[idx].Reason
				if lastReason == expectedReason {
					return
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for condition %s reason %s on Component %s/%s (last seen: %s)", conditionType, expectedReason, namespace, name, lastReason)
}

func TestReconcile_NoComponentDefinition_SetsRenderingFailed(t *testing.T) {
	comp := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-comp-no-def",
			Namespace: "default",
		},
		Spec: conurev1alpha1.ComponentSpec{
			ComponentType: "nonexistent-type",
			Values:        &runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	if err := k8sClient.Create(ctx, comp); err != nil {
		t.Fatalf("failed to create Component: %v", err)
	}

	waitForConditionReason(t, "test-comp-no-def", "default",
		conurev1alpha1.ComponentConditionTypeReady.String(),
		conurev1alpha1.ComponentReadyRenderingFailedReason.String(),
	)
}

func TestReconcile_ComponentIsPersisted(t *testing.T) {
	comp := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-comp-basic",
			Namespace: "default",
		},
		Spec: conurev1alpha1.ComponentSpec{
			ComponentType: "webservice",
			Values:        &runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	if err := k8sClient.Create(ctx, comp); err != nil {
		t.Fatalf("failed to create Component: %v", err)
	}

	fetched := &conurev1alpha1.Component{}
	key := types.NamespacedName{Name: "test-comp-basic", Namespace: "default"}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, key, fetched); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if fetched.Spec.ComponentType != "webservice" {
		t.Fatalf("expected ComponentType webservice, got %s", fetched.Spec.ComponentType)
	}
}
