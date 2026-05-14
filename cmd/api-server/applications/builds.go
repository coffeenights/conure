package applications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

// SystemNamespace is the namespace BuildKit Jobs run in. Mirrors the const
// used by cmd/api-server/main.go for key generation; duplicated here to
// avoid an import cycle (the applications package can't depend on main).
const SystemNamespace = "conure-system"

// RegistryCredentialsSecret is the docker-config secret name expected to
// exist (pre-created by the operator) in SystemNamespace. Mounted into the
// BuildKit container at /root/.docker so `buildctl ... --push` can
// authenticate.
const RegistryCredentialsSecret = "registry-credentials"

// Build job/lease tuning. These are intentionally short — the worst-case
// adoption latency for an orphaned remote build is leaseTTL + scanInterval
// (~90s). Lowering further increases Mongo load without practical benefit.
const (
	buildLeaseTTL       = 60 * time.Second
	buildHeartbeat      = 20 * time.Second
	buildPoll           = 5 * time.Second
	buildJobTTLAfter    = int32(3600)
	buildScanInterval   = 30 * time.Second
	buildAdoptionBatch  = 50
)

// TriggerBuild creates a new Build. Branching on build_location:
//
//   - local: the CLI has already built and pushed image_ref. We record a
//     succeeded build and roll the deploy forward synchronously by writing
//     a fresh draft revision (with source.ociRepository + source.tag
//     overridden) and applying it. The response carries the build record.
//   - remote: we create a BuildKit Job in conure-system that clones the
//     repo, builds with the chosen frontend, and pushes to image_ref. The
//     build is acquired by this replica's watcher (with a Mongo lease) and
//     finished asynchronously. 202 Accepted; the CLI polls.
//
// Path: POST /:orgID/a/:appID/e/:env/c/:componentID/builds
func (a *ApiHandler) TriggerBuild(c *gin.Context) {
	handler, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}

	var req TriggerBuildRequest
	if err := c.BindJSON(&req); err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}
	if err := req.validate(); err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}

	uID := c.MustGet("currentUser").(models.User).ID
	ctx := c.Request.Context()

	build := &models.Build{
		ComponentID:   component.ID,
		ApplicationID: handler.Model.ID,
		EnvironmentID: env.ID,
		BuildTool:     models.BuildTool(req.BuildTool),
		BuildLocation: models.BuildLocation(req.BuildLocation),
		Platform:      req.Platform,
		GitRepository: req.GitRepository,
		GitBranch:     req.GitBranch,
		ImageRef:      req.ImageRef,
		CreatedBy:     uID,
		Status:        models.BuildStatusPending,
	}

	switch build.BuildLocation {
	case models.BuildLocationLocal:
		// CLI already pushed; record terminal state and deploy.
		if err := models.Create(ctx, a.MongoDB, build); err != nil {
			log.Printf("creating local build: %v", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		if err := build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusSucceeded, req.ImageRef, ""); err != nil {
			log.Printf("marking local build succeeded: %v", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		if err := deployBuildImage(ctx, a, handler.Model, env, component, req.ImageRef, uID); err != nil {
			// Roll the build state back to failed so the user can see why.
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", err.Error())
			log.Printf("deploying local build: %v", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		c.JSON(http.StatusCreated, build)
		return

	case models.BuildLocationRemote:
		if err := models.Create(ctx, a.MongoDB, build); err != nil {
			log.Printf("creating remote build: %v", err)
			conureerrors.AbortWithError(c, err)
			return
		}
		clientset, err := a.kubeClient()
		if err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", "kubernetes client unavailable")
			conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
			return
		}
		job := renderBuildJob(build)
		created, err := clientset.K8s.BatchV1().Jobs(SystemNamespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("creating Job: %v", err))
			log.Printf("creating buildkit job: %v", err)
			conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
			return
		}
		if err := build.SetJob(ctx, a.MongoDB, created.Name, created.Namespace); err != nil {
			log.Printf("recording job on build: %v", err)
		}
		// Acquire lease + spawn watcher. If acquisition fails (very
		// unlikely on a fresh build), the periodic scan will adopt it.
		ok, lerr := build.TryAcquireLease(ctx, a.MongoDB, a.WatcherID, buildLeaseTTL)
		if lerr != nil {
			log.Printf("acquiring build lease: %v", lerr)
		}
		if ok {
			go a.watchBuildJob(context.Background(), *build)
		}
		c.JSON(http.StatusAccepted, build)
		return
	}
}

// ListBuilds returns the most recent builds for the (component, env) pair,
// newest first.
//
// Path: GET /:orgID/a/:appID/e/:env/c/:componentID/builds
func (a *ApiHandler) ListBuilds(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	builds, err := models.ListBuildsForComponent(c.Request.Context(), a.MongoDB, component.ID, env.ID, 100)
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if builds == nil {
		builds = []models.Build{}
	}
	c.JSON(http.StatusOK, BuildListResponse{Builds: builds})
}

// GetBuild returns a single build by ID.
//
// Path: GET /:orgID/a/:appID/e/:env/c/:componentID/builds/:buildID
func (a *ApiHandler) GetBuild(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	build, err := models.GetBuildByID(c.Request.Context(), a.MongoDB, c.Param("buildID"))
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if build.ComponentID != component.ID || build.EnvironmentID != env.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	c.JSON(http.StatusOK, build)
}

// StreamBuildLogs streams logs from the pod backing the BuildKit Job in
// chunked plain text, mirroring the pattern in pods.go. Local builds have
// no logs to stream — they return 400.
//
// Path: GET /:orgID/a/:appID/e/:env/c/:componentID/builds/:buildID/logs
func (a *ApiHandler) StreamBuildLogs(c *gin.Context) {
	_, component, env, ok := loadComponentEnv(c, a)
	if !ok {
		return
	}
	build, err := models.GetBuildByID(c.Request.Context(), a.MongoDB, c.Param("buildID"))
	if err != nil {
		conureerrors.AbortWithError(c, err)
		return
	}
	if build.ComponentID != component.ID || build.EnvironmentID != env.ID {
		conureerrors.AbortWithError(c, conureerrors.ErrObjectNotFound)
		return
	}
	if build.BuildLocation != models.BuildLocationRemote || build.JobName == "" {
		conureerrors.AbortWithError(c, conureerrors.ErrInvalidRequest)
		return
	}

	clientset, err := a.kubeClient()
	if err != nil {
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}

	// Find the pod backing the Job. The Job controller stamps
	// batch.kubernetes.io/job-name (and the legacy job-name label).
	podList, err := clientset.K8s.CoreV1().Pods(build.JobNamespace).List(c.Request.Context(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", build.JobName),
	})
	if err != nil || podList == nil || len(podList.Items) == 0 {
		conureerrors.AbortWithError(c, conureerrors.ErrPodNotFound)
		return
	}

	pod := podList.Items[0]

	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	follow := boolQuery(c, "follow")
	opts := corev1.PodLogOptions{
		Container: "build",
		Follow:    follow,
	}
	req := clientset.K8s.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	stream, err := req.Stream(c.Request.Context())
	if err != nil {
		fmt.Fprintf(c.Writer, "[error] opening log stream: %v\n", err)
		return
	}
	defer stream.Close()
	buf := make([]byte, 4096)
	for {
		n, rerr := stream.Read(buf)
		if n > 0 {
			if _, werr := c.Writer.Write(buf[:n]); werr != nil {
				return
			}
			c.Writer.Flush()
		}
		if rerr != nil {
			return
		}
	}
}

// validate enforces the shape rules the API can't express via binding tags.
// Railpack is allowed only for local builds — the remote BuildKit Job ships
// the dockerfile.v0 frontend; railpack is the gateway frontend the CLI
// invokes locally with `docker buildx --frontend ...` or the railpack CLI.
func (r TriggerBuildRequest) validate() error {
	switch models.BuildTool(r.BuildTool) {
	case models.BuildToolDockerfile, models.BuildToolRailpack:
	default:
		return conureerrors.ErrInvalidRequest
	}
	switch models.BuildLocation(r.BuildLocation) {
	case models.BuildLocationLocal:
		// imageRef carries the already-pushed image; everything else is
		// metadata.
		if r.ImageRef == "" {
			return conureerrors.ErrInvalidRequest
		}
	case models.BuildLocationRemote:
		if r.GitRepository == "" || r.GitBranch == "" || r.ImageRef == "" {
			return conureerrors.ErrInvalidRequest
		}
		// Railpack is local-only — the remote BuildKit Job ships only the
		// dockerfile frontend by default. Loosening this is a future
		// hardening exercise (gateway frontend + outbound network).
		if models.BuildTool(r.BuildTool) == models.BuildToolRailpack {
			return conureerrors.ErrInvalidRequest
		}
		if !isValidPlatform(r.Platform) {
			return conureerrors.ErrInvalidRequest
		}
	default:
		return conureerrors.ErrInvalidRequest
	}
	return nil
}

// deployBuildImage rolls a successful build forward by creating a new
// deployed revision whose values inherit the previous deployed revision (or
// latest draft) and override `source.ociRepository` + `source.tag` from the
// pushed image. The CRD apply runs synchronously; if it fails the caller
// flips the build to failed.
func deployBuildImage(ctx context.Context, a *ApiHandler, app *models.Application, env *models.Environment, component *models.Component, imageRef string, userID primitive.ObjectID) error {
	repo, tag := splitImageRef(imageRef)
	if repo == "" || tag == "" {
		return fmt.Errorf("malformed image_ref %q (need repo:tag)", imageRef)
	}

	baseValues, err := baseValuesForBuild(ctx, a, component, env)
	if err != nil {
		return err
	}
	values := mergeImageIntoValues(baseValues, repo, tag)

	if err := applyRevisionToK8s(ctx, a, app, env, component, values); err != nil {
		return err
	}
	rev := &models.ComponentRevision{
		ComponentID:   component.ID,
		EnvironmentID: env.ID,
		Values:        values,
		Comment:       fmt.Sprintf("Build %s:%s", repo, tag),
		CreatedBy:     userID,
	}
	return rev.CreateDeployed(ctx, a.MongoDB)
}

// baseValuesForBuild returns the values blob a build should layer the new
// image onto. Preference order: latest draft (the user may have prepped
// non-image edits there) → latest deployed (steady state) → empty map (no
// history yet — first build).
func baseValuesForBuild(ctx context.Context, a *ApiHandler, component *models.Component, env *models.Environment) (map[string]interface{}, error) {
	draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
	if err == nil && draft != nil {
		return cloneValues(draft.Values), nil
	}
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil, err
	}
	deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
	if err == nil && deployed != nil {
		return cloneValues(deployed.Values), nil
	}
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil, err
	}
	return map[string]interface{}{}, nil
}

func cloneValues(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// mergeImageIntoValues writes the pushed image into the well-known
// `source.{ociRepository,tag}` path that the webservice template consumes.
// Other top-level value keys (env, replicas, resources, …) are preserved.
func mergeImageIntoValues(values map[string]interface{}, repo, tag string) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	src, _ := values["source"].(map[string]interface{})
	if src == nil {
		src = map[string]interface{}{}
	}
	src["ociRepository"] = repo
	src["tag"] = tag
	values["source"] = src
	return values
}

// splitImageRef splits "registry/repo:tag" into ("registry/repo", "tag").
// Returns ("", "") when no colon is present after the last slash.
func splitImageRef(ref string) (string, string) {
	idx := strings.LastIndex(ref, ":")
	slash := strings.LastIndex(ref, "/")
	if idx < 0 || idx < slash {
		return "", ""
	}
	return ref[:idx], ref[idx+1:]
}

// renderBuildJob builds the BuildKit Job spec. The init container clones
// git_repository@git_branch into /workspace; the build container starts
// buildkitd, waits for it to be ready, and runs `buildctl build ... --push`.
//
// Privileged is required for rootful buildkitd. Cluster operators that
// can't grant privileged should deploy a pre-baked buildkitd Service and
// have this Job talk to it via BUILDKIT_HOST (future work).
func renderBuildJob(b *models.Build) *batchv1.Job {
	name := fmt.Sprintf("build-%s", b.ID.Hex())
	privileged := true
	backoffLimit := int32(0)
	ttl := buildJobTTLAfter

	frontendArgs := dockerfileFrontendArgs()
	if b.BuildTool == models.BuildToolRailpack {
		// Railpack on remote is rejected in validate(), but keep the
		// branch wired in case we relax that constraint later.
		frontendArgs = railpackFrontendArgs()
	}

	platformArg := ""
	if b.Platform != "" {
		platformArg = fmt.Sprintf("--opt platform=%s ", shellQuote(b.Platform))
	}

	buildScript := fmt.Sprintf(`
set -eu
buildkitd --addr unix:///run/buildkit/buildkitd.sock &
BUILDKITD_PID=$!
trap 'kill $BUILDKITD_PID 2>/dev/null || true' EXIT

# Wait for buildkitd to come up. Caps at ~30s.
for i in $(seq 1 60); do
  if buildctl --addr unix:///run/buildkit/buildkitd.sock debug workers >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

buildctl --addr unix:///run/buildkit/buildkitd.sock build \
  %s \
  --local context=/workspace \
  --local dockerfile=/workspace \
  %s\
  --output type=image,name=%s,push=true
`, frontendArgs, platformArg, shellQuote(b.ImageRef))

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SystemNamespace,
			Labels: map[string]string{
				"conure.io/build-id":       b.ID.Hex(),
				"conure.io/component-id":   b.ComponentID.Hex(),
				"conure.io/application-id": b.ApplicationID.Hex(),
				"app.kubernetes.io/name":   "conure-build",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"conure.io/build-id":     b.ID.Hex(),
						"app.kubernetes.io/name": "conure-build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: "alpine/git:latest",
							Args: []string{
								"clone",
								"--branch", b.GitBranch,
								"--depth", "1",
								b.GitRepository,
								"/workspace",
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "build",
							Image:   "moby/buildkit:latest",
							Command: []string{"sh", "-c", buildScript},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Env: []corev1.EnvVar{
								{Name: "DOCKER_CONFIG", Value: "/root/.docker"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
								{Name: "registry-creds", MountPath: "/root/.docker", ReadOnly: true},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name:         "workspace",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name: "registry-creds",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: RegistryCredentialsSecret,
									Items: []corev1.KeyToPath{
										{Key: ".dockerconfigjson", Path: "config.json"},
									},
									Optional: ptrBool(false),
								},
							},
						},
					},
				},
			},
		},
	}
}

func dockerfileFrontendArgs() string {
	return "--frontend dockerfile.v0"
}

// railpackFrontendArgs returns the gateway-frontend flags Railpack expects.
// Documented for forward-compatibility; remote builds reject this tool today.
func railpackFrontendArgs() string {
	return "--frontend gateway.v0 --opt source=ghcr.io/railwayapp/railpack-frontend:latest"
}

// shellQuote single-quotes a value for safe interpolation inside a sh -c
// script. Embedded single quotes are escaped via the close+escaped+reopen
// idiom.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isValidPlatform accepts BuildKit platform strings of the form
// `os/arch[/variant]` (e.g. linux/amd64, linux/arm64/v8). Empty is
// allowed — buildkitd then picks the worker's native platform.
func isValidPlatform(p string) bool {
	if p == "" {
		return true
	}
	parts := strings.Split(p, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			switch {
			case r >= 'a' && r <= 'z':
			case r >= 'A' && r <= 'Z':
			case r >= '0' && r <= '9':
			case r == '.' || r == '_' || r == '-':
			default:
				return false
			}
		}
	}
	return true
}

func ptrBool(b bool) *bool { return &b }

// watchBuildJob polls the BuildKit Job, heartbeats the lease, and on
// completion (success or failure) marks the build terminal and — on success
// — rolls the deploy forward.
//
// Designed to be safe under HA: the caller MUST hold the lease before
// spawning this. We heartbeat every buildHeartbeat; if Heartbeat returns
// false (the lease was stolen by another replica) we abandon the goroutine.
// Polling uses the watcher's own context so a server shutdown stops it; the
// lease then expires and another replica picks the build up via the
// periodic adoption scan.
func (a *ApiHandler) watchBuildJob(ctx context.Context, build models.Build) {
	pollT := time.NewTicker(buildPoll)
	defer pollT.Stop()
	hbT := time.NewTicker(buildHeartbeat)
	defer hbT.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hbT.C:
			ok, err := build.Heartbeat(ctx, a.MongoDB, a.WatcherID, buildLeaseTTL)
			if err != nil {
				log.Printf("watcher %s: heartbeat error: %v", build.ID.Hex(), err)
				continue
			}
			if !ok {
				log.Printf("watcher %s: lease lost; another replica owns this build", build.ID.Hex())
				return
			}
		case <-pollT.C:
			done, err := a.pollBuildJobOnce(ctx, &build)
			if err != nil {
				log.Printf("watcher %s: poll error: %v", build.ID.Hex(), err)
				continue
			}
			if done {
				return
			}
		}
	}
}

// pollBuildJobOnce is one polling step. Returns done=true when the build
// has reached a terminal state and the watcher should exit.
func (a *ApiHandler) pollBuildJobOnce(ctx context.Context, build *models.Build) (bool, error) {
	cli, err := a.kubeClient()
	if err != nil {
		return false, err
	}
	job, err := cli.K8s.BatchV1().Jobs(build.JobNamespace).Get(ctx, build.JobName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", "build Job not found")
			return true, nil
		}
		return false, err
	}

	if job.Status.Succeeded > 0 {
		application := &models.Application{}
		if err := application.GetByID(a.MongoDB, build.ApplicationID.Hex()); err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("loading application: %v", err))
			return true, nil
		}
		component := &models.Component{}
		if err := component.GetByID(a.MongoDB, build.ComponentID.Hex()); err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("loading component: %v", err))
			return true, nil
		}
		envName := envNameByID(application, build.EnvironmentID)
		if envName == "" {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", "environment not found")
			return true, nil
		}
		envObj, err := application.GetEnvironmentByName(a.MongoDB, envName)
		if err != nil || envObj == nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", "environment not found")
			return true, nil
		}
		if err := deployBuildImage(ctx, a, application, envObj, component, build.ImageRef, build.CreatedBy); err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("deploying: %v", err))
			return true, nil
		}
		_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusSucceeded, build.ImageRef, "")
		return true, nil
	}
	if job.Status.Failed > 0 {
		msg := tailBuildPodLogs(ctx, a, build.JobNamespace, build.JobName)
		if msg == "" {
			msg = "build failed; see job logs"
		}
		_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", msg)
		return true, nil
	}
	return false, nil
}

// envNameByID resolves env.ID → env.Name within an Application's
// Environments slice. Returns "" when no match.
func envNameByID(app *models.Application, envID string) string {
	for _, e := range app.Environments {
		if e.ID == envID {
			return e.Name
		}
	}
	return ""
}

// tailBuildPodLogs reads up to 2 KB of the build pod's logs to attach as
// the failure's ErrorMessage. Best-effort: returns "" on any failure so
// the caller can substitute a generic message.
func tailBuildPodLogs(ctx context.Context, a *ApiHandler, ns, jobName string) string {
	cli, err := a.kubeClient()
	if err != nil {
		return ""
	}
	pods, err := cli.K8s.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || pods == nil || len(pods.Items) == 0 {
		return ""
	}
	pod := pods.Items[0]
	tail := int64(40)
	stream, err := cli.K8s.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: "build",
		TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()
	buf := make([]byte, 2048)
	n, _ := stream.Read(buf)
	if n <= 0 {
		return ""
	}
	return strings.TrimSpace(string(buf[:n]))
}
