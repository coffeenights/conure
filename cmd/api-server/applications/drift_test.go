package applications

import (
	"sort"
	"testing"
)

func TestComputeDrift_NoDrift(t *testing.T) {
	live := map[string]interface{}{
		"image":    "v1",
		"replicas": 3,
	}
	deployed := map[string]interface{}{
		"image":    "v1",
		"replicas": 3,
	}
	report, err := ComputeDrift(live, deployed)
	if err != nil {
		t.Fatal(err)
	}
	if report.Drifted {
		t.Fatalf("expected no drift, got diff: %+v", report.Diff)
	}
}

func TestComputeDrift_ScalarChange(t *testing.T) {
	live := map[string]interface{}{"image": "v3"}
	deployed := map[string]interface{}{"image": "v2"}
	report, err := ComputeDrift(live, deployed)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Drifted {
		t.Fatal("expected drift")
	}
	if len(report.Diff) != 1 || report.Diff[0].Path != "image" {
		t.Fatalf("expected single diff at path image, got %+v", report.Diff)
	}
	if report.Diff[0].Live != "v3" || report.Diff[0].Deployed != "v2" {
		t.Errorf("unexpected diff content: %+v", report.Diff[0])
	}
}

func TestComputeDrift_NestedAndArray(t *testing.T) {
	live := map[string]interface{}{
		"resources": map[string]interface{}{
			"replicas": 2,
			"cpu":      "500m",
		},
		"ports": []interface{}{8080, 9090},
	}
	deployed := map[string]interface{}{
		"resources": map[string]interface{}{
			"replicas": 3,
			"cpu":      "500m",
		},
		"ports": []interface{}{8080, 9091},
	}
	report, err := ComputeDrift(live, deployed)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Drifted {
		t.Fatal("expected drift")
	}
	paths := make([]string, len(report.Diff))
	for i, e := range report.Diff {
		paths[i] = e.Path
	}
	sort.Strings(paths)
	want := []string{"ports.1", "resources.replicas"}
	if len(paths) != len(want) {
		t.Fatalf("expected paths %v, got %v", want, paths)
	}
	for i := range paths {
		if paths[i] != want[i] {
			t.Errorf("path[%d] = %s, want %s", i, paths[i], want[i])
		}
	}
}

func TestComputeDrift_NumericNormalization(t *testing.T) {
	// Mongo decodes ints as int32/int64, JSON gives float64. Drift must not
	// be reported when the underlying value is the same.
	live := map[string]interface{}{"replicas": float64(3)}
	deployed := map[string]interface{}{"replicas": int32(3)}
	report, err := ComputeDrift(live, deployed)
	if err != nil {
		t.Fatal(err)
	}
	if report.Drifted {
		t.Fatalf("expected no drift after normalization, got %+v", report.Diff)
	}
}
