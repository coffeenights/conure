package system

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

func newNode(name, os, arch string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/os":   os,
				"kubernetes.io/arch": arch,
			},
		},
	}
}

func TestDetectPlatform_DominantArchWins(t *testing.T) {
	cs := &k8sUtils.GenericClientset{
		K8s: fake.NewSimpleClientset(
			newNode("a", "linux", "amd64"),
			newNode("b", "linux", "amd64"),
			newNode("c", "linux", "arm64"),
		),
	}
	got := detectPlatform(context.Background(), cs)
	if got != "linux/amd64" {
		t.Errorf("expected linux/amd64, got %s", got)
	}
}

func TestDetectPlatform_TieBrokenLexicographically(t *testing.T) {
	cs := &k8sUtils.GenericClientset{
		K8s: fake.NewSimpleClientset(
			newNode("a", "linux", "arm64"),
			newNode("b", "linux", "amd64"),
		),
	}
	got := detectPlatform(context.Background(), cs)
	// linux/amd64 < linux/arm64 lexicographically.
	if got != "linux/amd64" {
		t.Errorf("expected linux/amd64 (lex tiebreak), got %s", got)
	}
}

func TestDetectPlatform_NoNodesReturnsEmpty(t *testing.T) {
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	if got := detectPlatform(context.Background(), cs); got != "" {
		t.Errorf("expected empty platform, got %s", got)
	}
}

func TestDetectPlatform_NilClientReturnsEmpty(t *testing.T) {
	if got := detectPlatform(context.Background(), nil); got != "" {
		t.Errorf("expected empty platform, got %s", got)
	}
}

func TestDetectPlatform_ListErrorReturnsEmpty(t *testing.T) {
	// Simulates missing RBAC / API error: a "list nodes" call returning an
	// error must NOT fall through to a confidently-wrong platform.
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden: cannot list nodes")
	})
	cs := &k8sUtils.GenericClientset{K8s: client}
	if got := detectPlatform(context.Background(), cs); got != "" {
		t.Errorf("expected empty platform on list error, got %s", got)
	}
}

func TestDetectPlatform_AllNodesUnlabeledReturnsEmpty(t *testing.T) {
	bare := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "bare"}}
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset(bare)}
	if got := detectPlatform(context.Background(), cs); got != "" {
		t.Errorf("expected empty platform when no nodes have arch/os labels, got %s", got)
	}
}

func TestDetectPlatform_IgnoresUnlabeledNodes(t *testing.T) {
	// A node without arch/os labels shouldn't count toward the tally.
	bare := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "bare"}}
	cs := &k8sUtils.GenericClientset{
		K8s: fake.NewSimpleClientset(
			bare,
			newNode("good", "linux", "arm64"),
		),
	}
	got := detectPlatform(context.Background(), cs)
	if got != "linux/arm64" {
		t.Errorf("expected linux/arm64, got %s", got)
	}
}
