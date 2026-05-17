package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/ui"
	"github.com/coffeenights/conure/internal/fieldroles"
	"github.com/coffeenights/conure/pkg/api"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy this component — promote latest draft, or build a new image then deploy",
	Long: `Deploy the linked component.

In its simplest form 'conure deploy' promotes the latest draft revision to
deployed (the prior behavior, kept for backward compatibility):

    conure deploy

With --image-ref, a new image build is recorded as part of the deploy. The
build runs either locally on this machine (Docker/buildx or railpack) or
remotely as a BuildKit Job in the cluster, controlled by --build-location:

    # Cross-build for the cluster from an M-series Mac
    conure deploy --image-ref ghcr.io/me/app:sha-abc --build-location local

    # Server-side build from a git repo (the API runs a BuildKit Job)
    conure deploy --image-ref ghcr.io/me/app:sha-abc --build-location remote \
                  --git-repository https://github.com/me/app --git-branch main

--platform overrides the build target architecture. When omitted, the CLI
queries the API for the cluster's dominant node platform and uses that —
so a developer on darwin/arm64 building for an amd64 cluster automatically
gets a linux/amd64 image, no flag required.`,
	RunE: runDeploy,
}

func init() {
	addEnvFlag(deployCmd)
	deployCmd.Flags().String("image-ref", "", "Image to push/deploy (e.g. ghcr.io/org/app:sha-abc). When set, runs a build before deploying.")
	// All build-shape flags default to empty: when unset, the CLI reads
	// the values from the component's latest revision (`source.buildTool`,
	// `source.buildLocation`, `source.gitRepository`, `source.gitBranch`).
	// Pass a flag explicitly to override the spec for one deploy.
	deployCmd.Flags().String("build-location", "", "Override component's source.buildLocation: local, remote, or auto.")
	deployCmd.Flags().String("build-tool", "", "Override component's source.buildTool: dockerfile or railpack.")
	deployCmd.Flags().String("platform", "", "Target platform (e.g. linux/amd64). Defaults to the cluster's dominant node platform via /system/info.")
	deployCmd.Flags().String("git-repository", "", "Override component's source.gitRepository (remote builds only).")
	deployCmd.Flags().String("git-branch", "", "Override component's source.gitBranch (remote builds only).")
	deployCmd.Flags().String("context", ".", "Local build context directory (for local builds).")
	deployCmd.Flags().Bool("approve", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}

	flagImageRef, _ := cmd.Flags().GetString("image-ref")
	flagLocation, _ := cmd.Flags().GetString("build-location")
	flagBuildTool, _ := cmd.Flags().GetString("build-tool")
	platform, _ := cmd.Flags().GetString("platform")
	flagGitRepo, _ := cmd.Flags().GetString("git-repository")
	flagGitBranch, _ := cmd.Flags().GetString("git-branch")
	ctxDir, _ := cmd.Flags().GetString("context")

	// Resolve the component's ComponentDefinition and read its build spec
	// BEFORE deciding build-vs-promote. The definition owns whether this
	// component type can build (Buildable) and where the image/build
	// fields live in its values (fieldRoles); the per-component sourceType
	// discriminator decides whether this instance builds. This is the
	// source of truth — flags only override individual fields for one
	// deploy. There is no fallback: an unresolvable definition is fatal
	// (this platform is pre-1.0; definitions must declare their roles).
	spec, err := readComponentSourceSpec(cmd, lc)
	if err != nil {
		return err
	}

	action, imageRef, err := decideDeployAction(spec, flagImageRef, lc.Link.ComponentName)
	if err != nil {
		return err
	}

	approve, _ := cmd.Flags().GetBool("approve")
	if !approve {
		var summary string
		switch action {
		case deployActionBuild:
			summary = fmt.Sprintf("build and deploy `%s` (image %s)", lc.Link.ComponentName, imageRef)
		default:
			summary = fmt.Sprintf("deploy the latest draft of `%s`", lc.Link.ComponentName)
		}
		ui.Error("This will %s to `%s`.\n", summary, lc.Env)
		var ok bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Deploy %s to %s?", lc.Link.ComponentName, lc.Env)).
			Affirmative("Deploy").
			Negative("Cancel").
			Value(&ok).
			Run(); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}

	if action == deployActionPromote {
		// Promote-only: push the latest draft to deployed. Correct for
		// non-buildable components and prebuilt-image (sourceType=oci)
		// components with no explicit --image-ref override.
		sp := ui.StartSpinner(fmt.Sprintf("Deploying `%s` to `%s`…", lc.Link.ComponentName, lc.Env))
		rev, err := lc.Client.DeployLatestDraft(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
		ui.StopSpinner(sp)
		if err != nil {
			return err
		}
		ui.Success("✓ Deployed v%d (%s) to %s\n", rev.Version, rev.ID, lc.Env)
		return nil
	}

	buildTool := firstNonEmpty(flagBuildTool, spec.BuildTool, "dockerfile")
	location := firstNonEmpty(flagLocation, spec.BuildLocation)
	gitRepo := firstNonEmpty(flagGitRepo, spec.GitRepository)
	gitBranch := firstNonEmpty(flagGitBranch, spec.GitBranch, "main")

	// Resolve the target platform up front. The cluster owns the answer;
	// we only fall back to local arch when /system/info can't be reached
	// (offline / unauth) — so the error path here is informational, not
	// fatal. The fallback always pins os=linux: the cluster runs Linux
	// containers, and the host OS only affects the local builder
	// (darwin/arm64 isn't a valid container platform — buildx would
	// reject it or produce an image the cluster can't run).
	clusterPlatform, kubeVer := resolveClusterPlatform(cmd, lc.Client)
	if platform == "" {
		if clusterPlatform != "" {
			platform = clusterPlatform
		} else {
			platform = "linux/" + runtime.GOARCH
			ui.Info("Could not detect cluster platform; defaulting to %s (host arch, assuming linux)\n", platform)
		}
	}

	// Resolve build-location. Empty (no flag, no spec value) and "auto"
	// both run the same picker. The picker prefers local when this
	// machine can target the cluster platform via buildx, otherwise
	// falls back to remote.
	resolvedLocation := location
	if resolvedLocation == "" || resolvedLocation == "auto" {
		resolvedLocation = pickBuildLocation(clusterPlatform, gitRepo)
	}

	switch resolvedLocation {
	case "local":
		// Railpack-as-local is allowed. Dockerfile-as-local uses buildx.
		// Both produce a pushed image at imageRef.
		if buildTool == "railpack" && !commandExists("railpack") {
			return fmt.Errorf("--build-tool=railpack requires the `railpack` CLI on PATH; install from https://railpack.com or use --build-tool=dockerfile")
		}
		if buildTool == "dockerfile" {
			if !commandExists("docker") {
				return fmt.Errorf("--build-tool=dockerfile requires Docker on PATH")
			}
			// runLocalBuild always invokes `docker buildx build`, so the
			// plugin must be present even on same-arch hosts. Surface this
			// up front instead of letting docker emit a cryptic error.
			if !hasBuildx() {
				return fmt.Errorf("--build-tool=dockerfile (local) requires the Docker buildx plugin; install it (https://docs.docker.com/build/buildx/install/) or use --build-location=remote")
			}
		}
		if err := runLocalBuild(cmd, buildTool, ctxDir, imageRef, platform); err != nil {
			return err
		}
		// Record the build with the API and let it deploy.
		sp := ui.StartSpinner(fmt.Sprintf("Recording build and deploying %s…", imageRef))
		b, err := lc.Client.TriggerBuild(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, api.TriggerBuildRequest{
			BuildTool:     buildTool,
			BuildLocation: "local",
			Platform:      platform,
			ImageRef:      imageRef,
		})
		ui.StopSpinner(sp)
		if err != nil {
			return err
		}
		ui.Success("✓ Built locally for %s and deployed to %s (build %s)\n", platform, lc.Env, b.ID)
		_ = kubeVer
		return nil

	case "remote":
		if gitRepo == "" {
			return errors.New("remote build needs a git repository (the server clones it inside the build Job): set the component spec's git.repository field role or pass --git-repository")
		}
		if buildTool == "railpack" {
			return errors.New("railpack is local-only (the remote BuildKit Job ships only the dockerfile frontend). Use --build-tool=dockerfile or --build-location=local.")
		}
		sp := ui.StartSpinner(fmt.Sprintf("Triggering remote build %s@%s…", gitRepo, gitBranch))
		b, err := lc.Client.TriggerBuild(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, api.TriggerBuildRequest{
			BuildTool:     buildTool,
			BuildLocation: "remote",
			Platform:      platform,
			GitRepository: gitRepo,
			GitBranch:     gitBranch,
			ImageRef:      imageRef,
		})
		ui.StopSpinner(sp)
		if err != nil {
			return err
		}
		ui.Info("Build %s queued (status: %s)\n", b.ID, b.Status)
		return tailRemoteBuild(cmd, lc, b.ID)

	default:
		return fmt.Errorf("unknown --build-location %q (expected: local, remote, auto)", location)
	}
}

// resolveClusterPlatform queries /system/info to discover the cluster's
// dominant node platform. Returns ("", "") on any error so the caller can
// fall back without aborting the deploy.
func resolveClusterPlatform(cmd *cobra.Command, c *apiclient.Client) (string, string) {
	info, err := c.SystemInfo(cmd.Context())
	if err != nil || info == nil {
		return "", ""
	}
	return info.Platform, info.KubernetesVersion
}

// pickBuildLocation implements --build-location=auto. Rules:
//
//   - User supplied --git-repository → remote (they clearly want the server
//     to handle it).
//   - Local arch differs from cluster arch and we don't have buildx → remote.
//   - Otherwise local (faster feedback loop; uses local Docker cache).
//
// We treat "buildx present" as the cross-build capability check; modern
// Docker desktop ships it by default, so this is a reliable signal.
func pickBuildLocation(clusterPlatform, gitRepo string) string {
	return pickBuildLocationWith(clusterPlatform, gitRepo, hasBuildx)
}

// pickBuildLocationWith is the testable core of pickBuildLocation. The
// buildx-presence probe is injected so unit tests don't depend on the host's
// Docker installation.
func pickBuildLocationWith(clusterPlatform, gitRepo string, hasBuildxFn func() bool) string {
	if gitRepo != "" {
		return "remote"
	}
	// runLocalBuild for dockerfile always uses `docker buildx build`, so a
	// host without buildx can't do local-dockerfile at all — same-arch or
	// not. Route to remote in that case so auto doesn't pick a path that
	// will fail at exec time.
	if !hasBuildxFn() {
		return "remote"
	}
	if clusterPlatform == "" {
		return "local"
	}
	localPlat := runtime.GOOS + "/" + runtime.GOARCH
	if normalizePlatform(localPlat) == normalizePlatform(clusterPlatform) {
		return "local"
	}
	// Cross-build needed; buildx is already confirmed above.
	return "local"
}

// normalizePlatform collapses darwin/arm64 → linux/arm64 for the comparison.
// The local OS is irrelevant once buildx is producing a linux image; only
// arch matters.
func normalizePlatform(p string) string {
	parts := strings.SplitN(p, "/", 2)
	if len(parts) != 2 {
		return p
	}
	return "linux/" + parts[1]
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hasBuildx() bool {
	if !commandExists("docker") {
		return false
	}
	out, err := exec.Command("docker", "buildx", "version").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "buildx")
}

// runLocalBuild shells out to the chosen builder. Output is streamed to
// stdout/stderr so the user sees the build progress in real time.
func runLocalBuild(cmd *cobra.Command, tool, ctxDir, imageRef, platform string) error {
	var args []string
	var bin string
	switch tool {
	case "dockerfile":
		bin = "docker"
		args = []string{"buildx", "build", "--platform", platform, "--push", "-t", imageRef, ctxDir}
	case "railpack":
		bin = "railpack"
		args = []string{"build", ctxDir, "--push", "--tag", imageRef, "--platform", platform}
	default:
		return fmt.Errorf("unknown build tool %q", tool)
	}
	ui.Info("Running: %s %s\n", bin, strings.Join(args, " "))
	c := exec.CommandContext(cmd.Context(), bin, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("local build failed: %w", err)
	}
	return nil
}

// tailRemoteBuild streams logs from the remote build's pod and polls until
// the build reaches a terminal state. Returns nil on success, non-nil on
// build failure so the CLI exits non-zero in CI.
func tailRemoteBuild(cmd *cobra.Command, lc *linkedCtx, buildID string) error {
	// Logs may not be available immediately — the pod has to schedule.
	// Try every 2s for up to 30s before giving up on log streaming, but
	// keep polling status either way.
	ctx := cmd.Context()
	logsStarted := false
	deadline := time.Now().Add(30 * time.Second)
	for !logsStarted && time.Now().Before(deadline) {
		body, err := lc.Client.StreamBuildLogs(ctx, lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, buildID, true)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		// Defensive backstop: the server should signal "not ready" with a
		// non-2xx (handled by the err branch above), but a regression that
		// returns 200 with an error body would otherwise be indistinguishable
		// from real logs. Peek the first chunk; if it's the not-started
		// sentinel, treat it as retryable instead of streaming one error
		// line and giving up. A single Read (not io.ReadFull) returns as
		// soon as any data is available, so this never withholds a slow
		// build's early output; the server writes the sentinel in one
		// Fprintf, well within the first chunk.
		head := make([]byte, 512)
		n, rerr := body.Read(head)
		head = head[:n]
		if logsNotReadySentinel(head) {
			body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		first := head
		stream := body
		go func() {
			defer stream.Close()
			if len(first) > 0 {
				_, _ = os.Stdout.Write(first)
			}
			// rerr is the error from the peek Read: io.EOF means the body
			// was fully consumed by that one Read; anything else non-nil
			// means the stream is already broken. Only keep copying if the
			// stream is still healthy.
			if rerr == nil {
				_, _ = io.Copy(os.Stdout, stream)
			}
		}()
		logsStarted = true
		break
	}

	// Poll status until terminal. The log stream closes once the pod
	// exits, but the API marks the build terminal only after the deploy
	// step finishes — so we poll status as the source of truth.
	for {
		b, err := lc.Client.GetBuild(ctx, lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID, buildID)
		if err != nil {
			return err
		}
		switch b.Status {
		case api.BuildStatusSucceeded:
			ui.Success("\n✓ Remote build succeeded; deployed %s to %s\n", b.ImageRef, lc.Env)
			return nil
		case api.BuildStatusFailed:
			if b.ErrorMessage != "" {
				ui.Error("\n✗ Remote build failed: %s\n", b.ErrorMessage)
			} else {
				ui.Error("\n✗ Remote build failed\n")
			}
			return fmt.Errorf("build %s failed", buildID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// logsNotReadySentinel reports whether a streamed log chunk is actually the
// server's "container not started yet" error written into a 200 body. This
// is a backstop for a server regression — the current server returns 425 for
// this case (handled by the HTTP error path), but matching the sentinel
// keeps the retry loop correct if that ever changes. Mirrors the string in
// cmd/api-server/applications/builds.go.
func logsNotReadySentinel(chunk []byte) bool {
	return bytes.Contains(chunk, []byte("[error] opening log stream"))
}

// componentSourceSpec carries the build-related fields the CLI resolves
// from a component's values via its ComponentDefinition's fieldRoles.
// Buildable/SourceType drive the build-vs-promote decision; the rest
// feed the build request. A field is empty only when the spec genuinely
// doesn't set it (the field role is declared but unset in values) —
// never because a role was missing, which is a hard error instead.
type componentSourceSpec struct {
	Buildable     bool
	SourceType    string // "git" | "oci" | "" (only when not buildable)
	OCIRepository string
	Tag           string
	BuildTool     string
	BuildLocation string
	GitRepository string
	GitBranch     string
}

// deployAction is the build-vs-promote decision for one `conure deploy`.
type deployAction int

const (
	// deployActionPromote: no build; push the latest draft to deployed.
	deployActionPromote deployAction = iota
	// deployActionBuild: build/record an image, then deploy it.
	deployActionBuild
)

// decideDeployAction is the pure core of `conure deploy`'s build-vs-promote
// branch, factored out so the matrix is unit-testable without an API client.
//
//   - flagImageRef set: build (explicit override). Fatal if the component
//     isn't buildable — the user asked for something the type can't do.
//   - not buildable: promote-only.
//   - buildable + sourceType=git: build; image-ref is the spec's
//     image.repository:image.tag (must be set — nothing to push to otherwise).
//   - buildable + sourceType=oci: promote-only (the prebuilt image is
//     already named in the spec; there is nothing to build).
//
// Returns the action and, for a build, the resolved image-ref.
func decideDeployAction(spec componentSourceSpec, flagImageRef, componentName string) (deployAction, string, error) {
	if flagImageRef != "" {
		if !spec.Buildable {
			return 0, "", fmt.Errorf("component type %q is not buildable, but --image-ref was given; remove --image-ref to promote the latest draft, or use a buildable component definition", componentName)
		}
		return deployActionBuild, flagImageRef, nil
	}
	if !spec.Buildable {
		return deployActionPromote, "", nil
	}
	if spec.SourceType != fieldroles.SourceTypeGit {
		// sourceType=oci: deploy the prebuilt image via promote.
		return deployActionPromote, "", nil
	}
	imageRef := spec.ImageRef()
	if imageRef == "" {
		return 0, "", errors.New("component spec does not set the image (image.repository + image.tag field roles) and no --image-ref was given; cannot build without a target image")
	}
	return deployActionBuild, imageRef, nil
}

// ImageRef returns the spec's image as `ociRepository:tag`, or "" when
// either half is unset (an incomplete spec can't name an image).
func (s componentSourceSpec) ImageRef() string {
	if s.OCIRepository == "" || s.Tag == "" {
		return ""
	}
	return s.OCIRepository + ":" + s.Tag
}

// readComponentSourceSpec fetches the linked component's env-scoped view,
// resolves its ComponentDefinition (join key: type + optional engine) to
// learn whether it's buildable and where its build/image fields live, and
// reads those fields through the fieldroles resolver.
//
// There is no graceful degradation: an unreachable API, an unresolvable
// definition, an undeclared role that we need, or a malformed sourceType
// is fatal. The platform is pre-1.0 — silently falling back to a guessed
// `source.*` layout is exactly the bug this change removes.
func readComponentSourceSpec(cmd *cobra.Command, lc *linkedCtx) (componentSourceSpec, error) {
	view, err := lc.Client.GetComponentInEnv(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil {
		return componentSourceSpec{}, fmt.Errorf("reading component spec: %w", err)
	}
	if view == nil {
		return componentSourceSpec{}, errors.New("reading component spec: empty response")
	}

	resolver, err := resolveComponentFieldRoles(cmd, lc, view)
	if err != nil {
		return componentSourceSpec{}, err
	}

	values := pickValues(view)
	if values == nil {
		values = map[string]interface{}{}
	}

	spec := componentSourceSpec{Buildable: resolver.Buildable()}
	if !resolver.Buildable() {
		// Nothing to read: a non-buildable definition need not declare
		// image/build roles, and the caller will promote-only.
		return spec, nil
	}

	spec.SourceType, err = resolver.SourceType(values)
	if err != nil {
		return componentSourceSpec{}, fmt.Errorf("component %q: %w", lc.Link.ComponentName, err)
	}

	// image.* is always meaningful for a buildable component (git builds
	// push here; oci deploys pull from here). Read leniently: a declared
	// role with an unset value is fine (the user may pass --image-ref).
	if spec.OCIRepository, err = getRole(resolver, values, fieldroles.RoleImageRepository); err != nil {
		return componentSourceSpec{}, err
	}
	if spec.Tag, err = getRole(resolver, values, fieldroles.RoleImageTag); err != nil {
		return componentSourceSpec{}, err
	}

	// git.*/build.* only matter for a git build. For an oci component the
	// definition may not declare them at all, so don't read them.
	if spec.SourceType == fieldroles.SourceTypeGit {
		if spec.GitRepository, err = getRole(resolver, values, fieldroles.RoleGitRepository); err != nil {
			return componentSourceSpec{}, err
		}
		if spec.GitBranch, err = getRole(resolver, values, fieldroles.RoleGitBranch); err != nil {
			return componentSourceSpec{}, err
		}
		if spec.BuildTool, err = getRole(resolver, values, fieldroles.RoleBuildTool); err != nil {
			return componentSourceSpec{}, err
		}
		if spec.BuildLocation, err = getRole(resolver, values, fieldroles.RoleBuildLocation); err != nil {
			return componentSourceSpec{}, err
		}
	}
	return spec, nil
}

// getRole reads a role's value, treating "declared but unset in values"
// as an empty string (the caller layers flags/defaults on top) but a
// missing role declaration or a type/shape error as fatal.
func getRole(r *fieldroles.Resolver, values map[string]interface{}, role string) (string, error) {
	v, _, err := r.Get(values, role)
	if err != nil {
		return "", fmt.Errorf("field role %q: %w", role, err)
	}
	return v, nil
}

// resolveComponentFieldRoles finds the ComponentDefinition backing the
// component (matched by view.Type + optional view.Engine) and builds a
// fieldroles resolver from its Buildable flag and FieldRoles map.
func resolveComponentFieldRoles(cmd *cobra.Command, lc *linkedCtx, view *api.ComponentInEnvResponse) (*fieldroles.Resolver, error) {
	if view.Type == "" {
		return nil, fmt.Errorf("component %q has no type recorded; cannot resolve its ComponentDefinition", lc.Link.ComponentName)
	}
	defs, err := lc.Client.ListComponentDefinitions(cmd.Context(), lc.Link.OrgID)
	if err != nil {
		return nil, fmt.Errorf("listing component definitions: %w", err)
	}
	var matches []api.ComponentDefinition
	for _, d := range defs {
		if d.Type != view.Type {
			continue
		}
		if view.Engine != "" && d.Engine != "" && d.Engine != view.Engine {
			continue
		}
		matches = append(matches, d)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no ComponentDefinition for component type %q (engine %q); cannot resolve field roles", view.Type, view.Engine)
	case 1:
		return fieldroles.New(matches[0].Buildable, matches[0].FieldRoles), nil
	default:
		return nil, fmt.Errorf("%d ComponentDefinitions match type %q; the component must pin an engine to disambiguate", len(matches), view.Type)
	}
}

// pickValues prefers the latest draft over the deployed revision: the
// draft is where unflushed edits live, so the spec the user just
// authored takes effect on this deploy.
func pickValues(view *api.ComponentInEnvResponse) map[string]interface{} {
	if view.LatestDraft != nil && view.LatestDraft.Values != nil {
		return view.LatestDraft.Values
	}
	if view.DeployedRevision != nil && view.DeployedRevision.Values != nil {
		return view.DeployedRevision.Values
	}
	return nil
}

// firstNonEmpty returns the first non-empty argument. Used to layer
// flag > spec > hardcoded default with a single readable line per
// build-related field.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
