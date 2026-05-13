package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
)

// applySetEnvelope wraps the engine-specific apply-set payload with a version
// tag and engine identifier. Multi-engine support requires the controller to
// know which engine rendered the cached set so the drift sweep can dispatch
// UnmarshalApplySets to the right adapter.
//
// Wire format (after gunzip, before base64):
//
//	{"v":1,"engine":"timoni","payload":[{"name":"...","objects":[...]}]}
//
// The legacy Timoni format is a bare JSON array (starts with '['). Read
// callers detect that by peeking the first non-whitespace byte and treat it
// as an unversioned, Timoni-engine payload — see ParseEnvelope.
type applySetEnvelope struct {
	Version int                            `json:"v"`
	Engine  conurev1alpha1.ComponentEngine `json:"engine"`
	Payload json.RawMessage                `json:"payload"`
}

const envelopeVersion = 1

// WrapEnvelope wraps an engine-specific apply-set marshal output with the
// versioned envelope. Callers should pass the bytes returned by
// Engine.MarshalApplySets verbatim.
func WrapEnvelope(engine conurev1alpha1.ComponentEngine, payload []byte) ([]byte, error) {
	env := applySetEnvelope{
		Version: envelopeVersion,
		Engine:  engine,
		Payload: payload,
	}
	return json.Marshal(env)
}

// ParseEnvelope returns the engine that produced the apply-set and the raw
// payload to feed to Engine.UnmarshalApplySets. Legacy un-enveloped Timoni
// payloads (a bare JSON array) are detected and reported as EngineTimoni so
// pre-refactor components continue to reconcile without migration.
func ParseEnvelope(data []byte) (conurev1alpha1.ComponentEngine, []byte, error) {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return "", nil, fmt.Errorf("apply-set envelope is empty")
	}
	if trimmed[0] == '[' {
		return conurev1alpha1.EngineTimoni, data, nil
	}

	var env applySetEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, fmt.Errorf("decoding apply-set envelope: %w", err)
	}
	if env.Version != envelopeVersion {
		return "", nil, fmt.Errorf("unsupported apply-set envelope version %d (want %d)", env.Version, envelopeVersion)
	}
	if env.Engine == "" {
		return "", nil, fmt.Errorf("apply-set envelope missing engine identifier")
	}
	return env.Engine, env.Payload, nil
}
