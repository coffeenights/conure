package applications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/coffeenights/conure/cmd/api-server/conureerrors"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
	"github.com/coffeenights/conure/cmd/api-server/providers"
	"github.com/coffeenights/conure/internal/fieldroles"
	k8sUtils "github.com/coffeenights/conure/internal/k8s"
)

// SystemNamespace is the namespace BuildKit Jobs run in. Mirrors the const
// used by cmd/api-server/main.go for key generation; duplicated here to
// avoid an import cycle (the applications package can't depend on main).
const SystemNamespace = "conure-system"

// Build job/lease tuning. These are intentionally short — the worst-case
// adoption latency for an orphaned remote build is leaseTTL + scanInterval
// (~90s). Lowering further increases Mongo load without practical benefit.
const (
	buildLeaseTTL      = 60 * time.Second
	buildHeartbeat     = 20 * time.Second
	buildPoll          = 5 * time.Second
	buildJobTTLAfter   = int32(3600)
	buildScanInterval  = 30 * time.Second
	buildAdoptionBatch = 50
	// buildJobDeadline bounds remote build wall-clock time. Anything past
	// this is almost certainly a stuck git clone or hung buildkitd and not
	// a legitimate slow build; without it a wedged Job holds its
	// privileged pod + Mongo lease forever while the CLI polls forever.
	buildJobDeadline = int64(45 * 60)
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
		registrySecretName, gitSecretName, credErr := resolveBuildCredentialSecrets(ctx, a, clientset, handler.Model, env, component)
		if credErr != nil {
			// A referenced-but-missing/wrong-kind credential is a hard
			// error: fail the build now with an actionable message rather
			// than spawn a Job that 403s on clone/push with no API signal.
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", credErr.Error())
			log.Printf("resolving build credentials: %v", credErr)
			// Surface the actionable reason (which credential, how to fix)
			// to the client, not just the bare invalid_request code.
			conureerrors.AbortWithError(c, conureerrors.WithDetail(
				conureerrors.ErrInvalidRequest, "%s", credErr.Error()))
			return
		}
		job := renderBuildJob(build, registrySecretName, gitSecretName)
		created, err := clientset.K8s.BatchV1().Jobs(SystemNamespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("creating Job: %v", err))
			log.Printf("creating buildkit job: %v", err)
			conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
			return
		}
		if err := setJobWithRetry(ctx, a.MongoDB, build, created.Name, created.Namespace); err != nil {
			// Without jobName the adoption scanner can never pick this build
			// up (see models.AdoptableBuilds: jobName must be non-empty), so
			// the Job would run untracked forever. Best-effort delete the
			// Job and mark the build failed.
			log.Printf("recording job on build: %v", err)
			if derr := clientset.K8s.BatchV1().Jobs(created.Namespace).Delete(ctx, created.Name, metav1.DeleteOptions{}); derr != nil && !k8sErrors.IsNotFound(derr) {
				log.Printf("deleting untracked buildkit job %s/%s: %v", created.Namespace, created.Name, derr)
			}
			_ = build.MarkTerminal(ctx, a.MongoDB, models.BuildStatusFailed, "", fmt.Sprintf("recording Job on Build: %v", err))
			conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
			return
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

	// The `build` container only starts once the init containers (git
	// clone, etc.) finish. While the pod is PodInitializing, the Kubernetes
	// API rejects a log request for it with "container ... is waiting to
	// start: PodInitializing". We must detect this *before* committing a
	// status code: once 200 is written we can no longer signal "retry", and
	// the client's polling loop would treat the error body as success.
	if !buildContainerStarted(&pod) {
		conureerrors.AbortWithError(c, conureerrors.ErrBuildLogsNotReady)
		return
	}

	follow := boolQuery(c, "follow")
	opts := corev1.PodLogOptions{
		Container: "build",
		Follow:    follow,
	}
	req := clientset.K8s.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	stream, err := req.Stream(c.Request.Context())
	if err != nil {
		// Race: the container state read above passed but the kubelet
		// hasn't surfaced logs yet. Still recoverable — the status code
		// is uncommitted, so return a retryable error rather than a 200
		// with an error body the client can't distinguish from success.
		if isContainerWaitingErr(err) {
			conureerrors.AbortWithError(c, conureerrors.ErrBuildLogsNotReady)
			return
		}
		conureerrors.AbortWithError(c, conureerrors.ErrInternalError)
		return
	}
	defer stream.Close()

	// Stream is open — only now is it safe to commit 200. From here on any
	// failure is mid-stream and surfaces as a closed connection.
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
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

// buildContainerStarted reports whether the pod's `build` container has
// progressed past Waiting — i.e. it is Running or has Terminated and
// therefore has logs to stream. A pod that is still PodInitializing has the
// build container in Waiting (reason ContainerCreating/PodInitializing), so
// this returns false and the caller can signal a retry.
func buildContainerStarted(pod *corev1.Pod) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != "build" {
			continue
		}
		// Running or Terminated both have retrievable logs. Only Waiting
		// (the zero-progress state) means "not yet".
		return cs.State.Waiting == nil
	}
	return false
}

// isContainerWaitingErr matches the Kubernetes API error returned when logs
// are requested for a container that hasn't started. The message is stable
// across kubelet versions: `... is waiting to start: PodInitializing` (or
// ContainerCreating). Used as a race backstop after the status check passes.
func isContainerWaitingErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "is waiting to start") ||
		strings.Contains(msg, "ContainerCreating") ||
		strings.Contains(msg, "PodInitializing")
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
		// metadata. Reject malformed refs (missing tag) up front so the
		// caller gets a 400 instead of a 500 from deployBuildImage after
		// the Build doc has already been persisted.
		if r.ImageRef == "" || !isFullyQualifiedImageRef(r.ImageRef) {
			return conureerrors.ErrInvalidRequest
		}
	case models.BuildLocationRemote:
		if r.GitRepository == "" || r.GitBranch == "" || r.ImageRef == "" {
			return conureerrors.ErrInvalidRequest
		}
		if !isFullyQualifiedImageRef(r.ImageRef) {
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
//
// If the base values came from a draft, that draft is marked deployed after
// the new deployed revision lands. Otherwise a later plain `conure deploy`
// would re-apply the pre-build values (including the old image) on top of
// what the build just rolled out.
func deployBuildImage(ctx context.Context, a *ApiHandler, app *models.Application, env *models.Environment, component *models.Component, imageRef string, userID primitive.ObjectID) error {
	repo, tag := splitImageRef(imageRef)
	if repo == "" || tag == "" {
		return fmt.Errorf("malformed image_ref %q (need repo:tag)", imageRef)
	}

	resolver, err := a.resolveFieldRoles(ctx, app.OrganizationID, component)
	if err != nil {
		return err
	}

	baseValues, draft, err := baseValuesForBuild(ctx, a, component, env)
	if err != nil {
		return err
	}
	values, err := mergeImageIntoValues(resolver, baseValues, repo, tag)
	if err != nil {
		return err
	}

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
	if err := rev.CreateDeployed(ctx, a.MongoDB); err != nil {
		return err
	}
	if draft != nil {
		if err := draft.MarkDeployed(ctx, a.MongoDB); err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
			return err
		}
	}
	return nil
}

// baseValuesForBuild returns the values blob a build should layer the new
// image onto. Preference order: latest draft (the user may have prepped
// non-image edits there) → latest deployed (steady state) → empty map (no
// history yet — first build).
//
// The returned *ComponentRevision is non-nil only when values came from a
// draft; callers use it to mark the draft deployed once the build rolls out,
// so it doesn't linger as a stale pending revision pointing at the old image.
func baseValuesForBuild(ctx context.Context, a *ApiHandler, component *models.Component, env *models.Environment) (map[string]interface{}, *models.ComponentRevision, error) {
	draft, err := models.LatestDraft(ctx, a.MongoDB, component.ID, env.ID)
	if err == nil && draft != nil {
		return cloneValues(draft.Values), draft, nil
	}
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil, nil, err
	}
	deployed, err := models.LatestDeployed(ctx, a.MongoDB, component.ID, env.ID)
	if err == nil && deployed != nil {
		return cloneValues(deployed.Values), nil, nil
	}
	if err != nil && !errors.Is(err, conureerrors.ErrObjectNotFound) {
		return nil, nil, err
	}
	return map[string]interface{}{}, nil, nil
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

// resolveBuildCredentialSecrets reads the component's optional
// image.credentialRef / git.credentialRef field roles from the values a
// build would deploy, resolves each to the org's Credential, and projects
// them into the system namespace. It returns the CONCRETE projected Secret
// names to mount in the build Job (registry for push, git for clone).
//
// Both roles are OPTIONAL (GetOptional): a definition that never declares
// them, or a component that never sets them, yields "" — the public,
// no-auth path, where renderBuildJob mounts nothing and clones/pushes
// anonymously. A referenced-but-missing or wrong-kind credential is a hard
// error with no fallback, surfaced before the Job is created.
func resolveBuildCredentialSecrets(ctx context.Context, a *ApiHandler, clientset *k8sUtils.GenericClientset, app *models.Application, env *models.Environment, component *models.Component) (registrySecretName, gitSecretName string, err error) {
	resolver, err := a.resolveFieldRoles(ctx, app.OrganizationID, component)
	if err != nil {
		return "", "", err
	}
	baseValues, _, err := baseValuesForBuild(ctx, a, component, env)
	if err != nil {
		return "", "", err
	}

	imageRef, err := resolver.GetOptional(baseValues, fieldroles.RoleImageCredentialRef)
	if err != nil {
		return "", "", fmt.Errorf("reading %s: %w", fieldroles.RoleImageCredentialRef, err)
	}
	gitRef, err := resolver.GetOptional(baseValues, fieldroles.RoleGitCredentialRef)
	if err != nil {
		return "", "", fmt.Errorf("reading %s: %w", fieldroles.RoleGitCredentialRef, err)
	}

	cr := &providers.CredentialResolver{DB: a.MongoDB, KeyStorage: a.KeyStorage}
	registrySecretName, err = cr.ProjectBuildCredential(ctx, clientset, app.OrganizationID, imageRef, models.CredentialKindRegistry)
	if err != nil {
		return "", "", err
	}
	gitSecretName, err = cr.ProjectBuildCredential(ctx, clientset, app.OrganizationID, gitRef, models.CredentialKindGit)
	if err != nil {
		return "", "", err
	}
	return registrySecretName, gitSecretName, nil
}

// mergeImageIntoValues writes the pushed image into the paths the
// component's ComponentDefinition declares for the image.repository and
// image.tag field roles. The definition owns where the image lives in its
// own #Config schema; there is no hardcoded `source.*` fallback. Other
// value keys (env, replicas, resources, …) are preserved.
func mergeImageIntoValues(r *fieldroles.Resolver, values map[string]interface{}, repo, tag string) (map[string]interface{}, error) {
	if values == nil {
		values = map[string]interface{}{}
	}
	if err := r.Set(values, fieldroles.RoleImageRepository, repo); err != nil {
		return nil, fmt.Errorf("writing built image repository: %w", err)
	}
	if err := r.Set(values, fieldroles.RoleImageTag, tag); err != nil {
		return nil, fmt.Errorf("writing built image tag: %w", err)
	}
	return values, nil
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

// isFullyQualifiedImageRef reports whether ref splits cleanly into
// (repo, tag). deployBuildImage requires both, so rejecting the malformed
// shape at request-validate time turns a post-persist 500 into a 400 before
// the Build document is ever written.
func isFullyQualifiedImageRef(ref string) bool {
	repo, tag := splitImageRef(ref)
	return repo != "" && tag != ""
}

// renderBuildJob builds the BuildKit Job spec. The init container clones
// git_repository@git_branch into /workspace; the build container starts
// buildkitd, waits for it to be ready, and runs `buildctl build ... --push`.
//
// registrySecretName / gitSecretName are CONCRETE projected Secret names
// (resolved org-side from the component's image.credentialRef /
// git.credentialRef field roles) or "" when the component declares no
// credential. Empty == public, no auth: the push Secret is simply not
// mounted and the clone is anonymous, exactly as before credentials existed.
// There is no hardcoded fallback Secret — a private push with no resolvable
// credential fails fast upstream in TriggerBuild, not silently here.
//
// Privileged is required for rootful buildkitd. Cluster operators that
// can't grant privileged should deploy a pre-baked buildkitd Service and
// have this Job talk to it via BUILDKIT_HOST (future work).
func renderBuildJob(b *models.Build, registrySecretName, gitSecretName string) *batchv1.Job {
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

	volumes := []corev1.Volume{
		{
			Name:         "workspace",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}

	// git-clone init container. Public source: a plain `clone`. Private
	// source (a projected git Secret was resolved from git.credentialRef):
	// mount the Secret and rewrite the URL to embed the token so HTTPS
	// auth works without a credential helper. The token is passed via env
	// (secretKeyRef), not the args, so it does not show up in the Job spec
	// or `kubectl describe`.
	gitContainer := corev1.Container{
		Name:  "git-clone",
		Image: "alpine/git:latest",
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}
	if gitSecretName == "" {
		gitContainer.Args = []string{
			"clone", "--branch", b.GitBranch, "--depth", "1", b.GitRepository, "/workspace",
		}
	} else {
		// Rewrite https://host/path -> https://user:token@host/path. Only
		// https is supported (SSH is out of scope); a non-https URL with a
		// credential is a misconfiguration and fails the clone clearly.
		gitContainer.Command = []string{"sh", "-c", `
set -eu
case "$GIT_REPO" in
  https://*) ;;
  *) echo "git.credentialRef is set but the repository URL is not https://; token auth only supports HTTPS" >&2; exit 1 ;;
esac
AUTH_URL=$(printf '%s' "$GIT_REPO" | sed -E "s#^https://#https://${GIT_USERNAME}:${GIT_TOKEN}@#")
git clone --branch "$GIT_BRANCH" --depth 1 "$AUTH_URL" /workspace
`}
		gitContainer.Env = []corev1.EnvVar{
			{Name: "GIT_REPO", Value: b.GitRepository},
			{Name: "GIT_BRANCH", Value: b.GitBranch},
			{Name: "GIT_USERNAME", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: gitSecretName},
					Key:                  "username",
				},
			}},
			{Name: "GIT_TOKEN", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: gitSecretName},
					Key:                  "token",
				},
			}},
		}
	}

	buildContainer := corev1.Container{
		Name:    "build",
		Image:   "moby/buildkit:latest",
		Command: []string{"sh", "-c", buildScript},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	}
	// Push auth. Only mount a docker config when a registry credential was
	// resolved (image.credentialRef). No credential -> no mount -> an
	// anonymous push (works for public registries; a private push with no
	// credential is rejected up front in TriggerBuild, so we never silently
	// fall back to a shared Secret here).
	if registrySecretName != "" {
		buildContainer.Env = []corev1.EnvVar{
			{Name: "DOCKER_CONFIG", Value: "/root/.docker"},
		}
		buildContainer.VolumeMounts = append(buildContainer.VolumeMounts,
			corev1.VolumeMount{Name: "registry-creds", MountPath: "/root/.docker", ReadOnly: true})
		volumes = append(volumes, corev1.Volume{
			Name: "registry-creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: registrySecretName,
					Items: []corev1.KeyToPath{
						{Key: ".dockerconfigjson", Path: "config.json"},
					},
					Optional: ptrBool(false),
				},
			},
		})
	}

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
			ActiveDeadlineSeconds:   ptrInt64(buildJobDeadline),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"conure.io/build-id":     b.ID.Hex(),
						"app.kubernetes.io/name": "conure-build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:  corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{gitContainer},
					Containers:     []corev1.Container{buildContainer},
					Volumes:        volumes,
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

// setJobWithRetry retries Build.SetJob a few times on transient Mongo
// errors. The Job already exists in the cluster by this point; if we never
// persist jobName the adoption scanner can't see this build, so the work
// runs untracked. Three attempts cover the typical transient blip without
// holding the request open for long.
func setJobWithRetry(ctx context.Context, db *database.MongoDB, build *models.Build, name, namespace string) error {
	const attempts = 3
	var err error
	for i := 0; i < attempts; i++ {
		if err = build.SetJob(ctx, db, name, namespace); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
	}
	return err
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

func ptrBool(b bool) *bool    { return &b }
func ptrInt64(i int64) *int64 { return &i }

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
//
// All terminal writes go through MarkTerminalIfOwner: if the lease was
// stolen mid-poll (e.g. a slow deploy outran the 60s TTL), the rightful
// owner will redo this work and we must not double-write.
func (a *ApiHandler) pollBuildJobOnce(ctx context.Context, build *models.Build) (bool, error) {
	cli, err := a.kubeClient()
	if err != nil {
		return false, err
	}
	job, err := cli.K8s.BatchV1().Jobs(build.JobNamespace).Get(ctx, build.JobName, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", "build Job not found")
			return true, nil
		}
		return false, err
	}

	if job.Status.Succeeded > 0 {
		application := &models.Application{}
		if err := application.GetByID(a.MongoDB, build.ApplicationID.Hex()); err != nil {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", fmt.Sprintf("loading application: %v", err))
			return true, nil
		}
		component := &models.Component{}
		if err := component.GetByID(a.MongoDB, build.ComponentID.Hex()); err != nil {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", fmt.Sprintf("loading component: %v", err))
			return true, nil
		}
		envName := envNameByID(application, build.EnvironmentID)
		if envName == "" {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", "environment not found")
			return true, nil
		}
		envObj, err := application.GetEnvironmentByName(a.MongoDB, envName)
		if err != nil || envObj == nil {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", "environment not found")
			return true, nil
		}
		// The deploy can outrun the 60s lease TTL (CRD apply + Mongo
		// writes are not bounded). Keep the lease alive while it runs
		// so the adoption scanner can't steal the build mid-deploy and
		// produce duplicate deployed revisions.
		stopHeartbeat := a.heartbeatDuringDeploy(ctx, build)
		deployErr := deployBuildImage(ctx, a, application, envObj, component, build.ImageRef, build.CreatedBy)
		stillOwner := stopHeartbeat()
		if !stillOwner {
			// Lease was stolen despite the keep-alive (e.g. Mongo
			// outage froze the heartbeat). Don't touch terminal
			// state — the new owner will redo the deploy.
			log.Printf("watcher %s: lease lost during deploy; abandoning", build.ID.Hex())
			return true, nil
		}
		if deployErr != nil {
			a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", fmt.Sprintf("deploying: %v", deployErr))
			return true, nil
		}
		a.markBuildTerminalGuarded(ctx, build, models.BuildStatusSucceeded, build.ImageRef, "")
		return true, nil
	}
	if job.Status.Failed > 0 {
		msg := tailBuildPodLogs(ctx, a, build.JobNamespace, build.JobName)
		if msg == "" {
			msg = "build failed; see job logs"
		}
		a.markBuildTerminalGuarded(ctx, build, models.BuildStatusFailed, "", msg)
		return true, nil
	}
	return false, nil
}

// markBuildTerminalGuarded wraps MarkTerminalIfOwner with a uniform log
// message. Tracks the "lease stolen mid-poll" case as a debug signal so
// duplicate-write attempts are visible in production.
func (a *ApiHandler) markBuildTerminalGuarded(ctx context.Context, build *models.Build, status models.BuildStatus, imageRef, errMsg string) {
	ok, err := build.MarkTerminalIfOwner(ctx, a.MongoDB, a.WatcherID, status, imageRef, errMsg)
	if err != nil {
		log.Printf("watcher %s: marking terminal: %v", build.ID.Hex(), err)
		return
	}
	if !ok {
		log.Printf("watcher %s: skipped terminal write (lease lost or build already terminal)", build.ID.Hex())
	}
}

// heartbeatDuringDeploy keeps the build's lease alive while a slow
// in-handler operation runs. Returns a stop function that cancels the
// heartbeat goroutine and reports whether the lease is still held.
//
// Heartbeats fire at buildHeartbeat (20s); the lease TTL is 60s, so a
// single missed write is survivable. A stop() call returning false means
// some heartbeat saw the lease was lost — the caller must NOT mutate
// terminal state.
func (a *ApiHandler) heartbeatDuringDeploy(ctx context.Context, build *models.Build) func() bool {
	hbCtx, cancel := context.WithCancel(ctx)
	lost := make(chan struct{})
	var lostFlag atomic.Bool
	go func() {
		t := time.NewTicker(buildHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				ok, err := build.Heartbeat(hbCtx, a.MongoDB, a.WatcherID, buildLeaseTTL)
				if err != nil {
					log.Printf("watcher %s: deploy-time heartbeat error: %v", build.ID.Hex(), err)
					continue
				}
				if !ok {
					lostFlag.Store(true)
					close(lost)
					return
				}
			}
		}
	}()
	return func() bool {
		cancel()
		select {
		case <-lost:
			return false
		default:
			return !lostFlag.Load()
		}
	}
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
