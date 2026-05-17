package timoni

import (
	"encoding/json"
	"reflect"
	"testing"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/fieldroles"
	timoniinternal "github.com/coffeenights/conure/internal/timoni"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStripCredentialRoles(t *testing.T) {
	tests := []struct {
		name       string
		values     map[string]interface{}
		fieldRoles map[string]string
		want       map[string]interface{}
	}{
		{
			// The exact shape from the reported failure: the definition
			// declares git.credentialRef -> source.gitCredentialRef and the
			// component set it alongside real source config. Only the
			// credential leaf is removed; siblings under source survive.
			name: "removes declared git credentialRef, keeps sibling source fields",
			values: map[string]interface{}{
				"source": map[string]interface{}{
					"gitCredentialRef": "my-org-git-cred",
					"gitRepository":    "https://github.com/me/app",
					"gitBranch":        "main",
				},
				"image": map[string]interface{}{"repository": "ghcr.io/me/app", "tag": "v1"},
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitCredentialRef: "source.gitCredentialRef",
			},
			want: map[string]interface{}{
				"source": map[string]interface{}{
					"gitRepository": "https://github.com/me/app",
					"gitBranch":     "main",
				},
				"image": map[string]interface{}{"repository": "ghcr.io/me/app", "tag": "v1"},
			},
		},
		{
			// When the credential ref was the only key under its parent, the
			// now-empty parent map is pruned so it does not survive as an
			// empty struct and trip the module's closed #config.
			name: "prunes parent map left empty after deletion",
			values: map[string]interface{}{
				"source": map[string]interface{}{"gitCredentialRef": "c"},
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitCredentialRef: "source.gitCredentialRef",
			},
			want: map[string]interface{}{},
		},
		{
			name: "strips both git and image credential roles at distinct paths",
			values: map[string]interface{}{
				"git":   map[string]interface{}{"credentialRef": "g", "repository": "r"},
				"image": map[string]interface{}{"credentialRef": "i", "repository": "ir"},
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitCredentialRef:   "git.credentialRef",
				fieldroles.RoleImageCredentialRef: "image.credentialRef",
			},
			want: map[string]interface{}{
				"git":   map[string]interface{}{"repository": "r"},
				"image": map[string]interface{}{"repository": "ir"},
			},
		},
		{
			name: "role declared but value unset is a no-op",
			values: map[string]interface{}{
				"source": map[string]interface{}{"gitRepository": "r"},
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitCredentialRef: "source.gitCredentialRef",
			},
			want: map[string]interface{}{
				"source": map[string]interface{}{"gitRepository": "r"},
			},
		},
		{
			name: "definition does not declare the credential role - no-op",
			values: map[string]interface{}{
				"source": map[string]interface{}{"gitCredentialRef": "should-stay"},
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitRepository: "source.gitRepository",
			},
			want: map[string]interface{}{
				"source": map[string]interface{}{"gitCredentialRef": "should-stay"},
			},
		},
		{
			name:       "nil fieldRoles is a no-op",
			values:     map[string]interface{}{"source": map[string]interface{}{"a": "b"}},
			fieldRoles: nil,
			want:       map[string]interface{}{"source": map[string]interface{}{"a": "b"}},
		},
		{
			// The declared path traverses a segment that is not a map in the
			// actual values: the path was never really set this way, so leave
			// values untouched rather than panic.
			name: "declared path does not match values shape - no-op",
			values: map[string]interface{}{
				"source": "not-a-map",
			},
			fieldRoles: map[string]string{
				fieldroles.RoleGitCredentialRef: "source.gitCredentialRef",
			},
			want: map[string]interface{}{
				"source": "not-a-map",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stripCredentialRoles(tt.values, tt.fieldRoles)
			if !reflect.DeepEqual(tt.values, tt.want) {
				t.Errorf("stripCredentialRoles() =\n  %#v\nwant\n  %#v", tt.values, tt.want)
			}
		})
	}
}

// TestTimoniValuesStripsCredentialsUnderWrappedShape proves the strip happens
// on the unwrapped values map (paths are relative to the component's values),
// while the final result is still nested under the "values" key the Timoni
// module expects.
func TestTimoniValuesStripsCredentialsUnderWrappedShape(t *testing.T) {
	raw, err := json.Marshal(map[string]interface{}{
		"source": map[string]interface{}{
			"gitCredentialRef": "my-org-git-cred",
			"gitRepository":    "https://github.com/me/app",
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	comp := &conurev1alpha1.Component{}
	comp.Spec.Values = &runtime.RawExtension{Raw: raw}

	got, err := timoniValues(comp, map[string]string{
		fieldroles.RoleGitCredentialRef: "source.gitCredentialRef",
	})
	if err != nil {
		t.Fatalf("timoniValues: %v", err)
	}

	// Get() nests the (stripped) values under the "values" key as a
	// *timoniinternal.Values; unwrap it to its underlying map to assert.
	wrapped, ok := got["values"].(*timoniinternal.Values)
	if !ok {
		t.Fatalf("timoniValues() top-level shape = %#v, want map under \"values\" key", got)
	}
	inner := map[string]interface{}(*wrapped)
	want := map[string]interface{}{
		"source": map[string]interface{}{
			"gitRepository": "https://github.com/me/app",
		},
	}
	if !reflect.DeepEqual(inner, want) {
		t.Errorf("timoniValues() inner values =\n  %#v\nwant\n  %#v", inner, want)
	}
}
