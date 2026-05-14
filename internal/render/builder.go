package render

import (
	"context"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
)

// Builder constructs Engine instances for a specific component. Implementations
// are engine-specific (one per Timoni, one per Helm). The controller owns a map
// of builders keyed by ComponentEngine and selects the right one based on the
// ComponentDefinition (render path) or the apply-sets envelope (drift path).
//
// The two-flavor split mirrors the two call sites on the handler today:
//
//   - Build is invoked from RenderComponent and pulls/loads the source.
//   - BuildForApply is invoked from ReconcileDeployedObjects and skips the
//     fetch — it only needs to be able to UnmarshalApplySets and ApplyObject
//     against the controller's REST config.
type Builder interface {
	// Build returns an Engine prepared to render the given component. creds is
	// the resolved "user:password" pair for the OCI registry hosting the
	// module/chart, or "" for anonymous pulls — the controller resolves it
	// before calling so the engine layer stays free of dockerconfigjson
	// plumbing.
	Build(ctx context.Context, def *conurev1alpha1.ComponentDefinition, comp *conurev1alpha1.Component, creds string) (Engine, error)
	// BuildForApply returns an Engine that can Unmarshal apply-sets and call
	// ApplyObject without performing a fetch/render. The drift sweep doesn't
	// have a ComponentDefinition handy, so only the Component is supplied.
	BuildForApply(ctx context.Context, comp *conurev1alpha1.Component) (Engine, error)
}

// Selector returns the Builder configured for a given ComponentDefinition.
// The controller passes a Selector into ComponentReconciler so the handler
// stays decoupled from the concrete engines registered at startup.
type Selector func(def *conurev1alpha1.ComponentDefinition) (Builder, error)

// SelectorForEngine returns the Builder for a previously rendered component's
// engine label (read from the apply-sets envelope). Used by the drift sweep,
// which doesn't have a ComponentDefinition handy and must dispatch on the
// engine that originally rendered the cached set.
type SelectorForEngine func(engine conurev1alpha1.ComponentEngine) (Builder, error)
