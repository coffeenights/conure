// Package fieldroles resolves conure's well-known field "roles" to concrete
// locations inside a component's free-form values map.
//
// Each ComponentDefinition declares its own #Config schema (the schema
// itself lives in an external Timoni/Helm module, not in this repo), so
// conure cannot assume where the image, git, or build fields live. The
// definition's spec.fieldRoles maps conure's fixed role vocabulary to
// dotted paths into that definition's values. This package is the single
// place that vocabulary and the path traversal are implemented, shared by
// the API server (writing the built image back into values) and the CLI
// (reading the build spec to decide what `conure deploy` should do).
//
// There is no default/fallback path set. A role that is read when it is
// needed but is not declared by the definition is a hard error. The
// platform is pre-1.0; definitions are expected to declare the roles their
// components use.
package fieldroles

import (
	"fmt"
	"strings"
)

// Well-known role keys. These strings are conure's stable vocabulary and
// the keys of ComponentDefinitionSpec.FieldRoles. They are NOT paths; the
// map's *values* are the dotted paths into a component's values.
const (
	// RoleSourceType is the build-vs-pull discriminator. The value found at
	// its path must be one of SourceTypeGit or SourceTypeOCI.
	RoleSourceType = "sourceType"

	// RoleImageRepository / RoleImageTag locate the application image
	// (the image conure builds or deploys), distinct from the
	// ComponentDefinition's own module artifact (spec.ociRepository).
	RoleImageRepository = "image.repository"
	RoleImageTag        = "image.tag"

	// RoleGitRepository / RoleGitBranch locate the git source for a
	// remote build. Read only when the discriminator resolves to git.
	RoleGitRepository = "git.repository"
	RoleGitBranch     = "git.branch"

	// RoleBuildTool / RoleBuildLocation / RoleBuildDockerfile locate the
	// build knobs. Read only when the discriminator resolves to git.
	RoleBuildTool       = "build.tool"
	RoleBuildLocation   = "build.location"
	RoleBuildDockerfile = "build.dockerfile"
)

// Discriminator values for RoleSourceType. conure owns this vocabulary;
// definitions do not get to rename them.
const (
	SourceTypeGit = "git"
	SourceTypeOCI = "oci"
)

// Resolver answers "where does role X live in this component's values?"
// for one ComponentDefinition. Construct via New.
type Resolver struct {
	buildable bool
	roles     map[string]string
}

// New builds a Resolver from a ComponentDefinition's buildable flag and
// fieldRoles map. The map is copied so later mutation of the caller's map
// can't change resolution. A nil map is allowed (every role lookup then
// errors) — that is valid for definitions with no image/build concept.
func New(buildable bool, roles map[string]string) *Resolver {
	cp := make(map[string]string, len(roles))
	for k, v := range roles {
		cp[k] = v
	}
	return &Resolver{buildable: buildable, roles: cp}
}

// Buildable reports whether the definition can build an image at all.
// When false, conure must not attempt a build and `conure deploy` is
// promote-only regardless of any per-component sourceType value.
func (r *Resolver) Buildable() bool {
	return r.buildable
}

// Path returns the dotted values path declared for role, or an error
// naming the role if the definition did not declare it. Callers should
// only call this for roles the current operation actually needs.
func (r *Resolver) Path(role string) (string, error) {
	p, ok := r.roles[role]
	if !ok || p == "" {
		return "", fmt.Errorf("component definition does not declare a path for the %q field role; add it to the definition's spec.fieldRoles", role)
	}
	return p, nil
}

// Get reads the string value at role's declared path. It errors if the
// role is undeclared, if an intermediate path segment is not a nested
// map, or if the final value is present but not a string. A missing leaf
// (path declared, but the component's values don't set it) returns
// ("", false, nil) so callers can distinguish "unset" from "misdeclared".
func (r *Resolver) Get(values map[string]interface{}, role string) (string, bool, error) {
	path, err := r.Path(role)
	if err != nil {
		return "", false, err
	}
	segs := strings.Split(path, ".")
	var cur interface{} = values
	for i, seg := range segs {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return "", false, fmt.Errorf("role %q: path %q traverses %q which is not an object", role, path, strings.Join(segs[:i], "."))
		}
		next, present := m[seg]
		if !present {
			return "", false, nil
		}
		cur = next
	}
	if cur == nil {
		return "", false, nil
	}
	s, ok := cur.(string)
	if !ok {
		return "", false, fmt.Errorf("role %q: value at %q is %T, want string", role, path, cur)
	}
	return s, true, nil
}

// Set writes val at role's declared path, creating intermediate maps as
// needed. It errors if the role is undeclared or if an existing
// intermediate segment is occupied by a non-map value (overwriting it
// would silently drop sibling data).
func (r *Resolver) Set(values map[string]interface{}, role, val string) error {
	path, err := r.Path(role)
	if err != nil {
		return err
	}
	segs := strings.Split(path, ".")
	cur := values
	for _, seg := range segs[:len(segs)-1] {
		existing, present := cur[seg]
		if !present || existing == nil {
			next := map[string]interface{}{}
			cur[seg] = next
			cur = next
			continue
		}
		next, ok := existing.(map[string]interface{})
		if !ok {
			return fmt.Errorf("role %q: cannot descend into %q at path %q: existing value is %T, not an object", role, seg, path, existing)
		}
		cur = next
	}
	cur[segs[len(segs)-1]] = val
	return nil
}

// SourceType resolves the build-vs-pull discriminator for a component.
// Returns SourceTypeGit or SourceTypeOCI. It errors if the component is
// not buildable (callers should check Buildable first and skip building),
// if the sourceType role is undeclared, if the value is unset, or if the
// value is outside conure's fixed vocabulary.
func (r *Resolver) SourceType(values map[string]interface{}) (string, error) {
	if !r.buildable {
		return "", fmt.Errorf("component definition is not buildable; sourceType is meaningless")
	}
	v, ok, err := r.Get(values, RoleSourceType)
	if err != nil {
		return "", err
	}
	if !ok || v == "" {
		return "", fmt.Errorf("component values do not set the sourceType discriminator (role %q at the definition's declared path)", RoleSourceType)
	}
	switch v {
	case SourceTypeGit, SourceTypeOCI:
		return v, nil
	default:
		return "", fmt.Errorf("sourceType %q is not a valid conure discriminator (want %q or %q)", v, SourceTypeGit, SourceTypeOCI)
	}
}
