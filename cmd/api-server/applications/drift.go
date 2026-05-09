package applications

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
)

// DriftEntry is a single point of divergence between the live K8s
// Component.Spec.Values and the last-deployed revision values stored in
// Mongo. Path is dot-separated (`spec.replicas`, `image`, `env.0.value`).
type DriftEntry struct {
	Path     string      `json:"path"`
	Live     interface{} `json:"live,omitempty"`
	Deployed interface{} `json:"deployed,omitempty"`
}

// DriftReport is the full structured output of comparing live values with
// the last-deployed revision. `Drifted` is true iff Diff is non-empty.
type DriftReport struct {
	Drifted bool         `json:"drifted"`
	Diff    []DriftEntry `json:"diff"`
}

// LiveValuesFromComponent unwraps the JSON-encoded Spec.Values from a live
// Component CRD into a map. A missing or empty Values field returns an empty
// map rather than an error — that's a valid state for an in-flight component.
func LiveValuesFromComponent(comp *conurev1alpha1.Component) (map[string]interface{}, error) {
	if comp == nil || comp.Spec.Values == nil || len(comp.Spec.Values.Raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal(comp.Spec.Values.Raw, &values); err != nil {
		return nil, fmt.Errorf("unmarshaling live component values: %w", err)
	}
	if values == nil {
		return map[string]interface{}{}, nil
	}
	return values, nil
}

// ComputeDrift compares two value maps and returns a structured diff. Both
// maps are normalized through JSON round-trip first so that numeric and
// type-tag differences (e.g. int vs float64 from Mongo's bson) don't show up
// as spurious drift.
func ComputeDrift(live, deployed map[string]interface{}) (DriftReport, error) {
	liveNorm, err := normalize(live)
	if err != nil {
		return DriftReport{}, err
	}
	deployedNorm, err := normalize(deployed)
	if err != nil {
		return DriftReport{}, err
	}

	var diff []DriftEntry
	walkDiff("", liveNorm, deployedNorm, &diff)
	sort.Slice(diff, func(i, j int) bool { return diff[i].Path < diff[j].Path })
	return DriftReport{Drifted: len(diff) > 0, Diff: diff}, nil
}

// normalize JSON-roundtrips a value so all comparisons happen on a single
// canonical representation (numbers as float64, no bson type tags, etc.).
func normalize(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func walkDiff(path string, live, deployed interface{}, out *[]DriftEntry) {
	if reflect.DeepEqual(live, deployed) {
		return
	}
	switch lv := live.(type) {
	case map[string]interface{}:
		dv, ok := deployed.(map[string]interface{})
		if !ok {
			*out = append(*out, DriftEntry{Path: path, Live: live, Deployed: deployed})
			return
		}
		keys := map[string]struct{}{}
		for k := range lv {
			keys[k] = struct{}{}
		}
		for k := range dv {
			keys[k] = struct{}{}
		}
		for k := range keys {
			walkDiff(joinPath(path, k), lv[k], dv[k], out)
		}
	case []interface{}:
		dv, ok := deployed.([]interface{})
		if !ok {
			*out = append(*out, DriftEntry{Path: path, Live: live, Deployed: deployed})
			return
		}
		n := len(lv)
		if len(dv) > n {
			n = len(dv)
		}
		for i := 0; i < n; i++ {
			var lItem, dItem interface{}
			if i < len(lv) {
				lItem = lv[i]
			}
			if i < len(dv) {
				dItem = dv[i]
			}
			walkDiff(fmt.Sprintf("%s.%d", path, i), lItem, dItem, out)
		}
	default:
		*out = append(*out, DriftEntry{Path: path, Live: live, Deployed: deployed})
	}
}

func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}
