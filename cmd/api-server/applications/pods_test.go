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
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				k8sUtils.ComponentNameLabel: componentName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				Ready:        ready,
				RestartCount: restarts,
			}},
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

func TestResolveLogPods(t *testing.T) {
	const ns = "abc12345-staging"
	clientset := &k8sUtils.GenericClientset{
		K8s: fake.NewSimpleClientset(
			makePod(ns, "web-1", "web", true, 0),
			makePod(ns, "web-2", "web", true, 0),
			makePod(ns, "other-1", "other", true, 0),
		),
	}
	ctx := context.Background()

	// Empty query → every pod matching the component label.
	got, err := resolveLogPods(ctx, clientset, ns, "web", "")
	if err != nil {
		t.Fatalf("empty query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("empty query: expected 2 pods, got %v", got)
	}

	// Valid subset.
	got, err = resolveLogPods(ctx, clientset, ns, "web", "web-1")
	if err != nil {
		t.Fatalf("valid subset: %v", err)
	}
	if len(got) != 1 || got[0] != "web-1" {
		t.Fatalf("valid subset: got %v", got)
	}

	// Whitespace + comma list.
	got, err = resolveLogPods(ctx, clientset, ns, "web", " web-1 , web-2 ")
	if err != nil {
		t.Fatalf("comma list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("comma list: got %v", got)
	}

	// Cross-component pod must be rejected — the user is asking for `other-1`
	// while scoped to component `web`. This is the security-relevant case.
	if _, err := resolveLogPods(ctx, clientset, ns, "web", "other-1"); !errors.Is(err, conureerrors.ErrPodNotFound) {
		t.Fatalf("expected ErrPodNotFound for cross-component request, got %v", err)
	}

	// Unknown pod name.
	if _, err := resolveLogPods(ctx, clientset, ns, "web", "ghost"); !errors.Is(err, conureerrors.ErrPodNotFound) {
		t.Fatalf("expected ErrPodNotFound for unknown pod, got %v", err)
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
