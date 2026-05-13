package common

import (
	"fmt"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// conureLabelKeys are the labels copied from the Component CRD onto every
// rendered child resource (and into pod templates of workload kinds) so the
// API server can locate pods by conure-native selectors instead of relying on
// Timoni module conventions (e.g. app.kubernetes.io/name).
var conureLabelKeys = []string{
	k8sUtils.ApplicationIDLabel,
	k8sUtils.OrganizationIDLabel,
	k8sUtils.EnvironmentLabel,
	k8sUtils.ComponentIDLabel,
	k8sUtils.ComponentNameLabel,
	k8sUtils.CreatedByLabel,
}

// SelectConureLabels picks the subset of source containing conure-managed
// label keys with non-empty values.
func SelectConureLabels(source map[string]string) map[string]string {
	out := map[string]string{}
	for _, k := range conureLabelKeys {
		if v, ok := source[k]; ok && v != "" {
			out[k] = v
		}
	}
	return out
}

func mergeLabels(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// podTemplatePaths lists the path to the pod template's labels for every
// workload kind that owns one. Adding labels here is what makes the resulting
// Pods carry the labels — top-level metadata.labels alone is not inherited.
//
// spec.selector is deliberately untouched: it is immutable on most kinds, and
// adding labels to the template without changing the selector is safe (the
// existing matchLabels remain a subset of the template's labels).
var podTemplatePaths = map[string][]string{
	"Deployment":  {"spec", "template", "metadata", "labels"},
	"StatefulSet": {"spec", "template", "metadata", "labels"},
	"DaemonSet":   {"spec", "template", "metadata", "labels"},
	"ReplicaSet":  {"spec", "template", "metadata", "labels"},
	"Job":         {"spec", "template", "metadata", "labels"},
	"CronJob":     {"spec", "jobTemplate", "spec", "template", "metadata", "labels"},
}

// PropagateLabelsToRendered merges labels into obj's top-level metadata.labels
// and, for known workload kinds, into the embedded pod template so resulting
// Pods inherit them too.
func PropagateLabelsToRendered(obj *unstructured.Unstructured, labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}
	obj.SetLabels(mergeLabels(obj.GetLabels(), labels))

	path, ok := podTemplatePaths[obj.GetKind()]
	if !ok {
		return nil
	}
	existing, _, err := unstructured.NestedStringMap(obj.Object, path...)
	if err != nil {
		return fmt.Errorf("reading %s/%s template labels: %w", obj.GetKind(), obj.GetName(), err)
	}
	return unstructured.SetNestedStringMap(obj.Object, mergeLabels(existing, labels), path...)
}
