package timoni

import (
	"encoding/json"
	"strings"

	conurev1alpha1 "github.com/coffeenights/conure/apis/core/v1alpha1"
	"github.com/coffeenights/conure/internal/fieldroles"
	timoniinternal "github.com/coffeenights/conure/internal/timoni"
)

// timoniValues mirrors the values-coercion logic that the component handler
// previously inlined: marshal the Component's Spec.Values RawExtension to
// JSON, decode with UseNumber so integers stay integers (the default decoder
// would float them and break CUE typing), and wrap under the "values" key
// that Timoni modules expect.
func timoniValues(comp *conurev1alpha1.Component, fieldRoles map[string]string) (map[string]interface{}, error) {
	valuesJSON, err := json.Marshal(comp.Spec.Values)
	if err != nil {
		return nil, err
	}
	values := timoniinternal.Values{}
	d := json.NewDecoder(strings.NewReader(string(valuesJSON)))
	d.UseNumber()
	if err := d.Decode(&values); err != nil {
		return nil, err
	}
	// Strip conure-internal credential paths from the unwrapped values map
	// (the fieldRoles paths are relative to the component's values, before
	// Get() nests them under the "values" key the module expects).
	stripCredentialRoles(values, fieldRoles)
	return values.Get(), nil
}

// credentialRoles are the field roles whose value is a conure-internal logical
// credential name that the rendering module has NO schema for and must never
// see — its closed #config rejects undeclared fields ("field not allowed").
// The API build path reads these to mount git Secrets in the build Job; the
// value is plumbing, not module config, so it is stripped before render.
//
// Only git.credentialRef qualifies. image.credentialRef is intentionally NOT
// here: a definition maps it onto a real module field (e.g.
// source.imagePullSecrets) whose value — the credential/Secret name — the
// template legitimately consumes. Conure resolves that name into a cluster
// Secret on the deploy path (EnsurePullSecret) but must leave the value in
// place so Timoni's rendered Deployment can reference it. Stripping it would
// silently drop the workload's imagePullSecrets.
var credentialRoles = []string{
	fieldroles.RoleGitCredentialRef,
}

// stripCredentialRoles removes, from values, the dotted paths that fieldRoles
// declares for the conure-internal credential roles, then prunes any parent
// map left empty by the deletion (so e.g. "source" does not survive as an
// empty struct and trip the module's #config). It is path-driven off the
// ComponentDefinition's own fieldRoles map, so it matches whatever path that
// definition declared (source.gitCredentialRef, git.credentialRef, …) and
// stays in lockstep with the path the API build path consumes. A nil or empty
// fieldRoles map, or roles it does not declare, are no-ops — these roles are
// optional by contract.
func stripCredentialRoles(values map[string]interface{}, fieldRoles map[string]string) {
	if len(values) == 0 || len(fieldRoles) == 0 {
		return
	}
	for _, role := range credentialRoles {
		path := fieldRoles[role]
		if path == "" {
			continue
		}
		deletePath(values, strings.Split(path, "."))
	}
}

// deletePath removes the leaf named by segs from the nested map m, then
// unwinds: any map along the path that became empty as a result is itself
// removed from its parent. It does nothing if any segment is missing or an
// intermediate segment is not a map (the path was never set, or the values
// shape does not match the declared path — neither is an error here).
func deletePath(m map[string]interface{}, segs []string) {
	if len(segs) == 0 {
		return
	}
	if len(segs) == 1 {
		delete(m, segs[0])
		return
	}
	child, ok := m[segs[0]].(map[string]interface{})
	if !ok {
		return
	}
	deletePath(child, segs[1:])
	if len(child) == 0 {
		delete(m, segs[0])
	}
}
