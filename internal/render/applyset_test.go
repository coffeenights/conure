package render

import (
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
)

func TestEnvelope_RoundTrip(t *testing.T) {
	payload := []byte(`[{"name":"main","objects":[]}]`)

	wrapped, err := WrapEnvelope(conurev1alpha1.EngineHelm, payload)
	if err != nil {
		t.Fatalf("WrapEnvelope: %v", err)
	}
	engine, got, err := ParseEnvelope(wrapped)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if engine != conurev1alpha1.EngineHelm {
		t.Fatalf("engine = %q, want helm", engine)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload round-trip mismatch: got %s want %s", got, payload)
	}
}

// TestEnvelope_LegacyTimoniArrayParsesAsTimoni is the back-compat contract:
// components rendered before the multi-engine refactor have a bare JSON array
// in their apply-set annotation (no envelope). ParseEnvelope must report them
// as EngineTimoni and return the payload unchanged so the drift sweep keeps
// working through the rename.
func TestEnvelope_LegacyTimoniArrayParsesAsTimoni(t *testing.T) {
	legacy := []byte(`[{"name":"main","objects":[{"apiVersion":"v1","kind":"Service"}]}]`)

	engine, got, err := ParseEnvelope(legacy)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if engine != conurev1alpha1.EngineTimoni {
		t.Fatalf("legacy payload should report timoni engine, got %q", engine)
	}
	if string(got) != string(legacy) {
		t.Fatalf("legacy payload should be returned unchanged")
	}
}

func TestEnvelope_RejectsUnknownVersion(t *testing.T) {
	data := []byte(`{"v":99,"engine":"timoni","payload":[]}`)
	if _, _, err := ParseEnvelope(data); err == nil {
		t.Fatal("expected unknown-version envelope to be rejected")
	}
}

func TestEnvelope_RejectsMissingEngine(t *testing.T) {
	data := []byte(`{"v":1,"payload":[]}`)
	if _, _, err := ParseEnvelope(data); err == nil {
		t.Fatal("expected envelope without engine to be rejected")
	}
}
