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
// and a Ready-condition status (when readyStatus is non-empty).
func componentWith(generation, observedGen int64, readyStatus metav1.ConditionStatus, reason conurev1alpha1.ComponentConditionReason) *conurev1alpha1.Component {
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
	if readyStatus != "" {
		c.Status.Conditions = []metav1.Condition{
			{
				Type:   conurev1alpha1.ComponentConditionTypeReady.String(),
				Status: readyStatus,
				Reason: reason.String(),
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
		status     metav1.ConditionStatus
		reason     conurev1alpha1.ComponentConditionReason
		want       bool
	}{
		{"no observed generation, no condition", 1, 0, "", "", false},
		{"observed but no condition yet", 1, 1, "", "", false},
		{"generation drifted ahead of observed", 2, 1, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady, false},
		{"in-progress (Ready=False)", 1, 1, metav1.ConditionFalse, conurev1alpha1.ComponentReasonInProgress, false},
		{"render failed (Ready=False)", 1, 1, metav1.ConditionFalse, conurev1alpha1.ComponentReasonRenderingFailed, false},
		{"deploy failed (Ready=False)", 1, 1, metav1.ConditionFalse, conurev1alpha1.ComponentReasonDeploymentFailed, false},
		{"ready — steady state", 1, 1, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isReconciledUpToDate(componentWith(tc.generation, tc.observed, tc.status, tc.reason))
			if got != tc.want {
				t.Fatalf("isReconciledUpToDate=%v, want %v", got, tc.want)
			}
		})
	}
}

// TestReconcile_NoOpInSteadyState exercises Bug 1: the controller used to re-run
// renderComponent every time it saw a Component that was already deployed. The
// guard in Reconcile must short-circuit when ObservedGeneration matches and
// Ready=True — otherwise renderComponent would fail looking up a (test-absent)
// ComponentDefinition and flip Rendered to Failed.
func TestReconcile_NoOpInSteadyState(t *testing.T) {
	ctx := context.Background()
	comp := componentWith(3, 3, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady)
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
	if got.Status.Conditions[0].Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready to remain True, got %s", got.Status.Conditions[0].Status)
	}
	if got.Status.ObservedGeneration != 3 {
		t.Fatalf("expected ObservedGeneration to remain 3, got %d", got.Status.ObservedGeneration)
	}
}

// TestReconcile_DoesWorkWhenGenerationDrifts is the inverse of the steady-state
// test: when spec.generation has moved ahead of ObservedGeneration, the guard
// must NOT skip work. We verify this by observing the side-effect of the
// reconciler attempting to render — without a matching ComponentDefinition in
// the fake client, that path flips Rendered to Failed.
func TestReconcile_DoesWorkWhenGenerationDrifts(t *testing.T) {
	ctx := context.Background()
	comp := componentWith(5, 4, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady)
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
	if _, ok := containsType(got.Status.Conditions, conurev1alpha1.ComponentConditionTypeRendered.String()); !ok {
		t.Fatal("expected the reconciler to write a Rendered condition when generation drifts")
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

	oldObj := componentWith(7, 6, metav1.ConditionFalse, conurev1alpha1.ComponentReasonInProgress)
	newObj := componentWith(7, 7, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady)

	if p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
		t.Fatal("expected GenerationChangedPredicate to filter out a status-only update")
	}

	// Sanity check: a real spec change (generation bump) MUST pass the filter.
	bumped := componentWith(8, 7, metav1.ConditionTrue, conurev1alpha1.ComponentReasonReady)
	if !p.Update(event.UpdateEvent{ObjectOld: newObj, ObjectNew: bumped}) {
		t.Fatal("expected generation bump to pass the predicate")
	}
}

func containsType(conditions []metav1.Condition, conditionType string) (metav1.Condition, bool) {
	for _, c := range conditions {
		if c.Type == conditionType {
			return c, true
		}
	}
	return metav1.Condition{}, false
}
