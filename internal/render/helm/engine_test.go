package helm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	chartv2loader "helm.sh/helm/v4/pkg/chart/v2/loader"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// loadSample loads the testdata/sample chart from disk. Using LoadDir/LoadFile
// directly keeps the test self-contained — we don't need to round-trip
// through tar/gzip just to exercise rendering.
func loadSample(t *testing.T) *Engine {
	t.Helper()
	chart, err := chartv2loader.Load(filepath.Join("testdata", "sample"))
	if err != nil {
		t.Fatalf("loading sample chart: %v", err)
	}
	def := &conurev1alpha1.ComponentDefinition{
		Spec: conurev1alpha1.ComponentDefinitionSpec{
			Engine: conurev1alpha1.EngineHelm,
		},
	}
	comp := &conurev1alpha1.Component{}
	comp.Name = "web"
	comp.Namespace = "default"
	comp.Spec.Values = &runtime.RawExtension{Raw: []byte(`{"replicas":2}`)}

	values, err := decodeValues(comp)
	if err != nil {
		t.Fatalf("decoding values: %v", err)
	}
	return &Engine{
		chart:   chart,
		values:  values,
		release: buildReleaseOptions(def, comp),
		caps:    buildCapabilities(def),
	}
}

func TestRender_ProducesDeploymentAndService(t *testing.T) {
	e := loadSample(t)
	sets, err := e.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 ResourceSet, got %d", len(sets))
	}

	kinds := map[string]bool{}
	for _, o := range sets[0].Objects {
		kinds[o.GetKind()] = true
	}
	if !kinds["Deployment"] {
		t.Fatal("expected Deployment in rendered output")
	}
	if !kinds["Service"] {
		t.Fatal("expected Service in rendered output")
	}
}

// TestRender_FiltersHelmHooks asserts that any manifest carrying the
// helm.sh/hook annotation is dropped — see helmHookAnnotation.
func TestRender_FiltersHelmHooks(t *testing.T) {
	e := loadSample(t)
	sets, _ := e.Render(context.Background())
	for _, o := range sets[0].Objects {
		if _, isHook := o.GetAnnotations()["helm.sh/hook"]; isHook {
			t.Fatalf("hook resource %s/%s should have been filtered", o.GetKind(), o.GetName())
		}
		if o.GetKind() == "Job" {
			t.Fatal("Job (annotated as helm hook in testdata) should have been filtered")
		}
	}
}

// TestRender_ValuesPropagate verifies that user-supplied values override
// defaults in values.yaml — confirming the CoalesceValues + ToRenderValues
// path actually feeds chart templates.
func TestRender_ValuesPropagate(t *testing.T) {
	e := loadSample(t)
	sets, err := e.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, o := range sets[0].Objects {
		if o.GetKind() != "Deployment" {
			continue
		}
		replicas, found, err := nestedInt64(o.Object, "spec", "replicas")
		if err != nil || !found {
			t.Fatalf("Deployment.spec.replicas missing: %v %v", err, found)
		}
		if replicas != 2 {
			t.Fatalf("expected replicas=2 (from override), got %d", replicas)
		}
		return
	}
	t.Fatal("Deployment not found")
}

// TestEngine_ApplySetsRoundTrip asserts the engine's Marshal/Unmarshal can
// round-trip an apply-set so the drift sweep sees the same objects back.
func TestEngine_ApplySetsRoundTrip(t *testing.T) {
	e := loadSample(t)
	sets, err := e.Render(context.Background())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	data, err := e.MarshalApplySets(sets)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := e.UnmarshalApplySets(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got) != len(sets) {
		t.Fatalf("set count mismatch: got %d want %d", len(got), len(sets))
	}
	if len(got[0].Objects) != len(sets[0].Objects) {
		t.Fatalf("object count mismatch")
	}
}

// nestedInt64 reads a numeric field that may have decoded into a json.Number
// or a float (unstructured stores both depending on the input pipeline).
func nestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	cur := interface{}(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return 0, false, nil
		}
		cur, ok = m[f]
		if !ok {
			return 0, false, nil
		}
	}
	switch v := cur.(type) {
	case int64:
		return v, true, nil
	case float64:
		return int64(v), true, nil
	case json.Number:
		i, err := v.Int64()
		return i, true, err
	}
	return 0, false, nil
}
