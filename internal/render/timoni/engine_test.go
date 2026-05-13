package timoni

import (
	"testing"

	timoniapi "github.com/stefanprodan/timoni/api/v1alpha1"
	tmod "github.com/stefanprodan/timoni/pkg/module"
)

// TestEngine_DigestDelegatesToManager is the adapter-level mirror of the
// handler digest gate: the conure handler trusts Digest() to surface the OCI
// manifest digest that Timoni's fetcher resolved. We construct a Manager with
// a Module.Digest set (the field the upstream fetcher populates after a pull)
// and verify the adapter's Digest() returns it verbatim.
func TestEngine_DigestDelegatesToManager(t *testing.T) {
	const want = "sha256:0123456789abcdef"
	mgr := &tmod.Manager{
		Module: &timoniapi.ModuleReference{Digest: want},
	}
	e := New(mgr)
	if got := e.Digest(); got != want {
		t.Fatalf("Digest() = %q, want %q", got, want)
	}
}

// TestEngine_DigestEmptyWhenNoManager confirms the no-pull path (the engine
// constructed for BuildForApply with a fresh Manager that hasn't been built)
// returns an empty digest rather than panicking — the handler then skips the
// gate when OCIDigest is unset on the ComponentDefinition.
func TestEngine_DigestEmptyWhenNoManager(t *testing.T) {
	e := New(&tmod.Manager{})
	if got := e.Digest(); got != "" {
		t.Fatalf("Digest() = %q, want empty", got)
	}
}
