package cue

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Renderer handles CUE template rendering with input values.
type Renderer struct {
	ctx *cue.Context
}

// NewRenderer creates a new CUE renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		ctx: cuecontext.New(),
	}
}

// Render takes a CUE template string and a map of values,
// unifies them, and returns the rendered result as bytes.
func (r *Renderer) Render(template string, values map[string]any) ([]byte, error) {
	// Compile the template
	templateVal := r.ctx.CompileString(template)
	if err := templateVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile template: %w", err)
	}

	// Encode the values map into a CUE value
	valuesVal := r.ctx.Encode(values)
	if err := valuesVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to encode values: %w", err)
	}

	// Unify template with values
	result := templateVal.Unify(valuesVal)
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("failed to unify template with values: %w", err)
	}

	// Validate the result is concrete
	if err := result.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("result is not concrete: %w", err)
	}

	// Marshal to JSON
	bytes, err := result.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return bytes, nil
}

// RenderToValue renders and returns the CUE value for further processing.
func (r *Renderer) RenderToValue(template string, values map[string]any) (cue.Value, error) {
	templateVal := r.ctx.CompileString(template)
	if err := templateVal.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to compile template: %w", err)
	}

	valuesVal := r.ctx.Encode(values)
	if err := valuesVal.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to encode values: %w", err)
	}

	result := templateVal.Unify(valuesVal)
	if err := result.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to unify: %w", err)
	}

	return result, nil
}

// RenderPartial renders without requiring all fields to be concrete.
// Useful for intermediate processing steps.
func (r *Renderer) RenderPartial(template string, values map[string]any) ([]byte, error) {
	templateVal := r.ctx.CompileString(template)
	if err := templateVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile template: %w", err)
	}

	valuesVal := r.ctx.Encode(values)
	if err := valuesVal.Err(); err != nil {
		return nil, fmt.Errorf("failed to encode values: %w", err)
	}

	result := templateVal.Unify(valuesVal)
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("failed to unify: %w", err)
	}

	bytes, err := result.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return bytes, nil
}
