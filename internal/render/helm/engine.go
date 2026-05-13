// Package helm adapts Helm v4 chart rendering to the render.Engine interface.
//
// Conure renders Helm charts via the pure SDK templating path (engine.Render);
// release state is tracked in the Component's apply-sets annotation, not in
// Helm's Secret/ConfigMap storage. Apply happens through internal/render/apply,
// so Helm and Timoni share drift-detection semantics.
//
// Limitations (MVP):
//   - Helm hooks (helm.sh/hook annotation) are filtered out.
//   - The `lookup` template function is not enabled.
//   - Capabilities are static (DefaultCapabilities + HelmEngineSpec overrides).
package helm

import (
	"context"
	"encoding/json"
	"fmt"

	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/coffeenights/conure/internal/render"
	"github.com/coffeenights/conure/internal/render/apply"
)

// Engine is a render.Engine backed by a loaded Helm chart and the controller's
// shared apply manager.
type Engine struct {
	chart   *chartv2.Chart
	values  map[string]interface{}
	release chartcommon.ReleaseOptions
	caps    *chartcommon.Capabilities
	digest  string
	applier *apply.Manager
}

// Compile-time interface assertion.
var _ render.Engine = (*Engine)(nil)

func (e *Engine) Render(ctx context.Context) ([]render.ResourceSet, error) {
	objs, err := renderChart(e.chart, e.values, e.release, e.caps)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, nil
	}
	return []render.ResourceSet{{Name: e.chart.Name(), Objects: objs}}, nil
}

func (e *Engine) Digest() string { return e.digest }

// MarshalApplySets emits a plain JSON array of ResourceSets — the same on-wire
// shape Timoni uses, so the apply-set envelope only needs an `engine` tag to
// dispatch read paths correctly.
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
	if e.applier == nil {
		return nil, fmt.Errorf("helm engine has no apply manager configured")
	}
	return e.applier.Apply(ctx, obj, force)
}
