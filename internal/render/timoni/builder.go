package timoni

import (
	"context"

	tmod "github.com/stefanprodan/timoni/pkg/module"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/render"
)

// Builder is the render.Builder for Timoni CUE modules. CacheDir is forwarded
// to Timoni's fetcher so OCI artifacts are reused across reconciles.
type Builder struct {
	CacheDir string
}

// NewBuilder returns a Timoni Builder.
func NewBuilder(cacheDir string) *Builder {
	return &Builder{CacheDir: cacheDir}
}

// Build wires up a full Timoni Manager (pulls + builds) and wraps it in an
// Engine. Values are pulled from the Component's Spec.Values RawExtension by
// the caller (handler) before this call — see Builder.buildValues for the
// helper exported below.
func (b *Builder) Build(ctx context.Context, def *conurev1alpha1.ComponentDefinition, comp *conurev1alpha1.Component, creds string) (render.Engine, error) {
	values, err := timoniValues(comp)
	if err != nil {
		return nil, err
	}
	mgr, err := tmod.NewManager(
		ctx,
		comp.Name,
		"oci://"+def.Spec.OCIRepository,
		def.Spec.OCITag,
		comp.Namespace,
		creds,
		false,
		values,
	)
	if err != nil {
		return nil, err
	}
	if b.CacheDir != "" {
		mgr.CacheDir = b.CacheDir
	}
	return New(mgr), nil
}

// BuildForApply mirrors the existing handler.go behaviour of constructing a
// no-op Timoni Manager (insecure=true, empty source/version) that the drift
// sweep uses solely to call Unmarshal + ApplyObject.
func (b *Builder) BuildForApply(ctx context.Context, comp *conurev1alpha1.Component) (render.Engine, error) {
	mgr, err := tmod.NewManager(ctx, comp.Name, "", "", comp.Namespace, "", true, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return New(mgr), nil
}
