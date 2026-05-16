package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/coffeenights/conure/internal/cli/apiclient"
	"github.com/coffeenights/conure/internal/cli/ui"
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
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	lc, err := resolveTarget(cmd)
	if err != nil {
		return err
	}

	imageRef, _ := cmd.Flags().GetString("image-ref")
	if imageRef == "" {
		// Backwards-compatible: no image, just promote latest draft.
		sp := ui.StartSpinner(fmt.Sprintf("Deploying `%s` to `%s`…", lc.Link.ComponentName, lc.Env))
		rev, err := lc.Client.DeployLatestDraft(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
		ui.StopSpinner(sp)
		if err != nil {
			return err
		}
		ui.Success("✓ Deployed v%d (%s) to %s\n", rev.Version, rev.ID, lc.Env)
		return nil
	}

	flagLocation, _ := cmd.Flags().GetString("build-location")
	flagBuildTool, _ := cmd.Flags().GetString("build-tool")
	platform, _ := cmd.Flags().GetString("platform")
	flagGitRepo, _ := cmd.Flags().GetString("git-repository")
	flagGitBranch, _ := cmd.Flags().GetString("git-branch")
	ctxDir, _ := cmd.Flags().GetString("context")

	// Read the component's spec from the latest revision; it's the source
	// of truth for buildTool/buildLocation/gitRepository/gitBranch. Flags,
	// when set, override the spec for this one deploy. Failures here are
	// non-fatal: we fall back to flag-only resolution and a hardcoded
	// dockerfile/remote default.
	spec := readComponentSourceSpec(cmd, lc)

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
			return errors.New("--build-location=remote requires --git-repository (the server clones it inside the build Job)")
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
		if err == nil {
			go func() {
				defer body.Close()
				_, _ = io.Copy(os.Stdout, body)
			}()
			logsStarted = true
			break
		}
		time.Sleep(2 * time.Second)
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

// componentSourceSpec carries the build-related fields the CLI cares
// about from a component's latest revision values. Empty fields mean
// "not set on the spec" — fall back to flags or hardcoded defaults.
type componentSourceSpec struct {
	BuildTool     string
	BuildLocation string
	GitRepository string
	GitBranch     string
}

// readComponentSourceSpec fetches the linked component's env-scoped view
// and extracts `source.{buildTool,buildLocation,gitRepository,gitBranch}`
// from the values blob. Preference order: latest draft (the user may
// have prepped non-image edits there) → deployed revision. Errors are
// logged at info level and produce a zero spec so the caller can fall
// back to flag-only defaults — `conure deploy --image-ref` must keep
// working even when the API is temporarily unreachable.
func readComponentSourceSpec(cmd *cobra.Command, lc *linkedCtx) componentSourceSpec {
	view, err := lc.Client.GetComponentInEnv(cmd.Context(), lc.Link.OrgID, lc.Link.AppID, lc.Env, lc.Link.ComponentID)
	if err != nil || view == nil {
		if err != nil {
			ui.Info("Could not read component spec for defaults: %v\n", err)
		}
		return componentSourceSpec{}
	}
	values := pickValues(view)
	src, _ := values["source"].(map[string]interface{})
	if src == nil {
		return componentSourceSpec{}
	}
	return componentSourceSpec{
		BuildTool:     stringFromValues(src, "buildTool"),
		BuildLocation: stringFromValues(src, "buildLocation"),
		GitRepository: stringFromValues(src, "gitRepository"),
		GitBranch:     stringFromValues(src, "gitBranch"),
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

func stringFromValues(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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
