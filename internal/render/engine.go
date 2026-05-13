package render

import (
	"context"

	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Engine renders a single component instance and applies its manifests to the
// cluster. Implementations encapsulate everything engine-specific: OCI pulls,
// templating, value coalescing. Callers only see ResourceSet and the SSA
// change-set entries returned by ApplyObject.
//
// Render is called once per reconcile that needs a fresh template; the result
// is then persisted to the Component's apply-sets annotation. ApplyObject is
// called per object, both after Render and during the periodic drift sweep
// (which uses Unmarshal to rehydrate the cached set).
type Engine interface {
	Render(ctx context.Context) ([]ResourceSet, error)

	// Digest returns the resolved OCI manifest digest of the pulled artifact,
	// or "" if the engine has not pulled (e.g. a BuildForApply instance) or
	// the source does not expose one.
	Digest() string

	MarshalApplySets(sets []ResourceSet) ([]byte, error)
	UnmarshalApplySets(data []byte) ([]ResourceSet, error)

	ApplyObject(ctx context.Context, obj *unstructured.Unstructured, force bool) (*ssa.ChangeSetEntry, error)
}
