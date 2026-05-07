package common

import (
	"testing"
)

func TestGetHashForSpec_Deterministic(t *testing.T) {
	spec := map[string]string{"key": "value"}

	hash1 := GetHashForSpec(spec)
	hash2 := GetHashForSpec(spec)

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash, got %s and %s", hash1, hash2)
	}
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestGetHashForSpec_DifferentInputs(t *testing.T) {
	hash1 := GetHashForSpec(map[string]string{"key": "value1"})
	hash2 := GetHashForSpec(map[string]string{"key": "value2"})

	if hash1 == hash2 {
		t.Fatal("expected different hashes for different inputs")
	}
}

func TestGetHashForSpec_Struct(t *testing.T) {
	type testSpec struct {
		Name    string `json:"name"`
		Replica int    `json:"replica"`
	}

	hash := GetHashForSpec(testSpec{Name: "web", Replica: 3})
	if hash == "" {
		t.Fatal("expected non-empty hash for struct input")
	}

	// Same struct different values should differ
	hash2 := GetHashForSpec(testSpec{Name: "web", Replica: 5})
	if hash == hash2 {
		t.Fatal("expected different hashes for different struct values")
	}
}

func TestSetHashToLabels_NewMap(t *testing.T) {
	labels := SetHashToLabels(nil, "abc123")

	if labels == nil {
		t.Fatal("expected non-nil labels map")
	}
	if labels[HashLabelName] != "abc123" {
		t.Fatalf("expected hash abc123, got %s", labels[HashLabelName])
	}
}

func TestSetHashToLabels_ExistingMap(t *testing.T) {
	labels := map[string]string{"app": "myapp"}

	labels = SetHashToLabels(labels, "def456")

	if labels[HashLabelName] != "def456" {
		t.Fatalf("expected hash def456, got %s", labels[HashLabelName])
	}
	if labels["app"] != "myapp" {
		t.Fatal("expected existing label to be preserved")
	}
}

func TestSetHashToLabels_OverwritesExisting(t *testing.T) {
	labels := map[string]string{HashLabelName: "old"}

	labels = SetHashToLabels(labels, "new")

	if labels[HashLabelName] != "new" {
		t.Fatalf("expected hash to be overwritten to 'new', got %s", labels[HashLabelName])
	}
}

func TestGetHashFromLabels(t *testing.T) {
	labels := map[string]string{HashLabelName: "abc123"}

	hash := GetHashFromLabels(labels)
	if hash != "abc123" {
		t.Fatalf("expected abc123, got %s", hash)
	}
}

func TestGetHashFromLabels_Missing(t *testing.T) {
	labels := map[string]string{"app": "myapp"}

	hash := GetHashFromLabels(labels)
	if hash != "" {
		t.Fatalf("expected empty string for missing hash, got %s", hash)
	}
}

func TestHashRoundTrip(t *testing.T) {
	spec := map[string]interface{}{"replicas": 3, "image": "nginx:latest"}
	hash := GetHashForSpec(spec)

	labels := SetHashToLabels(nil, hash)
	retrieved := GetHashFromLabels(labels)

	if retrieved != hash {
		t.Fatalf("round-trip failed: set %s, got %s", hash, retrieved)
	}
}
