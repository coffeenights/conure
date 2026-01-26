package cue

import (
	"encoding/json"
	"testing"

	cuego "cuelang.org/go/cue"
)

func TestRenderer_Render(t *testing.T) {
	r := NewRenderer()

	template := `
		name: string
		replicas: int
		image: string
		labels: {
			app: name
			version: string
		}
	`

	values := map[string]any{
		"name":     "my-app",
		"replicas": 3,
		"image":    "nginx:latest",
		"labels": map[string]any{
			"version": "v1.0.0",
		},
	}

	result, err := r.Render(template, values)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output["name"] != "my-app" {
		t.Errorf("expected name=my-app, got %v", output["name"])
	}

	if output["replicas"].(float64) != 3 {
		t.Errorf("expected replicas=3, got %v", output["replicas"])
	}

	labels := output["labels"].(map[string]any)
	if labels["app"] != "my-app" {
		t.Errorf("expected labels.app=my-app, got %v", labels["app"])
	}
}

func TestRenderer_RenderWithDefaults(t *testing.T) {
	r := NewRenderer()

	template := `
		name: string
		replicas: int | *1
		enabled: bool | *true
	`

	values := map[string]any{
		"name": "test-service",
	}

	result, err := r.Render(template, values)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(result, &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output["replicas"].(float64) != 1 {
		t.Errorf("expected default replicas=1, got %v", output["replicas"])
	}

	if output["enabled"] != true {
		t.Errorf("expected default enabled=true, got %v", output["enabled"])
	}
}

func TestRenderer_RenderToValue(t *testing.T) {
	r := NewRenderer()

	template := `
		port: int
		host: string
	`

	values := map[string]any{
		"port": 8080,
		"host": "localhost",
	}

	val, err := r.RenderToValue(template, values)
	if err != nil {
		t.Fatalf("RenderToValue failed: %v", err)
	}

	portVal := val.LookupPath(cuego.ParsePath("port"))
	port, err := portVal.Int64()
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}

	if port != 8080 {
		t.Errorf("expected port=8080, got %d", port)
	}
}

func TestRenderer_RenderInvalidTemplate(t *testing.T) {
	r := NewRenderer()

	template := `{ invalid cue syntax`
	values := map[string]any{}

	_, err := r.Render(template, values)
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestRenderer_RenderMissingRequiredField(t *testing.T) {
	r := NewRenderer()

	template := `
		required: string
	`

	values := map[string]any{}

	_, err := r.Render(template, values)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}
