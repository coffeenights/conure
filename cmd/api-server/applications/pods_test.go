package applications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// withFakeKube swaps a fake clientset onto the shared handler for the duration
// of one test. Tests run sequentially in this package, so this is safe.
func withFakeKube(t *testing.T, objects ...*corev1.Pod) {
	t.Helper()
	objs := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		objs = append(objs, o)
	}
	testConf.app.Kube = &k8sUtils.GenericClientset{K8s: fake.NewSimpleClientset(objs...)}
	t.Cleanup(func() { testConf.app.Kube = nil })
}

// seedComponentInEnv creates a Mongo identity row for `componentName` linked
// to `app`, seeds a deployed v1 in `env`, and registers cleanup. Returns the
// component so callers can build URLs against its hex ID.
func seedComponentInEnv(t *testing.T, app *models.Application, env *models.Environment, componentName string) *models.Component {
	t.Helper()
	component := &models.Component{
		Name:          componentName,
		Type:          "service",
		ApplicationID: app.ID,
	}
	if err := component.Create(testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	cleanupComponent(t, component)

	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
	}
	if err := rev.CreateDeployed(context.Background(), testConf.app.MongoDB); err != nil {
		t.Fatal(err)
	}
	return component
}

// makePod is a tiny builder that fills in the kubectl-relevant fields the
// handler rolls up. Container statuses default to one ready container.
func makePod(namespace, name, componentName string, ready bool, restarts int32) *corev1.Pod {
	return makePodWithContainers(namespace, name, componentName, ready, restarts, "app")
}

func makePodWithContainers(namespace, name, componentName string, ready bool, restarts int32, containers ...string) *corev1.Pod {
	containerSpecs := make([]corev1.Container, len(containers))
	containerStatuses := make([]corev1.ContainerStatus, len(containers))
	for i, c := range containers {
		containerSpecs[i] = corev1.Container{Name: c}
		containerStatuses[i] = corev1.ContainerStatus{
			Name:         c,
			Ready:        ready,
			RestartCount: restarts,
		}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				k8sUtils.ComponentNameLabel: componentName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: containerSpecs,
		},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: containerStatuses,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
				Reason: "Ready",
			}},
		},
	}
}

func TestListComponentPods_FiltersByLabel(t *testing.T) {
	org, app, env := orgWithApp(t, "TestListPods_Filter", "staging")
	component := seedComponentInEnv(t, app, env, "web")

	ns := env.GetNamespace()
	withFakeKube(t,
		makePod(ns, "web-1", "web", true, 0),
		makePod(ns, "web-2", "web", false, 3),
		makePod(ns, "other-1", "other", true, 0), // different component, must not appear
	)

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/pods"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", resp.Code, resp.Body.String())
	}

	var got ComponentPodsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pods) != 2 {
		t.Fatalf("expected 2 pods (web-1, web-2), got %d: %+v", len(got.Pods), got.Pods)
	}

	byName := map[string]PodResponse{}
	for _, p := range got.Pods {
		byName[p.Name] = p
	}
	if !byName["web-1"].Ready || byName["web-1"].Restarts != 0 {
		t.Errorf("web-1 rollup wrong: %+v", byName["web-1"])
	}
	if byName["web-2"].Ready || byName["web-2"].Restarts != 3 {
		t.Errorf("web-2 rollup wrong: %+v", byName["web-2"])
	}
	if got := byName["web-1"].Containers; len(got) != 1 || got[0] != "app" {
		t.Errorf("web-1 containers wrong: %v", got)
	}
}

func TestListComponentPods_EmptyNamespace(t *testing.T) {
	org, app, env := orgWithApp(t, "TestListPods_Empty", "staging")
	component := seedComponentInEnv(t, app, env, "web")
	withFakeKube(t) // no pods seeded

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/pods"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var got ComponentPodsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Pods) != 0 {
		t.Fatalf("expected zero pods, got %d", len(got.Pods))
	}
}

func TestListComponentPods_UnknownComponent404(t *testing.T) {
	org, app, env := orgWithApp(t, "TestListPods_UnknownComponent", "staging")
	withFakeKube(t)

	// Hitting a fabricated (well-formed but unknown) component ID must 404
	// rather than leaking pods from an unrelated namespace.
	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/000000000000000000000000/pods"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown component, got %d", resp.Code)
	}
}

func TestResolveLogTargets(t *testing.T) {
	const ns = "abc12345-staging"
	clientset := &k8sUtils.GenericClientset{
		K8s: fake.NewSimpleClientset(
			makePodWithContainers(ns, "web-1", "web", true, 0, "app", "sidecar"),
			makePodWithContainers(ns, "web-2", "web", true, 0, "app"),
			makePod(ns, "other-1", "other", true, 0),
		),
	}
	ctx := context.Background()

	// Empty pods + empty container ⇒ every container of every pod for the
	// component. web-1 has 2 containers, web-2 has 1 ⇒ 3 targets total.
	got, err := resolveLogTargets(ctx, clientset, ns, "web", "", "")
	if err != nil {
		t.Fatalf("bare: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("bare: expected 3 targets, got %d: %+v", len(got), got)
	}

	// Pod filter alone.
	got, err = resolveLogTargets(ctx, clientset, ns, "web", "web-2", "")
	if err != nil {
		t.Fatalf("pod filter: %v", err)
	}
	if len(got) != 1 || got[0].pod != "web-2" || got[0].container != "app" {
		t.Fatalf("pod filter: got %+v", got)
	}

	// Container filter — only pods that define it should appear; web-2
	// (which has no `sidecar`) is silently skipped.
	got, err = resolveLogTargets(ctx, clientset, ns, "web", "", "sidecar")
	if err != nil {
		t.Fatalf("container filter: %v", err)
	}
	if len(got) != 1 || got[0].pod != "web-1" || got[0].container != "sidecar" {
		t.Fatalf("container filter: got %+v", got)
	}

	// Whitespace + comma list of pods.
	got, err = resolveLogTargets(ctx, clientset, ns, "web", " web-1 , web-2 ", "app")
	if err != nil {
		t.Fatalf("comma list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("comma list: expected 2 (one per pod), got %+v", got)
	}

	// Cross-component pod must be rejected — `other-1` is not owned by `web`.
	// Security guard: callers can't tail unrelated pods by name.
	if _, err := resolveLogTargets(ctx, clientset, ns, "web", "other-1", ""); !errors.Is(err, conureerrors.ErrPodNotFound) {
		t.Fatalf("expected ErrPodNotFound for cross-component request, got %v", err)
	}

	// Unknown pod name.
	if _, err := resolveLogTargets(ctx, clientset, ns, "web", "ghost", ""); !errors.Is(err, conureerrors.ErrPodNotFound) {
		t.Fatalf("expected ErrPodNotFound for unknown pod, got %v", err)
	}

	// Container filter with no matching pods ⇒ empty (handler turns this
	// into a 404; resolveLogTargets just returns nil).
	got, err = resolveLogTargets(ctx, clientset, ns, "web", "", "missing")
	if err != nil {
		t.Fatalf("missing container: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing container: expected empty, got %+v", got)
	}
}

func TestStreamComponentLogs_NoPods404(t *testing.T) {
	org, app, env := orgWithApp(t, "TestStreamLogs_NoPods", "staging")
	component := seedComponentInEnv(t, app, env, "web")
	withFakeKube(t) // namespace empty

	url := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/logs"
	resp := doJSON(t, "GET", url, nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no pods), got %d (%s)", resp.Code, resp.Body.String())
	}
}

func TestStreamComponentLogs_BadQueryValidation(t *testing.T) {
	org, app, env := orgWithApp(t, "TestStreamLogs_BadQuery", "staging")
	component := seedComponentInEnv(t, app, env, "web")
	ns := env.GetNamespace()
	withFakeKube(t, makePod(ns, "web-1", "web", true, 0))

	base := "/organizations/" + org.ID.Hex() + "/a/" + app.ID.Hex() + "/e/" + env.Name + "/c/" + component.ID.Hex() + "/logs"

	cases := []struct {
		name, query string
	}{
		{"non-numeric tail", "?tail=lots"},
		{"negative tail", "?tail=-5"},
		{"unparseable since", "?since=banana"},
		{"zero since", "?since=0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, "GET", base+tc.query, nil)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%s)", tc.name, resp.Code, resp.Body.String())
			}
		})
	}
}
