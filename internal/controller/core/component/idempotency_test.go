package component

import (
	"context"
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// componentWith builds a Component with the given generation, observedGeneration
// and Ready-condition reason. An empty reason means no Ready condition is set.
func componentWith(generation, observedGen int64, reason string) *conurev1alpha1.Component {
	c := &conurev1alpha1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Namespace:  "default",
			Generation: generation,
		},
		Status: conurev1alpha1.ComponentStatus{
			ObservedGeneration: observedGen,
		},
	}
	if reason != "" {
		c.Status.Conditions = []metav1.Condition{
			{
				Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
				Status: metav1.ConditionFalse,
				Reason: reason,
			},
		}
	}
	return c
}

func TestIsReconciledUpToDate(t *testing.T) {
	cases := []struct {
		name       string
		generation int64
		observed   int64
		reason     string
		want       bool
	}{
		{"no observed generation, no condition", 1, 0, "", false},
		{"observed but no condition yet", 1, 1, "", false},
		{"generation drifted ahead of observed", 2, 1, conurev1alpha1.ComponentReadyDeployingSucceedReason.String(), false},
		{"in-progress rendering", 1, 1, conurev1alpha1.ComponentReadyRenderingReason.String(), false},
		{"render failed", 1, 1, conurev1alpha1.ComponentReadyRenderingFailedReason.String(), false},
		{"deploying not yet succeeded", 1, 1, conurev1alpha1.ComponentReadyDeployingReason.String(), false},
		{"deploy succeeded — steady state", 1, 1, conurev1alpha1.ComponentReadyDeployingSucceedReason.String(), true},
		{"running — steady state", 1, 1, conurev1alpha1.ComponentReadyRunningReason.String(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isReconciledUpToDate(componentWith(tc.generation, tc.observed, tc.reason))
			if got != tc.want {
				t.Fatalf("isReconciledUpToDate=%v, want %v", got, tc.want)
			}
		})
	}
}

// TestReconcile_NoOpInSteadyState exercises Bug 1: the controller used to re-run
// renderComponent every time it saw a Component that was already deployed. The
// guard in Reconcile must short-circuit when ObservedGeneration matches and the
// condition is in a terminal-success state — otherwise renderComponent would
// fail looking up a (test-absent) ComponentDefinition and flip the status to
// RenderingFailed.
func TestReconcile_NoOpInSteadyState(t *testing.T) {
	ctx := context.Background()
	comp := componentWith(3, 3, conurev1alpha1.ComponentReadyDeployingSucceedReason.String())
	comp.Spec = conurev1alpha1.ComponentSpec{ComponentType: "webservice"}

	c := newFakeClient(comp)
	r := &ComponentReconciler{Client: c, Scheme: newTestScheme()}

	res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: comp.Name, Namespace: comp.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if res.RequeueAfter != RequeueAfter {
		t.Fatalf("expected RequeueAfter=%v, got %v", RequeueAfter, res.RequeueAfter)
	}

	got := &conurev1alpha1.Component{}
	if err := c.Get(ctx, types.NamespacedName{Name: comp.Name, Namespace: comp.Namespace}, got); err != nil {
		t.Fatalf("failed to fetch component: %v", err)
	}
	if len(got.Status.Conditions) != 1 {
		t.Fatalf("expected exactly 1 condition, got %d", len(got.Status.Conditions))
	}
	if got.Status.Conditions[0].Reason != conurev1alpha1.ComponentReadyDeployingSucceedReason.String() {
		t.Fatalf("expected condition to remain DeployingSucceed, got %s", got.Status.Conditions[0].Reason)
	}
	if got.Status.ObservedGeneration != 3 {
		t.Fatalf("expected ObservedGeneration to remain 3, got %d", got.Status.ObservedGeneration)
	}
}

// TestReconcile_DoesWorkWhenGenerationDrifts is the inverse of the steady-state
// test: when spec.generation has moved ahead of ObservedGeneration, the guard
// must NOT skip work. We verify this by observing the side-effect of the
// reconciler attempting to render — without a matching ComponentDefinition in
// the fake client, that path flips the condition to RenderingFailed.
func TestReconcile_DoesWorkWhenGenerationDrifts(t *testing.T) {
	ctx := context.Background()
	comp := componentWith(5, 4, conurev1alpha1.ComponentReadyDeployingSucceedReason.String())
	comp.Spec = conurev1alpha1.ComponentSpec{ComponentType: "webservice"}

	c := newFakeClient(comp)
	r := &ComponentReconciler{Client: c, Scheme: newTestScheme()}

	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: comp.Name, Namespace: comp.Namespace}}); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	got := &conurev1alpha1.Component{}
	if err := c.Get(ctx, types.NamespacedName{Name: comp.Name, Namespace: comp.Namespace}, got); err != nil {
		t.Fatalf("failed to fetch component: %v", err)
	}
	if len(got.Status.Conditions) == 0 {
		t.Fatalf("expected the reconciler to update conditions when generation drifts")
	}
	if got.Status.Conditions[0].Reason == conurev1alpha1.ComponentReadyDeployingSucceedReason.String() {
		t.Fatalf("expected condition to leave DeployingSucceed when generation drifted, but reconciler short-circuited")
	}
}

// TestGenerationChangedPredicate_FiltersStatusOnlyUpdate exercises Bug 2: the
// reconciler used to receive watch events for its own status patches because
// SetupWithManager registered no event filter. With
// predicate.GenerationChangedPredicate wired in, an Update event whose old and
// new objects share the same metadata.generation must be filtered out — even
// when their status fields differ.
func TestGenerationChangedPredicate_FiltersStatusOnlyUpdate(t *testing.T) {
	p := predicate.GenerationChangedPredicate{}

	oldObj := componentWith(7, 6, conurev1alpha1.ComponentReadyRenderingReason.String())
	newObj := componentWith(7, 7, conurev1alpha1.ComponentReadyDeployingSucceedReason.String())

	if p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
		t.Fatal("expected GenerationChangedPredicate to filter out a status-only update")
	}

	// Sanity check: a real spec change (generation bump) MUST pass the filter.
	bumped := componentWith(8, 7, conurev1alpha1.ComponentReadyDeployingSucceedReason.String())
	if !p.Update(event.UpdateEvent{ObjectOld: newObj, ObjectNew: bumped}) {
		t.Fatal("expected generation bump to pass the predicate")
	}
}
