package helm

import (
	"fmt"
	"path"
	"sort"
	"strings"

	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chartutil "helm.sh/helm/v4/pkg/chart/common/util"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	helmengine "helm.sh/helm/v4/pkg/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// helmHookAnnotation marks Helm hook resources (helm.sh/hook). Conure does not
// support hook lifecycle semantics — there is no discrete install/upgrade
// event, just continuous reconvergence — so any object carrying this
// annotation is dropped at render time. Document this limitation in the user
// docs for the Helm engine.
const helmHookAnnotation = "helm.sh/hook"

// renderChart executes the Helm SDK template engine and returns the rendered
// objects in apply order. The flow:
//  1. Validate user values against values.schema.json (when the chart ships one).
//  2. Coalesce user values into chart defaults.
//  3. Build the render-values tree (.Values, .Chart, .Release, .Capabilities, .Files).
//  4. Run engine.Render to get filename -> rendered YAML.
//  5. Prepend CRDs from crds/, drop NOTES.txt, drop helm.sh/hook objects.
//  6. Sort filenames lexicographically for deterministic ordering.
//  7. Split YAML docs, parse to *unstructured.Unstructured, flatten lists.
func renderChart(ch *chartv2.Chart, userVals map[string]interface{}, rel chartcommon.ReleaseOptions, caps *chartcommon.Capabilities) ([]*unstructured.Unstructured, error) {
	if err := ch.Validate(); err != nil {
		return nil, fmt.Errorf("chart validation: %w", err)
	}
	if err := chartutil.ValidateAgainstSchema(ch, userVals); err != nil {
		return nil, fmt.Errorf("values schema validation: %w", err)
	}
	if _, err := chartutil.CoalesceValues(ch, userVals); err != nil {
		return nil, fmt.Errorf("coalescing values: %w", err)
	}
	vals, err := chartutil.ToRenderValues(ch, userVals, rel, caps)
	if err != nil {
		return nil, fmt.Errorf("building render values: %w", err)
	}

	rendered, err := helmengine.Render(ch, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering chart: %w", err)
	}

	// Helm returns a map of filename -> rendered YAML. Sort filenames for
	// deterministic apply order, which keeps the apply-set hash stable across
	// reconciles.
	filenames := make([]string, 0, len(rendered))
	for name := range rendered {
		if strings.EqualFold(path.Base(name), "NOTES.txt") {
			continue
		}
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)

	objs := make([]*unstructured.Unstructured, 0, len(filenames))

	// Prepend CRDs: Helm intentionally excludes them from engine.Render
	// because they are installed differently when run through `helm install`.
	// We treat them as ordinary manifests applied via SSA.
	for _, crd := range ch.CRDObjects() {
		parsed, err := parseYAMLObjects(crd.File.Data)
		if err != nil {
			return nil, fmt.Errorf("parsing CRD %s: %w", crd.Filename, err)
		}
		objs = append(objs, parsed...)
	}

	for _, name := range filenames {
		body := rendered[name]
		if strings.TrimSpace(body) == "" {
			continue
		}
		parsed, err := parseYAMLObjects([]byte(body))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		for _, o := range parsed {
			if _, isHook := o.GetAnnotations()[helmHookAnnotation]; isHook {
				continue
			}
			objs = append(objs, o)
		}
	}
	return objs, nil
}

// parseYAMLObjects splits a YAML document stream and parses each non-empty
// document into an *unstructured.Unstructured. Top-level kind: List documents
// are flattened into their items so callers always receive a flat slice.
func parseYAMLObjects(data []byte) ([]*unstructured.Unstructured, error) {
	docs := splitYAMLDocs(data)
	out := make([]*unstructured.Unstructured, 0, len(docs))
	for _, doc := range docs {
		if len(strings.TrimSpace(string(doc))) == 0 {
			continue
		}
		// Convert YAML -> JSON so json.Unmarshal can populate the
		// unstructured map directly. sigs.k8s.io/yaml handles the conversion
		// and tolerates Helm's templated output.
		jsonBytes, err := yaml.YAMLToJSON(doc)
		if err != nil {
			return nil, err
		}
		// Empty templated docs render as `null` after YAMLToJSON.
		if string(jsonBytes) == "null" {
			continue
		}
		obj := &unstructured.Unstructured{}
		if err := obj.UnmarshalJSON(jsonBytes); err != nil {
			return nil, err
		}
		if obj.GetKind() == "" {
			continue
		}
		if obj.GetKind() == "List" {
			items, _, err := unstructured.NestedSlice(obj.Object, "items")
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				out = append(out, &unstructured.Unstructured{Object: m})
			}
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

// splitYAMLDocs splits a YAML stream on `---` document separators. Goyaml's
// own decoder would also work but introduces complications with raw `---`
// strings in template output; a textual split is sufficient because Helm's
// engine itself emits manifests as a `---`-separated stream.
func splitYAMLDocs(data []byte) [][]byte {
	parts := strings.Split(string(data), "\n---")
	out := make([][]byte, 0, len(parts))
	for i, p := range parts {
		// Trim the leading "---" that the first document inherits when the
		// stream begins with a separator.
		if i == 0 {
			p = strings.TrimPrefix(p, "---")
		}
		out = append(out, []byte(p))
	}
	return out
}
