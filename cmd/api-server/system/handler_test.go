package system

import (
	"context"
	"runtime"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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

func TestDetectPlatform_FallsBackToRuntime(t *testing.T) {
	// No nodes — the fallback path should kick in.
	cs := &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset()}
	got := detectPlatform(context.Background(), cs)
	want := runtime.GOOS + "/" + runtime.GOARCH
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestDetectPlatform_NilClientReturnsFallback(t *testing.T) {
	got := detectPlatform(context.Background(), nil)
	want := runtime.GOOS + "/" + runtime.GOARCH
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
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
