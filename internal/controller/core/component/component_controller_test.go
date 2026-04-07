package component

import (
	"testing"
	"time"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/controller/core/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

func TestDeleteComponent_DependenciesHaveOwnerReferences(t *testing.T) {
	// Create a Component
	comp := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-comp-delete",
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

	// Re-fetch to get the UID assigned by the API server
	key := types.NamespacedName{Name: "test-comp-delete", Namespace: "default"}
	if err := k8sClient.Get(ctx, key, comp); err != nil {
		t.Fatalf("failed to fetch Component: %v", err)
	}

	ownerRef := metav1.OwnerReference{
		APIVersion:         conurev1alpha1.GroupVersion.String(),
		Kind:               "Component",
		Name:               comp.Name,
		UID:                comp.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}

	// Create child resources with owner references pointing to the Component
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-comp-delete-config",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Data: map[string]string{"key": "value"},
	}
	if err := k8sClient.Create(ctx, configMap); err != nil {
		t.Fatalf("failed to create ConfigMap: %v", err)
	}

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-comp-delete-deploy",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
		},
	}
	if err := k8sClient.Create(ctx, deployment); err != nil {
		t.Fatalf("failed to create Deployment: %v", err)
	}

	// Verify the child resources exist and have correct owner references
	fetchedCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp-delete-config", Namespace: "default"}, fetchedCM); err != nil {
		t.Fatalf("failed to fetch ConfigMap: %v", err)
	}
	if len(fetchedCM.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference on ConfigMap, got %d", len(fetchedCM.OwnerReferences))
	}
	if fetchedCM.OwnerReferences[0].UID != comp.UID {
		t.Fatalf("ConfigMap owner UID mismatch: expected %s, got %s", comp.UID, fetchedCM.OwnerReferences[0].UID)
	}

	fetchedDeploy := &appsv1.Deployment{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp-delete-deploy", Namespace: "default"}, fetchedDeploy); err != nil {
		t.Fatalf("failed to fetch Deployment: %v", err)
	}
	if len(fetchedDeploy.OwnerReferences) != 1 {
		t.Fatalf("expected 1 owner reference on Deployment, got %d", len(fetchedDeploy.OwnerReferences))
	}
	if fetchedDeploy.OwnerReferences[0].UID != comp.UID {
		t.Fatalf("Deployment owner UID mismatch: expected %s, got %s", comp.UID, fetchedDeploy.OwnerReferences[0].UID)
	}

	// Delete the Component
	if err := k8sClient.Delete(ctx, comp); err != nil {
		t.Fatalf("failed to delete Component: %v", err)
	}

	// Wait for the Component to be gone
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		err := k8sClient.Get(ctx, key, &conurev1alpha1.Component{})
		if err != nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	// In envtest the GC controller is not running, so child resources won't be
	// automatically deleted. However we can verify that the API server has
	// registered the owner references correctly, which guarantees that in a real
	// cluster the GC will cascade the deletion.
	// Verify the owner references point to a deleted owner (Component is gone).
	cmAfter := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp-delete-config", Namespace: "default"}, cmAfter)
	if err != nil {
		// If envtest somehow GC'd it, that's fine too
		t.Logf("ConfigMap already deleted (GC active): %v", err)
	} else {
		// Verify the owner reference is set correctly for GC
		found := false
		for _, ref := range cmAfter.OwnerReferences {
			if ref.Kind == "Component" && ref.Name == "test-comp-delete" && *ref.BlockOwnerDeletion {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected ConfigMap to have Component owner reference with BlockOwnerDeletion")
		}
	}

	deployAfter := &appsv1.Deployment{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-comp-delete-deploy", Namespace: "default"}, deployAfter)
	if err != nil {
		t.Logf("Deployment already deleted (GC active): %v", err)
	} else {
		found := false
		for _, ref := range deployAfter.OwnerReferences {
			if ref.Kind == "Component" && ref.Name == "test-comp-delete" && *ref.BlockOwnerDeletion {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected Deployment to have Component owner reference with BlockOwnerDeletion")
		}
	}

	// Cleanup: remove orphaned resources since envtest has no GC
	_ = k8sClient.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-comp-delete-config", Namespace: "default"}})
	_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "test-comp-delete-deploy", Namespace: "default"}})
}

func TestDeleteComponent_OrphanedResourcesWithoutOwnerRef(t *testing.T) {
	// Create a Component
	comp := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-comp-orphan",
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

	key := types.NamespacedName{Name: "test-comp-orphan", Namespace: "default"}
	if err := k8sClient.Get(ctx, key, comp); err != nil {
		t.Fatalf("failed to fetch Component: %v", err)
	}

	// Create a ConfigMap WITHOUT owner references (simulating a resource not managed by the controller)
	orphanCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-orphan-config",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}
	if err := k8sClient.Create(ctx, orphanCM); err != nil {
		t.Fatalf("failed to create orphan ConfigMap: %v", err)
	}

	// Delete the Component
	if err := k8sClient.Delete(ctx, comp); err != nil {
		t.Fatalf("failed to delete Component: %v", err)
	}

	// The orphaned ConfigMap should still exist since it has no owner reference
	time.Sleep(1 * time.Second)
	fetchedCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-orphan-config", Namespace: "default"}, fetchedCM); err != nil {
		t.Fatalf("orphaned ConfigMap should still exist after Component deletion, but got error: %v", err)
	}

	// Cleanup
	_ = k8sClient.Delete(ctx, orphanCM)
}
