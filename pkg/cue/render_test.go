package cue

import (
	"testing"

	cuego "cuelang.org/go/cue"
)

func TestRenderer_LoadRemotePackage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test that requires network access")
	}

	r := NewRenderer()

	vals := map[string]any{
		"values": map[string]any{
			"replicas": 3,
		},
	}

	val, err := r.LoadRemotePackage(
		t.Context(),
		"ghcr.io/coffeenights/conure-templates/service-flat",
		"v0.0.2",
		"service",
		vals,
	)
	if err != nil {
		t.Fatalf("LoadRemotePackage failed: %v", err)
	}

	// Look up the deployment field
	deployment := val.LookupPath(cuego.ParsePath("deployment"))
	if err := deployment.Err(); err != nil {
		t.Fatalf("Failed to lookup deployment: %v", err)
	}

	t.Logf("deployment: %v", deployment)
}
