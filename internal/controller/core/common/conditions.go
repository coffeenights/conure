package common

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// ContainsCondition checks if a condition with the given type exists in the conditions slice.
// Returns the index and true if exists, -1 and false otherwise.
func ContainsCondition(conditions []metav1.Condition, conditionType string) (int, bool) {
	for index, condition := range conditions {
		if condition.Type == conditionType {
			return index, true
		}
	}
	return -1, false
}

// SetCondition sets a condition with the given type, status, reason and message in the conditions slice.
func SetCondition(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus, reason string, message string) []metav1.Condition {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Time{Time: time.Now()},
	}
	index, exists := ContainsCondition(conditions, conditionType)
	if exists {
		conditions[index] = condition
	} else {
		conditions = append(conditions, condition)
	}
	return conditions
}

// ApplyStatus updates the status of the object with the latest generation to prevent.
func ApplyStatus(ctx context.Context, object client.Object, restClient client.Client) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newObject := object.DeepCopyObject().(client.Object)
		if err := restClient.Get(ctx, client.ObjectKey{Namespace: object.GetNamespace(), Name: object.GetName()}, newObject); err != nil {
			return err
		}
		object.SetResourceVersion(newObject.GetResourceVersion())
		return restClient.Status().Update(ctx, object)
	})
	if err != nil {
		return err
	}
	return nil
}
