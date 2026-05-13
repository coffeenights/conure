package ui

import (
	"encoding/json"
	"fmt"
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

// Render is the canonical "either dump JSON or run my text renderer" helper.
// In JSON mode it serializes v; otherwise it runs textFn. Returning the
// error from textFn (rather than always nil) lets text rendering report
// failures the same way JSON mode does.
func Render(v any, textFn func() error) error {
	if jsonMode {
		return PrintJSON(v)
	}
	if textFn == nil {
		return nil
	}
	return textFn()
}
