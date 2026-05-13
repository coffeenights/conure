package ui

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

// OutputMode is the format Render produces when called with data. Text is
// the human-friendly default; JSON and YAML are machine-readable.
type OutputMode string

const (
	OutputText OutputMode = "text"
	OutputJSON OutputMode = "json"
	OutputYAML OutputMode = "yaml"
)

// PrintJSON marshals v as pretty JSON to stdout. Returns an error only if
// marshalling fails — the json branch is normally simple enough that callers
// can ignore it, but the error is surfaced so the cobra command can return
// it cleanly.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// PrintYAML marshals v as YAML to stdout. Uses sigs.k8s.io/yaml so the
// existing `json:"..."` tags on pkg/api types double as YAML keys, and
// map[string]interface{} values round-trip cleanly (no map[interface{}]
// interface{} artifacts).
func PrintYAML(v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling YAML: %w", err)
	}
	fmt.Print(string(data))
	return nil
}

// Render is the canonical "either dump structured output or run my text
// renderer" helper. JSON/YAML modes serialize v; text mode runs textFn so
// commands keep one code path for both formats.
func Render(v any, textFn func() error) error {
	switch outputMode {
	case OutputJSON:
		return PrintJSON(v)
	case OutputYAML:
		return PrintYAML(v)
	}
	if textFn == nil {
		return nil
	}
	return textFn()
}
