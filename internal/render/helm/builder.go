package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chartv2loader "helm.sh/helm/v4/pkg/chart/v2/loader"
	helmregistry "helm.sh/helm/v4/pkg/registry"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/render"
	"github.com/coffeenights/conure/internal/render/apply"
)

// Builder is the render.Builder for Helm charts. It is wired up once at
// controller startup with a shared apply.Manager; each Build call creates a
// new registry client (so credentials are scoped to the specific component
// being rendered).
type Builder struct {
	applier *apply.Manager
}

// NewBuilder returns a Helm Builder that applies through the supplied
// apply.Manager.
func NewBuilder(applier *apply.Manager) *Builder {
	return &Builder{applier: applier}
}

// Build pulls the chart artifact from the OCI registry referenced by the
// ComponentDefinition, loads it, and prepares an Engine ready to render.
func (b *Builder) Build(ctx context.Context, def *conurev1alpha1.ComponentDefinition, comp *conurev1alpha1.Component, creds string) (render.Engine, error) {
	ref := strings.TrimPrefix(def.Spec.OCIRepository, "oci://")
	if def.Spec.OCITag != "" {
		ref = ref + ":" + def.Spec.OCITag
	}

	client, err := newRegistryClient(ref, creds)
	if err != nil {
		return nil, fmt.Errorf("building registry client: %w", err)
	}

	pull, err := client.Pull(ref, helmregistry.PullOptWithChart(true))
	if err != nil {
		return nil, fmt.Errorf("pulling chart %s: %w", ref, err)
	}
	if pull.Chart == nil || len(pull.Chart.Data) == 0 {
		return nil, fmt.Errorf("pull result for %s contained no chart layer", ref)
	}

	chart, err := chartv2loader.LoadArchive(bytes.NewReader(pull.Chart.Data))
	if err != nil {
		return nil, fmt.Errorf("loading chart archive: %w", err)
	}

	values, err := decodeValues(comp)
	if err != nil {
		return nil, err
	}

	rel := buildReleaseOptions(def, comp)
	caps := buildCapabilities(def)

	digest := ""
	if pull.Manifest != nil {
		digest = pull.Manifest.Digest
	}

	return &Engine{
		chart:   chart,
		values:  values,
		release: rel,
		caps:    caps,
		digest:  digest,
		applier: b.applier,
	}, nil
}

// BuildForApply returns a render.Engine wired only for the apply path. The
// drift sweep uses it to unmarshal the cached apply-set and re-apply each
// resource — no fetch, no template render.
func (b *Builder) BuildForApply(ctx context.Context, comp *conurev1alpha1.Component) (render.Engine, error) {
	return &Engine{applier: b.applier}, nil
}

// decodeValues converts the Component's Spec.Values RawExtension into a
// plain map. UseNumber is set so chart values that read like ints stay
// JSON numbers (Helm template authors regularly compare against integer
// literals — letting the default decoder float them breaks those checks).
func decodeValues(comp *conurev1alpha1.Component) (map[string]interface{}, error) {
	if comp.Spec.Values == nil || len(comp.Spec.Values.Raw) == 0 {
		return map[string]interface{}{}, nil
	}
	d := json.NewDecoder(bytes.NewReader(comp.Spec.Values.Raw))
	d.UseNumber()
	out := map[string]interface{}{}
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding component values: %w", err)
	}
	return out, nil
}

// buildReleaseOptions wires up .Release.* template inputs. We always render
// in upgrade mode: conure re-renders on every reconcile that needs work, so
// "install" semantics would re-fire one-time bootstrap branches on each loop.
// Charts that gate manifests on IsInstall alone won't render anything — a
// documented MVP limitation.
func buildReleaseOptions(def *conurev1alpha1.ComponentDefinition, comp *conurev1alpha1.Component) chartcommon.ReleaseOptions {
	name := comp.Name
	namespace := comp.Namespace
	if h := def.Spec.Helm; h != nil {
		if h.ReleaseName != "" {
			name = h.ReleaseName
		}
		if h.Namespace != "" {
			namespace = h.Namespace
		}
	}
	return chartcommon.ReleaseOptions{
		Name:      name,
		Namespace: namespace,
		Revision:  1,
		IsInstall: false,
		IsUpgrade: true,
	}
}

// buildCapabilities returns the .Capabilities tree exposed to chart templates.
// MVP uses Helm's DefaultCapabilities and appends any extra APIVersions the
// ComponentDefinition declares. A follow-up may pull live cluster discovery
// to populate this dynamically.
func buildCapabilities(def *conurev1alpha1.ComponentDefinition) *chartcommon.Capabilities {
	caps := chartcommon.DefaultCapabilities.Copy()
	if h := def.Spec.Helm; h != nil {
		if h.KubeVersion != "" {
			if kv, err := chartcommon.ParseKubeVersion(h.KubeVersion); err == nil {
				caps.KubeVersion = *kv
			}
		}
		for _, v := range h.APIVersions {
			caps.APIVersions = append(caps.APIVersions, v)
		}
	}
	return caps
}

// newRegistryClient builds a Helm OCI registry client and logs in to the
// host portion of ref when creds are supplied. Anonymous pulls work too —
// matching the credential-less ComponentDefinition path.
func newRegistryClient(ref string, creds string) (*helmregistry.Client, error) {
	client, err := helmregistry.NewClient()
	if err != nil {
		return nil, err
	}
	if creds == "" {
		return client, nil
	}
	user, pass, ok := strings.Cut(creds, ":")
	if !ok {
		return nil, fmt.Errorf("credentials not in user:password form")
	}
	host := registryHost(ref)
	if err := client.Login(host,
		helmregistry.LoginOptBasicAuth(user, pass),
	); err != nil {
		return nil, fmt.Errorf("registry login to %s: %w", host, err)
	}
	return client, nil
}

// registryHost extracts the host from an OCI reference like
// "ghcr.io/org/chart:tag" or "oci://ghcr.io/org/chart". Mirrors the helper
// in internal/controller/core/component/credentials.go but duplicated here
// to keep the helm adapter self-contained.
func registryHost(ref string) string {
	ref = strings.TrimPrefix(ref, "oci://")
	if i := strings.Index(ref, "/"); i > 0 {
		return ref[:i]
	}
	return ref
}
