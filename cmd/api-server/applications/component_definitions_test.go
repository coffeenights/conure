package applications

import (
	"testing"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

// TestApplyRequest_SparseMergePreservesBase is the regression for the
// destructive-set bug: `set webservice --oci-tag newtag` used to send the
// flag's zero value for every other field, and applyRequest overwrote them,
// silently wiping field_roles/buildable on the override. Now only the fields
// the caller actually sent (non-nil pointers) are applied; everything else
// keeps the merge base (the inherited default or existing override).
func TestApplyRequest_SparseMergePreservesBase(t *testing.T) {
	base := models.ComponentDefinition{
		Type:          "webservice",
		Engine:        "timoni",
		OCIRepository: "ghcr.io/acme/web",
		OCITag:        "1.0.0",
		Buildable:     true,
		FieldRoles:    map[string]string{"image.repository": "source.ociRepository"},
	}
	cd := base // applyRequest mutates in place; the handler seeds cd from the base

	// Caller passed only --oci-tag.
	applyRequest(&cd, &componentDefinitionRequest{
		Type:   "webservice",
		OCITag: strptr("2.0.0"),
	})

	if cd.OCITag != "2.0.0" {
		t.Fatalf("oci_tag not applied: %q", cd.OCITag)
	}
	if !cd.Buildable {
		t.Errorf("buildable was wiped by a sparse set (the bug)")
	}
	if cd.FieldRoles["image.repository"] != "source.ociRepository" {
		t.Errorf("field_roles were wiped by a sparse set (the bug): %+v", cd.FieldRoles)
	}
	if cd.OCIRepository != "ghcr.io/acme/web" {
		t.Errorf("oci_repository was wiped by a sparse set: %q", cd.OCIRepository)
	}
}

// TestApplyRequest_FullDocumentReplaces covers the --from-file path: a
// complete document populates every field (including explicit zero values),
// so applying it is a full replace — same code path, no mode flag.
func TestApplyRequest_FullDocumentReplaces(t *testing.T) {
	cd := models.ComponentDefinition{
		Type:          "webservice",
		Engine:        "timoni",
		OCIRepository: "ghcr.io/old/web",
		Buildable:     true,
		FieldRoles:    map[string]string{"image.repository": "source.ociRepository"},
	}
	emptyRoles := map[string]string{}
	applyRequest(&cd, &componentDefinitionRequest{
		Type:          "webservice",
		Engine:        "timoni",
		Description:   strptr(""),
		OCIRepository: strptr("ghcr.io/new/web"),
		OCITag:        strptr(""),
		OCIDigest:     strptr(""),
		OCIRegistry:   strptr(""),
		Buildable:     boolptr(false),
		FieldRoles:    &emptyRoles,
	})

	if cd.OCIRepository != "ghcr.io/new/web" {
		t.Errorf("oci_repository not replaced: %q", cd.OCIRepository)
	}
	if cd.Buildable {
		t.Errorf("buildable not cleared by full-document replace")
	}
	if len(cd.FieldRoles) != 0 {
		t.Errorf("field_roles not cleared by full-document replace: %+v", cd.FieldRoles)
	}
}
