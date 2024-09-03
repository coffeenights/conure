package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ContainsCondition(conditions []metav1.Condition, conditionType string) (int, bool) {
	for index, condition := range conditions {
		if condition.Type == conditionType {
			return index, true
		}
	}
	return -1, false
}
