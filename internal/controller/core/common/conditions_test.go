package common

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestContainsCondition_Found(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "AllGood"},
		{Type: "Rendering", Status: metav1.ConditionFalse, Reason: "Pending"},
	}

	index, found := ContainsCondition(conditions, "Rendering")
	if !found {
		t.Fatal("expected condition to be found")
	}
	if index != 1 {
		t.Fatalf("expected index 1, got %d", index)
	}
}

func TestContainsCondition_NotFound(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue, Reason: "AllGood"},
	}

	index, found := ContainsCondition(conditions, "Deploying")
	if found {
		t.Fatal("expected condition not to be found")
	}
	if index != -1 {
		t.Fatalf("expected index -1, got %d", index)
	}
}

func TestContainsCondition_EmptySlice(t *testing.T) {
	index, found := ContainsCondition(nil, "Ready")
	if found {
		t.Fatal("expected condition not to be found in nil slice")
	}
	if index != -1 {
		t.Fatalf("expected index -1, got %d", index)
	}
}

func TestSetCondition_AddsNew(t *testing.T) {
	var conditions []metav1.Condition

	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "Deployed", "all components deployed")

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	c := conditions[0]
	if c.Type != "Ready" {
		t.Fatalf("expected type Ready, got %s", c.Type)
	}
	if c.Status != metav1.ConditionTrue {
		t.Fatalf("expected status True, got %s", c.Status)
	}
	if c.Reason != "Deployed" {
		t.Fatalf("expected reason Deployed, got %s", c.Reason)
	}
	if c.Message != "all components deployed" {
		t.Fatalf("expected message 'all components deployed', got %s", c.Message)
	}
	if c.LastTransitionTime.IsZero() {
		t.Fatal("expected LastTransitionTime to be set")
	}
}

func TestSetCondition_UpdatesExisting(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Pending", Message: "waiting"},
	}

	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "Deployed", "done")

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(conditions))
	}
	c := conditions[0]
	if c.Status != metav1.ConditionTrue {
		t.Fatalf("expected updated status True, got %s", c.Status)
	}
	if c.Reason != "Deployed" {
		t.Fatalf("expected updated reason Deployed, got %s", c.Reason)
	}
}

func TestSetCondition_PreservesLastTransitionTimeOnSameStatus(t *testing.T) {
	conditions := SetCondition(nil, "Ready", metav1.ConditionFalse, "Pending", "msg one")
	original := conditions[0].LastTransitionTime
	if original.IsZero() {
		t.Fatal("expected LastTransitionTime to be set on first call")
	}

	// Force a measurable wall-clock gap so an unintended overwrite is observable.
	time.Sleep(10 * time.Millisecond)

	conditions = SetCondition(conditions, "Ready", metav1.ConditionFalse, "Pending", "msg two")
	if !conditions[0].LastTransitionTime.Equal(&original) {
		t.Fatalf("expected LastTransitionTime to be preserved on same Status; got %v, want %v", conditions[0].LastTransitionTime, original)
	}
	if conditions[0].Message != "msg two" {
		t.Fatalf("expected message to be updated to 'msg two', got %q", conditions[0].Message)
	}
}

func TestSetCondition_UpdatesLastTransitionTimeOnStatusFlip(t *testing.T) {
	conditions := SetCondition(nil, "Ready", metav1.ConditionFalse, "Pending", "")
	original := conditions[0].LastTransitionTime

	time.Sleep(10 * time.Millisecond)

	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "Deployed", "")
	if conditions[0].LastTransitionTime.Equal(&original) {
		t.Fatal("expected LastTransitionTime to advance when Status flips")
	}
}

func TestSetCondition_MultipleTypes(t *testing.T) {
	var conditions []metav1.Condition

	conditions = SetCondition(conditions, "Ready", metav1.ConditionTrue, "OK", "")
	conditions = SetCondition(conditions, "Rendering", metav1.ConditionFalse, "InProgress", "")

	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}

	// Update only one
	conditions = SetCondition(conditions, "Ready", metav1.ConditionFalse, "Degraded", "error")

	if len(conditions) != 2 {
		t.Fatalf("expected still 2 conditions after update, got %d", len(conditions))
	}
	if conditions[0].Reason != "Degraded" {
		t.Fatalf("expected Ready reason to be updated to Degraded, got %s", conditions[0].Reason)
	}
	if conditions[1].Reason != "InProgress" {
		t.Fatalf("expected Rendering reason to remain InProgress, got %s", conditions[1].Reason)
	}
}
