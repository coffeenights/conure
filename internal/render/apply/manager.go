// Package apply wires the shared fluxcd/pkg/ssa apply path used by render
// adapters that don't have their own server-side-apply implementation. The
// Timoni adapter delegates to its upstream Manager.ApplyObject instead — that
// path predates this package and retains its own FieldManager identity for
// back-compat with components already deployed in production clusters.
package apply

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// FieldManager is the server-side-apply field manager used by adapters built
// on this package. It is distinct from Timoni's own "timoni" FieldManager so
// the two paths don't conflict over field ownership in mixed-engine clusters.
const FieldManager = "conure"

// OwnerGroup is the label-key prefix that ssa.ResourceManager uses for the
// apply-set ownership labels (e.g. <group>/name, <group>/namespace).
const OwnerGroup = "component.core.conure.io"

// Manager wraps a single ssa.ResourceManager for reuse across reconciles.
// Construct one at controller startup and inject into every engine adapter
// that needs to apply manifests.
type Manager struct {
	rm *ssa.ResourceManager
}

// New builds a Manager from a REST config. The REST config is typically the
// one exposed by the controller-runtime manager (mgr.GetConfig()).
func New(cfg *rest.Config, scheme *runtime.Scheme) (*Manager, error) {
	// Match Timoni's quality-of-service tuning so apply throughput is
	// comparable between the two engines.
	c := rest.CopyConfig(cfg)
	c.QPS = 100
	c.Burst = 300

	httpClient, err := rest.HTTPClientFor(c)
	if err != nil {
		return nil, fmt.Errorf("creating http client: %w", err)
	}
	mapper, err := apiutil.NewDynamicRESTMapper(c, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating rest mapper: %w", err)
	}
	kubeClient, err := client.New(c, client.Options{Mapper: mapper, Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("creating kube client: %w", err)
	}

	poller := polling.NewStatusPoller(kubeClient, mapper, polling.Options{})

	rm := ssa.NewResourceManager(kubeClient, poller, ssa.Owner{
		Field: FieldManager,
		Group: OwnerGroup,
	})
	rm.SetConcurrency(4)
	return &Manager{rm: rm}, nil
}

// Apply performs a single SSA apply via the shared ResourceManager. The
// returned ChangeSetEntry surfaces drift / create / update outcomes through
// fluxcd's standard taxonomy.
func (m *Manager) Apply(ctx context.Context, obj *unstructured.Unstructured, force bool) (*ssa.ChangeSetEntry, error) {
	opts := ssa.DefaultApplyOptions()
	opts.Force = force
	opts.WaitTimeout = 5 * time.Minute
	return m.rm.Apply(ctx, obj, opts)
}
