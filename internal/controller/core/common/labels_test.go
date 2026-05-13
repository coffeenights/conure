package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSelectConureLabels_FiltersAndDropsEmpty(t *testing.T) {
	src := map[string]string{
		"conure.io/component-id":   "c1",
		"conure.io/component-name": "web",
		"conure.io/environment":    "",
		"unrelated/label":          "ignore",
	}
	got := SelectConureLabels(src)
	if got["conure.io/component-id"] != "c1" || got["conure.io/component-name"] != "web" {
		t.Fatalf("missing expected keys: %#v", got)
	}
	if _, ok := got["conure.io/environment"]; ok {
		t.Fatalf("empty value should have been dropped: %#v", got)
	}
	if _, ok := got["unrelated/label"]; ok {
		t.Fatalf("non-conure key should not pass: %#v", got)
	}
}

func TestPropagateLabelsToRendered_ConfigMap(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]interface{}{"name": "cfg"},
	}}
	if err := PropagateLabelsToRendered(obj, map[string]string{"conure.io/component-name": "web"}); err != nil {
		t.Fatal(err)
	}
	if obj.GetLabels()["conure.io/component-name"] != "web" {
		t.Fatalf("metadata.labels not set: %#v", obj.GetLabels())
	}
}

func TestPropagateLabelsToRendered_Deployment(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "web"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{"app.kubernetes.io/name": "web"},
				},
			},
		},
	}}
	in := map[string]string{
		"conure.io/component-id":   "c1",
		"conure.io/component-name": "web",
	}
	if err := PropagateLabelsToRendered(obj, in); err != nil {
		t.Fatal(err)
	}
	tpl, _, err := unstructured.NestedStringMap(obj.Object, "spec", "template", "metadata", "labels")
	if err != nil {
		t.Fatal(err)
	}
	if tpl["conure.io/component-id"] != "c1" || tpl["conure.io/component-name"] != "web" {
		t.Fatalf("pod template labels missing conure keys: %#v", tpl)
	}
	if tpl["app.kubernetes.io/name"] != "web" {
		t.Fatalf("existing template label was clobbered: %#v", tpl)
	}
}

func TestPropagateLabelsToRendered_CronJob(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]interface{}{"name": "cron"},
		"spec": map[string]interface{}{
			"jobTemplate": map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{},
					},
				},
			},
		},
	}}
	if err := PropagateLabelsToRendered(obj, map[string]string{"conure.io/component-name": "cron"}); err != nil {
		t.Fatal(err)
	}
	tpl, _, err := unstructured.NestedStringMap(obj.Object, "spec", "jobTemplate", "spec", "template", "metadata", "labels")
	if err != nil {
		t.Fatal(err)
	}
	if tpl["conure.io/component-name"] != "cron" {
		t.Fatalf("cron pod template labels missing: %#v", tpl)
	}
}

func TestPropagateLabelsToRendered_NoLabelsNoop(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "web"},
	}}
	if err := PropagateLabelsToRendered(obj, nil); err != nil {
		t.Fatal(err)
	}
	if obj.GetLabels() != nil {
		t.Fatalf("expected no labels added: %#v", obj.GetLabels())
	}
}
