package fieldroles

import "testing"

func webserviceRoles() map[string]string {
	return map[string]string{
		RoleSourceType:      "source.sourceType",
		RoleImageRepository: "source.ociRepository",
		RoleImageTag:        "source.tag",
		RoleGitRepository:   "source.gitRepository",
		RoleGitBranch:       "source.gitBranch",
		RoleBuildTool:       "source.buildTool",
		RoleBuildLocation:   "source.buildLocation",
		RoleBuildDockerfile: "source.dockerfilePath",
	}
}

func TestGet_NestedValue(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{
		"source": map[string]interface{}{
			"ociRepository": "ghcr.io/me/app",
			"tag":           "v1",
		},
	}
	got, ok, err := r.Get(values, RoleImageRepository)
	if err != nil || !ok || got != "ghcr.io/me/app" {
		t.Fatalf("Get(image.repository) = %q, ok=%v, err=%v; want ghcr.io/me/app", got, ok, err)
	}
}

func TestGet_MissingLeafIsNotAnError(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{}}
	got, ok, err := r.Get(values, RoleImageTag)
	if err != nil {
		t.Fatalf("unset leaf must not error, got %v", err)
	}
	if ok || got != "" {
		t.Fatalf("unset leaf: want (\"\", false), got (%q, %v)", got, ok)
	}
}

func TestGet_UndeclaredRoleErrors(t *testing.T) {
	r := New(true, map[string]string{RoleImageRepository: "source.ociRepository"})
	if _, _, err := r.Get(map[string]interface{}{}, RoleGitRepository); err == nil {
		t.Fatal("reading an undeclared role must error (no fallback)")
	}
}

func TestGet_IntermediateNotAnObject(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": "not-a-map"}
	if _, _, err := r.Get(values, RoleImageTag); err == nil {
		t.Fatal("traversing a non-object intermediate must error")
	}
}

func TestGet_LeafWrongType(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{"tag": 42}}
	if _, _, err := r.Get(values, RoleImageTag); err == nil {
		t.Fatal("non-string leaf must error")
	}
}

func TestSet_CreatesNestedMaps(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{}
	if err := r.Set(values, RoleImageRepository, "ghcr.io/me/app"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := r.Set(values, RoleImageTag, "sha-abc"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	src, ok := values["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("source not created as a map: %#v", values["source"])
	}
	if src["ociRepository"] != "ghcr.io/me/app" || src["tag"] != "sha-abc" {
		t.Fatalf("Set wrote wrong values: %#v", src)
	}
}

func TestSet_PreservesSiblings(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{
		"source": map[string]interface{}{
			"command":   []string{"run"},
			"gitBranch": "main",
		},
	}
	if err := r.Set(values, RoleImageTag, "v2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	src := values["source"].(map[string]interface{})
	if _, ok := src["command"]; !ok {
		t.Fatal("Set dropped sibling key 'command'")
	}
	if src["gitBranch"] != "main" || src["tag"] != "v2" {
		t.Fatalf("Set clobbered siblings: %#v", src)
	}
}

func TestSet_IntermediateOccupiedByScalarErrors(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": "scalar"}
	if err := r.Set(values, RoleImageTag, "v1"); err == nil {
		t.Fatal("Set must refuse to overwrite a scalar intermediate")
	}
}

func TestSet_UndeclaredRoleErrors(t *testing.T) {
	r := New(true, map[string]string{})
	if err := r.Set(map[string]interface{}{}, RoleImageTag, "v1"); err == nil {
		t.Fatal("Set on an undeclared role must error")
	}
}

func TestSourceType_Git(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{"sourceType": "git"}}
	got, err := r.SourceType(values)
	if err != nil || got != SourceTypeGit {
		t.Fatalf("SourceType = %q, err=%v; want git", got, err)
	}
}

func TestSourceType_OCI(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{"sourceType": "oci"}}
	got, err := r.SourceType(values)
	if err != nil || got != SourceTypeOCI {
		t.Fatalf("SourceType = %q, err=%v; want oci", got, err)
	}
}

func TestSourceType_NotBuildable(t *testing.T) {
	r := New(false, webserviceRoles())
	if _, err := r.SourceType(map[string]interface{}{}); err == nil {
		t.Fatal("SourceType on a non-buildable definition must error")
	}
}

func TestSourceType_UnsetValue(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{}}
	if _, err := r.SourceType(values); err == nil {
		t.Fatal("unset sourceType must error")
	}
}

func TestSourceType_OutsideVocabulary(t *testing.T) {
	r := New(true, webserviceRoles())
	values := map[string]interface{}{"source": map[string]interface{}{"sourceType": "svn"}}
	if _, err := r.SourceType(values); err == nil {
		t.Fatal("a sourceType outside git/oci must error")
	}
}

func TestNew_CopiesRolesMap(t *testing.T) {
	orig := map[string]string{RoleImageTag: "source.tag"}
	r := New(true, orig)
	orig[RoleImageTag] = "mutated.path"
	p, err := r.Path(RoleImageTag)
	if err != nil || p != "source.tag" {
		t.Fatalf("resolver must snapshot the roles map; got %q, err=%v", p, err)
	}
}

func TestGetOptional_UndeclaredRoleIsNotAnError(t *testing.T) {
	// A definition that never declares the credential role: absence must
	// read as "public, no auth", not the hard error Get would give.
	r := New(true, webserviceRoles())
	got, err := r.GetOptional(map[string]interface{}{}, RoleGitCredentialRef)
	if err != nil {
		t.Fatalf("undeclared optional role must not error, got %v", err)
	}
	if got != "" {
		t.Fatalf("undeclared optional role: want \"\", got %q", got)
	}
}

func TestGetOptional_DeclaredButUnsetIsEmpty(t *testing.T) {
	r := New(true, map[string]string{RoleGitCredentialRef: "source.gitCredentialRef"})
	got, err := r.GetOptional(map[string]interface{}{"source": map[string]interface{}{}}, RoleGitCredentialRef)
	if err != nil {
		t.Fatalf("declared-but-unset optional role must not error, got %v", err)
	}
	if got != "" {
		t.Fatalf("declared-but-unset: want \"\", got %q", got)
	}
}

func TestGetOptional_DeclaredAndSetReturnsValue(t *testing.T) {
	r := New(true, map[string]string{RoleImageCredentialRef: "source.imageCredentialRef"})
	values := map[string]interface{}{"source": map[string]interface{}{"imageCredentialRef": "my-ghcr"}}
	got, err := r.GetOptional(values, RoleImageCredentialRef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-ghcr" {
		t.Fatalf("want %q, got %q", "my-ghcr", got)
	}
}

func TestGetOptional_StructuralErrorStillPropagates(t *testing.T) {
	// Declared role whose path traverses a non-object: this is a real
	// misdeclaration, not "absent" — it must still error even though the
	// role is optional.
	r := New(true, map[string]string{RoleGitCredentialRef: "source.gitCredentialRef"})
	values := map[string]interface{}{"source": "not-a-map"}
	if _, err := r.GetOptional(values, RoleGitCredentialRef); err == nil {
		t.Fatal("optional role with a non-object intermediate must still error")
	}

	// Declared role whose leaf is the wrong type must still error.
	r2 := New(true, map[string]string{RoleImageCredentialRef: "source.ref"})
	v2 := map[string]interface{}{"source": map[string]interface{}{"ref": 7}}
	if _, err := r2.GetOptional(v2, RoleImageCredentialRef); err == nil {
		t.Fatal("optional role with a non-string leaf must still error")
	}
}
