package timoni

import (
	"github.com/fluxcd/pkg/ssa"
	"github.com/stefanprodan/timoni/pkg/module"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ModuleManager abstracts the Timoni module.Manager methods used by controllers,
// enabling unit testing without OCI registry access.
type ModuleManager interface {
	GetApplySets() ([]module.ResourceSet, error)
	GetDigest() string
	MarshalApplySets(sets []module.ResourceSet) ([]byte, error)
	UnmarshalApplySets(data []byte) ([]module.ResourceSet, error)
	ApplyObject(resource *unstructured.Unstructured, force bool) (*ssa.ChangeSetEntry, error)
}
