// Package system exposes cluster-level metadata that the CLI needs to make
// build decisions: target platform (so M-series Macs cross-build for amd64
// clusters), Kubernetes version, and whether remote builds are enabled.
package system

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"sort"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// Handler serves system-info responses. Kube may be nil; we lazily construct
// a clientset on demand (same pattern as applications.ApiHandler) so unit
// tests can hit the route without wiring a fake clientset.
type Handler struct {
	Kube *k8sUtils.GenericClientset
}

func NewHandler(kube *k8sUtils.GenericClientset) *Handler {
	return &Handler{Kube: kube}
}

// InfoResponse is the JSON shape returned by GET /system/info.
type InfoResponse struct {
	// Platform is the dominant node architecture in the cluster, in the
	// Docker "os/arch" form (e.g. "linux/amd64", "linux/arm64"). The CLI
	// uses this as the default target for local builds so a developer on
	// an M-series Mac cross-builds for an amd64 cluster instead of
	// shipping an unrunnable image.
	Platform string `json:"platform"`
	// KubernetesVersion is the server's GitVersion (e.g. "v1.32.0"). Useful
	// for the CLI to flag obvious incompatibilities but otherwise advisory.
	KubernetesVersion string `json:"kubernetes_version"`
}

// GetInfo returns the cluster's dominant node platform and Kubernetes
// version. Detection strategy:
//
//   - List nodes, read `kubernetes.io/arch` (and `kubernetes.io/os`) labels.
//   - Pick the most common (arch,os) pair across the listed nodes. Ties are
//     broken by lexicographic order so the result is deterministic.
//   - Fall back to runtime.GOARCH/runtime.GOOS only when the listing fails
//     or returns no nodes, which is essentially impossible in a real
//     cluster and indicates a misconfigured kubeconfig in tests.
func (h *Handler) GetInfo(c *gin.Context) {
	clientset, err := h.kubeClient()
	if err != nil {
		log.Printf("system/info: clientset: %v", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	platform := detectPlatform(c.Request.Context(), clientset)
	kubeVer := detectKubeVersion(clientset)

	c.JSON(http.StatusOK, InfoResponse{
		Platform:          platform,
		KubernetesVersion: kubeVer,
	})
}

func (h *Handler) kubeClient() (*k8sUtils.GenericClientset, error) {
	if h.Kube != nil {
		return h.Kube, nil
	}
	return k8sUtils.GetClientset()
}

// detectPlatform walks every node and returns the most common
// "<os>/<arch>" pair. Falls back to the API server process arch on any
// error or empty result.
func detectPlatform(ctx context.Context, clientset *k8sUtils.GenericClientset) string {
	fallback := runtime.GOOS + "/" + runtime.GOARCH
	if clientset == nil || clientset.K8s == nil {
		return fallback
	}
	nodes, err := clientset.K8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || nodes == nil || len(nodes.Items) == 0 {
		return fallback
	}
	counts := map[string]int{}
	for i := range nodes.Items {
		labels := nodes.Items[i].Labels
		arch := labels["kubernetes.io/arch"]
		os := labels["kubernetes.io/os"]
		if arch == "" || os == "" {
			continue
		}
		counts[os+"/"+arch]++
	}
	if len(counts) == 0 {
		return fallback
	}
	type entry struct {
		key   string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for k, v := range counts {
		entries = append(entries, entry{k, v})
	}
	// Most common first; lexicographic tiebreak keeps results stable.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].key < entries[j].key
	})
	return entries[0].key
}

func detectKubeVersion(clientset *k8sUtils.GenericClientset) string {
	if clientset == nil || clientset.K8s == nil {
		return ""
	}
	disc := clientset.K8s.Discovery()
	if disc == nil {
		return ""
	}
	v, err := disc.ServerVersion()
	if err != nil || v == nil {
		return ""
	}
	return v.GitVersion
}
