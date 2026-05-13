// Package timoni adapts the Timoni module manager to the render.Engine
// interface. The render-layer ResourceSet has the same JSON shape as Timoni's
// own engine.ResourceSet, so Marshal/Unmarshal round-trip through plain JSON
// without explicit conversion.
package timoni

import (
	"context"
	"encoding/json"

	"github.com/fluxcd/pkg/ssa"
	tmod "github.com/stefanprodan/timoni/pkg/module"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/coffeenights/conure/internal/render"
)

// Engine wraps a Timoni module manager and exposes it through render.Engine.
type Engine struct {
	mgr *tmod.Manager
}

// New constructs a Timoni-backed Engine. mgr may be nil for the BuildForApply
// flavor (no fetch); ApplyObject still works because Timoni's Manager.ApplyObject
// builds its own RESTClientGetter internally on each call.
func New(mgr *tmod.Manager) *Engine {
	return &Engine{mgr: mgr}
}

func (e *Engine) Render(ctx context.Context) ([]render.ResourceSet, error) {
	sets, err := e.mgr.GetApplySets()
	if err != nil {
		return nil, err
	}
	out := make([]render.ResourceSet, len(sets))
	for i := range sets {
		out[i] = render.ResourceSet{Name: sets[i].Name, Objects: sets[i].Objects}
	}
	return out, nil
}

func (e *Engine) Digest() string {
	if e.mgr == nil {
		return ""
	}
	return e.mgr.GetDigest()
}

// MarshalApplySets emits a plain JSON array, matching the on-wire shape that
// Timoni has always produced. The versioned envelope is wrapped around this
// payload at the handler layer (see render.WrapEnvelope).
func (e *Engine) MarshalApplySets(sets []render.ResourceSet) ([]byte, error) {
	return json.Marshal(sets)
}

func (e *Engine) UnmarshalApplySets(data []byte) ([]render.ResourceSet, error) {
	var sets []render.ResourceSet
	if err := json.Unmarshal(data, &sets); err != nil {
		return nil, err
	}
	return sets, nil
}

func (e *Engine) ApplyObject(ctx context.Context, obj *unstructured.Unstructured, force bool) (*ssa.ChangeSetEntry, error) {
	return e.mgr.ApplyObject(obj, force)
}
