package application

import (
	"testing"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func waitForResource(t *testing.T, name, namespace string, obj *conurev1alpha1.Application) {
	t.Helper()
	key := types.NamespacedName{Name: name, Namespace: namespace}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := k8sClient.Get(ctx, key, obj); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for Application %s/%s", namespace, name)
}

func TestReconcile_CreatesAndPersistsApplication(t *testing.T) {
	app := &conurev1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app-reconcile",
			Namespace: "default",
		},
	}

	if err := k8sClient.Create(ctx, app); err != nil {
		t.Fatalf("failed to create Application: %v", err)
	}

	fetched := &conurev1alpha1.Application{}
	waitForResource(t, "test-app-reconcile", "default", fetched)

	if fetched.Name != "test-app-reconcile" {
		t.Fatalf("expected name test-app-reconcile, got %s", fetched.Name)
	}
}

func TestReconcile_MissingApplicationReturnsNotFound(t *testing.T) {
	fetched := &conurev1alpha1.Application{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "does-not-exist",
		Namespace: "default",
	}, fetched)
	if err == nil {
		t.Fatal("expected error for non-existent Application")
	}
}
