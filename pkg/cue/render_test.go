package cue

import (
	"testing"
)

func TestRenderer_LoadRemotePackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test that requires network access")
	}

	r := NewRenderer("ghcr.io")

	vals := map[string]any{
		"values": map[string]any{
			"replicas": 3,
		},
	}

	val, err := r.LoadRemotePackage(
		t.Context(),
		"ghcr.io/coffeenights/conure-templates/service",
		"v0.0.3",
		"main",
		vals,
	)
	if err != nil {
		t.Fatalf("LoadRemotePackage failed: %v", err)
	}

	// Get the apply sets from the output field
	sets, err := r.GetApplySets(val)
	if err != nil {
		t.Fatalf("Failed to get apply sets: %v", err)
	}

	if len(sets) == 0 {
		t.Fatal("Expected at least one object in output")
	}

	for i, obj := range sets {
		t.Logf("output[%d]: kind=%s name=%s", i, obj.GetKind(), obj.GetName())
	}
}
