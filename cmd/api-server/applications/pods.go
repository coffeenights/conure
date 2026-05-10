package applications

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// ListComponentPods returns the pods backing a component in the given env,
// each enriched with phase, ready state, restart count, container names, and
// raw conditions. Selector is the conure.io/component-name label written by
// the controller's Timoni render.
//
// Path: GET /:orgID/a/:applicationID/e/:environment/c/:componentID/pods
func (a *ApiHandler) ListComponentPods(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}

	clientset, err := a.kubeClient()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	pods, err := listPodsForComponent(c.Request.Context(), clientset, env.GetNamespace(), component.Name)
	if err != nil {
		log.Printf("Error listing pods: %v\n", err)
		conureerrors.AbortWithError(c, err)
		return
	}

	resp := ComponentPodsResponse{Pods: make([]PodResponse, 0, len(pods))}
	for i := range pods {
		resp.Pods = append(resp.Pods, podToResponse(&pods[i]))
	}
	c.JSON(http.StatusOK, resp)
}

// StreamComponentLogs streams logs from one or more pods over a single
// chunked plain-text response. When `pods` is omitted, every pod for the
// component is streamed. Each line is prefixed with [podName] when more
// than one pod is involved, so a multiplexed CLI tail is unambiguous.
//
// Path: GET /:orgID/a/:applicationID/e/:environment/c/:componentID/logs
//
//	?pods=p1,p2 — restrict to specific pods (must belong to the component)
//	&follow=true — tail forever
//	&tail=N — emit only the last N lines per pod
//	&since=DURATION — Go duration (e.g. "5m"); maps to SinceSeconds
//	&container=NAME — restrict to a single container
//	&previous=true — read the previous container instance's log
func (a *ApiHandler) StreamComponentLogs(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}

	clientset, err := a.kubeClient()
	if err != nil {
		log.Printf("Error getting clientset: %v\n", err)
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	namespace := env.GetNamespace()
	pods, err := resolveLogPods(c.Request.Context(), clientset, namespace, component.Name, c.Query("pods"))
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if len(pods) == 0 {
		conureerrors.AbortWithError(c, conureerrors.ErrPodNotFound)
		return
	}

	follow := boolQuery(c, "follow")
	previous := boolQuery(c, "previous")
	container := c.Query("container")

	var tailLines *int64
	if v := c.Query("tail"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		tailLines = &n
	}
	var sinceSeconds *int64
	if v := c.Query("since"); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil || dur <= 0 {
			conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
			return
		}
		secs := int64(dur.Seconds())
		sinceSeconds = &secs
	}

	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	prefixLines := len(pods) > 1

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	out := make(chan string, 256)
	errCh := make(chan error, len(pods))
	var wg sync.WaitGroup

	for _, pod := range pods {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			opts := corev1.PodLogOptions{
				Follow:    follow,
				Previous:  previous,
				Container: container,
			}
			if tailLines != nil {
				opts.TailLines = tailLines
			}
			if sinceSeconds != nil {
				opts.SinceSeconds = sinceSeconds
			}
			req := clientset.K8s.CoreV1().Pods(namespace).GetLogs(podName, &opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					errCh <- fmt.Errorf("%s: %w", podName, err)
				}
				return
			}
			defer stream.Close()
			r := bufio.NewReader(stream)
			for {
				line, err := r.ReadString('\n')
				if line != "" {
					if prefixLines {
						line = fmt.Sprintf("[%s] %s", podName, line)
					}
					select {
					case out <- line:
					case <-ctx.Done():
						return
					}
				}
				if err != nil {
					if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
						errCh <- fmt.Errorf("%s: %w", podName, err)
					}
					return
				}
			}
		}(pod)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case e, ok := <-errCh:
			if !ok {
				continue
			}
			fmt.Fprintf(c.Writer, "[error] %s\n", e.Error())
			c.Writer.Flush()
		case line, ok := <-out:
			if !ok {
				return
			}
			if _, err := io.WriteString(c.Writer, line); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

// listPodsForComponent fetches every pod in `namespace` carrying the
// component-name label. Returns an empty slice (not error) when the namespace
// has no matching pods, which is a normal "not running yet" state.
func listPodsForComponent(ctx context.Context, clientset *k8sUtils.GenericClientset, namespace, componentName string) ([]corev1.Pod, error) {
	selector := fmt.Sprintf("%s=%s", k8sUtils.ComponentNameLabel, componentName)
	list, err := clientset.K8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if list == nil {
		return nil, nil
	}
	return list.Items, nil
}

// resolveLogPods picks which pods to stream from. Empty `query` ⇒ every pod
// for the component. Otherwise the comma list is intersected with the
// component's pods so callers can't tail an unrelated pod by guessing its
// name.
func resolveLogPods(ctx context.Context, clientset *k8sUtils.GenericClientset, namespace, componentName, query string) ([]string, error) {
	pods, err := listPodsForComponent(ctx, clientset, namespace, componentName)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(pods))
	for i := range pods {
		allowed[pods[i].Name] = struct{}{}
	}

	if strings.TrimSpace(query) == "" {
		out := make([]string, 0, len(pods))
		for i := range pods {
			out = append(out, pods[i].Name)
		}
		return out, nil
	}

	var out []string
	for _, raw := range strings.Split(query, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := allowed[name]; !ok {
			return nil, conureerrors.ErrPodNotFound
		}
		out = append(out, name)
	}
	return out, nil
}

func podToResponse(p *corev1.Pod) PodResponse {
	resp := PodResponse{
		Name:  p.Name,
		Phase: string(p.Status.Phase),
	}
	for _, ctr := range p.Spec.Containers {
		resp.Containers = append(resp.Containers, ctr.Name)
	}
	// "Ready" matches kubectl: true only when every container is ready and
	// at least one container exists.
	ready := len(p.Status.ContainerStatuses) > 0
	for _, cs := range p.Status.ContainerStatuses {
		if !cs.Ready {
			ready = false
		}
		resp.Restarts += cs.RestartCount
	}
	resp.Ready = ready
	for _, cond := range p.Status.Conditions {
		resp.Conditions = append(resp.Conditions, PodConditionResponse{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
	}
	return resp
}

func boolQuery(c *gin.Context, key string) bool {
	v := strings.ToLower(c.Query(key))
	return v == "true" || v == "1" || v == "yes"
}
