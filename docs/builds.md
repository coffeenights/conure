# Builds

Conure builds container images and deploys them in a single CLI step. Two
build modes are supported:

> **Before you start:** you need a container registry the cluster can pull
> from. See [container-registries.md](container-registries.md) for picking
> one (GHCR, ECR/GAR/ACR, Forgejo, Harbor, …) and wiring the push
> credentials.

| Mode      | Where the build runs              | Triggered by                    | Frontends           |
|-----------|-----------------------------------|---------------------------------|---------------------|
| `local`   | The developer's machine           | `conure deploy --image-ref ...` | Dockerfile, Railpack |
| `remote`  | A BuildKit Job in `conure-system` | `conure deploy --image-ref ...` | Dockerfile (Railpack guarded — see [Railpack](#railpack)) |

The build shape (tool + location + git repo + branch) lives on the
component's spec under `source.*`. The CLI reads those values from the
latest revision and you only need to pass `--image-ref` for a standard
deploy. Every other CLI flag is an override for one-off deploys — see
*Flags* below.

## CLI usage

```bash
# Backwards-compatible: promote the latest draft (no image build)
conure deploy

# Build using the component's `source.buildTool` + `source.buildLocation`
conure deploy --image-ref ghcr.io/me/app:sha-abc

# One-off: force a remote build even if the spec says local
conure deploy --image-ref ghcr.io/me/app:sha-abc --build-location remote

# Override the git branch for this deploy
conure deploy --image-ref ghcr.io/me/app:sha-abc --git-branch hotfix-123
```

### Where defaults come from

Each build field is resolved in the order **flag → component spec → hardcoded fallback**:

| Field             | Spec source                | Hardcoded fallback                    |
|-------------------|----------------------------|---------------------------------------|
| `build-tool`      | `source.buildTool`         | `dockerfile`                          |
| `build-location`  | `source.buildLocation`     | `auto` (see *Architecture detection*) |
| `git-repository`  | `source.gitRepository`     | —                                     |
| `git-branch`      | `source.gitBranch`         | `main`                                |
| `platform`        | — (cluster-derived)        | host `GOOS/GOARCH`                    |

The "spec source" is read from the latest draft revision when present,
otherwise from the last deployed revision.

### Flags

| Flag                | Default          | Notes                                                                                |
|---------------------|------------------|--------------------------------------------------------------------------------------|
| `--image-ref`       | —                | Target image (`registry/repo:tag`). Triggers a build when set.                       |
| `--build-location`  | spec / `auto`    | `local`, `remote`, or `auto`. Overrides `source.buildLocation`.                      |
| `--build-tool`      | spec / `dockerfile` | `dockerfile` or `railpack`. Overrides `source.buildTool`.                         |
| `--platform`        | from `/system/info` | Cluster's dominant node platform.                                                 |
| `--git-repository`  | spec             | Overrides `source.gitRepository` (remote builds only).                               |
| `--git-branch`      | spec / `main`    | Overrides `source.gitBranch` (remote builds only).                                   |
| `--context`         | `.`              | Local build context directory.                                                       |

## Architecture detection

A developer on an Apple Silicon Mac (`darwin/arm64`) deploying to an
`amd64` cluster must not push an `arm64` image — the pod won't start.
Conure handles this for you:

1. The CLI calls `GET /system/info`, which returns the cluster's dominant
   node platform (e.g. `linux/amd64`) computed from `kubernetes.io/arch`
   labels.
2. `--platform` defaults to that value. Both build tools pass it to
   `docker buildx build --platform linux/amd64 --push`, producing the
   right image regardless of the host architecture (Railpack builds also
   go through buildx — see [Railpack](#railpack)).
3. `--build-location auto` further refines the decision:

   | Local arch == cluster arch? | `docker buildx` present? | `--git-repository` set? | Result    |
   |-----------------------------|--------------------------|-------------------------|-----------|
   | yes                         | (n/a)                    | no                      | `local`   |
   | no                          | yes                      | no                      | `local` (cross-build) |
   | no                          | no                       | no                      | `remote`  |
   | (any)                       | (any)                    | yes                     | `remote`  |

The presence of `--git-repository` is treated as a strong signal that the
user wants a remote build.

## Local builds

The CLI shells out to:

- `docker buildx build --platform <platform> --push -t <image-ref> <context>`
  when `--build-tool=dockerfile` (default if a `Dockerfile` exists).
- For `--build-tool=railpack`, a **two-step** sequence — see
  [Railpack](#railpack) below for why. In short: `railpack prepare` emits a
  plan, then `docker buildx build` consumes it via Railpack's BuildKit
  gateway frontend with the same `--push`. Requires both the
  [Railpack CLI](https://railpack.com) **and** Docker + buildx on `PATH`.

After the build pushes successfully, the CLI calls
`POST .../builds` with `build_location: local` and the image ref. The API
records the build as `succeeded`, writes a new deployed revision with
`source.ociRepository` and `source.tag` overridden, and applies the
Component CRD synchronously.

> **Railpack is currently local-only.** The API rejects
> `build_tool=railpack` for `build_location=remote` today. This is a
> guard, not a hard limitation — see [Railpack](#railpack) for why and the
> planned design to lift it.

## Remote builds

The API creates a Kubernetes `Job` in `conure-system`:

```
initContainers:
  git-clone   (alpine/git:latest)  →  git clone --branch <branch> --depth 1 <repo> /workspace
containers:
  build       (moby/buildkit:latest, privileged: true)
              buildkitd & buildctl build --frontend dockerfile.v0 \
                                          --output type=image,name=<image-ref>,push=true
volumes:
  workspace        (emptyDir)
  registry-creds   (Secret: registry-credentials at /root/.docker)
ttlSecondsAfterFinished: 3600
```

### Cluster prerequisites

1. **Privileged pods** must be allowed in `conure-system` (rootful
   buildkitd requires it). If your cluster enforces Pod Security Standards
   "restricted", label the namespace `pod-security.kubernetes.io/enforce=privileged`.
2. **Registry credentials**: create a `dockerconfigjson` Secret named
   `registry-credentials` in `conure-system`:

   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=ghcr.io \
     --docker-username=<user> \
     --docker-password=<token> \
     --namespace conure-system
   ```

3. **RBAC**: the API ServiceAccount needs `jobs` create/get/list/watch/delete,
   `pods` get/list, and `pods/log` get/list in `conure-system`. These are
   included in the Helm chart's `api-rbac.yaml`.

### Lifecycle

```
POST .../builds {build_location:remote}
  ─► API creates Build (status=pending)
  ─► API creates Job in conure-system
  ─► API records JobName/JobNamespace, status=building
  ─► API acquires a Mongo lease on the build, spawns watcher goroutine
  ─► 202 Accepted with the Build record
```

The CLI polls `GET .../builds/:id` until `status` is `succeeded` or
`failed`, and streams `GET .../builds/:id/logs` in parallel.

When the Job succeeds the watcher writes a new deployed revision (same
image-ref merge as the local path) and marks the build `succeeded`. On
failure it tails the build pod's last lines into `error_message`.

## Railpack

[Railpack](https://railpack.com) is a Dockerfile-less builder: it analyses
a source tree, infers the language/toolchain, and produces an image. Conure
supports it as an alternative to a hand-written `Dockerfile`. This section
documents how it actually works inside conure — it is **not** a drop-in
`docker build` replacement, and several intuitive assumptions are wrong.
Everything here was verified empirically against `railpack` 0.23.0.

### `railpack build` cannot push (the core constraint)

The single most important fact, and the source of a fixed bug:

> **`railpack build` has no `--push` and no `--tag`.** Its flags are
> `--name`, `--output`, `--platform`, `--progress`, `--show-plan`,
> `--cache-key`, `--env`, `--previous`, `--build-cmd`, `--start-cmd`,
> `--config-file`. It builds into the **local Docker image store** (named
> via `--name`) or dumps the filesystem locally (`--output`). It has no
> registry-push code path.

Confirmed three ways: the `railpack --help` output, Railpack's own source
(`cli/build.go` calls `BuildWithBuildkitClient` with only `ImageName` /
`OutputDir`), and the official docs. A previous conure CLI ran
`railpack build . --push --tag X --platform Y` — three of those four flags
do not exist; it never worked.

Railpack is deliberately a **build-plan generator + BuildKit gateway
frontend**, not a Docker replacement. Pushing, multi-arch, auth, and
tagging are BuildKit/buildx's job by design.

### The supported build-and-push path

```bash
# 1. Generate the build plan the frontend consumes (--info-out is
#    optional and unused by the buildx step, so it is omitted)
railpack prepare <dir> --plan-out plan.json

# 2. Build & push via buildx using Railpack's gateway frontend
docker buildx build \
  --build-arg BUILDKIT_SYNTAX=ghcr.io/railwayapp/railpack-frontend:latest \
  -f plan.json \
  --platform <platform> \
  --push -t <image-ref> \
  <dir>
```

This is the exact `docker buildx build … --push -t …` the `dockerfile`
tool already uses — only difference is `-f plan.json` plus the
`BUILDKIT_SYNTAX` build-arg pointing at Railpack's frontend image. It is
the same buildx push path, not a new workflow.

This is what `runLocalBuild()` in `cmd/cli/deploy.go` does for
`--build-tool=railpack`: the two steps are an internal implementation
detail; from the user's side it is still a single `conure deploy`.

### The Railpack frontend needs a plan — it does NOT self-prepare

Verified: pointing `docker buildx build --build-arg
BUILDKIT_SYNTAX=…railpack-frontend …` at raw source **with no `-f
plan.json`** fails — buildx hands the project's `Dockerfile` to the
frontend, which tries to parse it as a Railpack plan:

```
ERROR: failed to parse railpack plan: invalid character '#' …
```

So a `railpack prepare` step is **mandatory** before the frontend can
build. This is why remote railpack is not just "delete the guard".

### A railpack.json may be required for the image to RUN

`railpack prepare` only fails to find a **start command** for projects
outside its heuristics (FastAPI/Flask/Django/`main.py`). For anything
else — e.g. a console-script entrypoint, an MCP server — it builds an
image with no valid start command and the container won't start. Fix is a
`railpack.json` in the project root:

```json
{
  "$schema": "https://schema.railpack.com",
  "deploy": { "startCommand": "/app/.venv/bin/<entrypoint>" }
}
```

`deploy.startCommand` is the verified schema field (run `railpack schema`).
The venv lands at `/app/.venv` and `/app/.venv/bin` is on `$PATH` in the
Railpack runtime image, but using the absolute path matches the common
project-`Dockerfile` `CMD` convention and is robustly PATH-independent.
Transport/port/etc. are **per-deployment env vars** (conure component
variables), not baked into `railpack.json`.

### Image facts (don't guess these)

- `ghcr.io/railwayapp/railpack-frontend` — entrypoint `[/railpack frontend]`.
  It bundles the same `/railpack` binary **but is a scratch image with no
  userland**. `railpack prepare` inside it fails (it needs to download &
  exec `mise`). **Cannot be reused as a prepare/init image.**
- `ghcr.io/railwayapp/railpack-builder` / `-runtime` — base images for
  *built* apps (bash, no railpack binary). Not a CLI image.
- There is **no published standalone `railpack` CLI image** — only release
  tarballs (`railpack-vX-<arch>-{apple-darwin,unknown-linux-musl}.tar.gz`).
- `railpack prepare` downloads `mise` at runtime to
  `/tmp/railpack/mise/mise-<ver>` (~80 MB). **Alpine/musl fails** — mise
  ships glibc-only binaries (`exit status 127`). A **glibc base
  (`debian:bookworm-slim`) works**: install via `railpack.com/install.sh`,
  then `prepare` succeeds.

### Local vs. remote support

| | Local | Remote |
|---|---|---|
| Status | ✅ Supported (verified end-to-end) | ⛔ Guarded off (see below) |
| Mechanism | CLI: `railpack prepare` → `docker buildx build` (frontend) | — |

Remote railpack is currently **rejected** in two places:

- Server: `TriggerBuildRequest.validate()` in
  `cmd/api-server/applications/builds.go` returns `ErrInvalidRequest` for
  `build_tool=railpack` + `build_location=remote`.
- CLI: `cmd/cli/deploy.go` errors before the request is sent.

`renderBuildJob()` already branches on `BuildToolRailpack` and wires
`railpackFrontendArgs()` (`--frontend gateway.v0 --opt
source=ghcr.io/railwayapp/railpack-frontend:latest`) into the `buildctl`
invocation — the branch is dead today but kept for when the guard is
relaxed.

### Planned design for remote railpack (not yet implemented)

The remote Job today is `git-clone` (init) → `buildctl build` (main). The
railpack frontend needs a plan in the build context, which nothing
generates remotely. The planned change:

1. **Add a `railpack-prepare` init container** to `renderBuildJob()`,
   ordered **after `git-clone`, before `build`**, gated on
   `b.BuildTool == models.BuildToolRailpack` (dockerfile builds are
   unchanged — they keep the single git-clone init container). It runs
   `railpack prepare /workspace --plan-out /workspace/railpack-plan.json
   --info-out /workspace/railpack-info.json` on the shared `workspace`
   `emptyDir`, so the plan is present when `buildctl` runs.
2. **Image for that init container:** a **conure-built, published
   `railpack-cli` image** (`debian:bookworm-slim` + a pinned `railpack`
   binary baked in). Reasons it must be custom: no upstream CLI image
   exists; the frontend image can't run `prepare`; Alpine breaks mise;
   building it ourselves makes Job startup fast and the railpack version
   reproducible/pinned to match the local CLI. (Pre-seeding `mise` into
   the image for fully-offline prepare was investigated but is **not
   required** — see egress note.)
3. **Verify the `buildctl` plan-file flag.** The local path uses
   `docker buildx -f plan.json`; the `buildctl` equivalent for the gateway
   frontend to locate the plan from the `--local dockerfile=/workspace`
   mount (`--opt filename=railpack-plan.json` vs. default lookup) must be
   confirmed empirically before finalizing `buildScript` — do not assume.
4. **Remove both guards** (server `validate()` + CLI) and update the now
   stale comments / this doc's support table.
5. **Tests:** flip `TestTriggerBuildRequest_Validate`'s "remote railpack
   is rejected" case to expect acceptance; add a `renderBuildJob` test
   asserting **2 init containers** for railpack (git-clone +
   railpack-prepare) and **1** for dockerfile.

**Egress is NOT a new concern.** Remote dockerfile builds *already* require
build-network egress: `buildctl` pulls base images (`FROM python:…`),
fetches apt/pip/uv packages, and pushes to the registry. Remote railpack
adds only one more ghcr.io image pull (the frontend) plus the railpack/mise
fetch in prepare — and the latter is eliminated by baking railpack into our
own image. The environment that already runs remote docker builds already
satisfies remote railpack's needs. No NetworkPolicy work is required.

**Out of scope for the implementation:** live-cluster end-to-end run
(needs a cluster + registry creds) — verify Job *structure* via unit tests
and treat the e2e as a manual follow-up, same as the local fix.

## Multi-replica safety

The watcher state is durably owned by Mongo via a **lease** on each Build
document. Two fields are involved:

| Field              | Meaning                                                                 |
|--------------------|-------------------------------------------------------------------------|
| `watcherID`        | The replica currently watching the build. Empty when no one owns it.    |
| `watcherExpiresAt` | The deadline after which any replica may take over.                     |

Each replica generates a unique `WatcherID` at startup (`hostname-<random>`).
The lease is acquired with a single atomic Mongo `updateOne` whose filter
allows the claim only when the field is empty, already ours, or expired —
so concurrent replicas racing for the same build produce exactly one
winner.

Once acquired, the watcher heartbeats every 20 s (refreshing
`watcherExpiresAt` to now + 60 s). If a heartbeat fails to match (another
replica stole the lease), the goroutine exits cleanly.

### Adoption (recovering orphaned builds)

If a replica dies mid-build, the surviving replicas pick up its work via a
periodic scan:

- Every 30 s, each replica queries Mongo for builds with
  `status in {pending, building}`, `buildLocation == remote`,
  `jobName` set, and a missing/expired lease.
- For each match, the replica tries to acquire the lease. Exactly one
  replica wins per build (the others' updates match zero documents).
- The winner spawns a new watcher; its first poll reads the Job state from
  the cluster — which is unaffected by who watches it — and reacts
  appropriately (e.g. mark succeeded if the Job already finished while no
  one was watching).

Worst-case adoption latency is `leaseTTL + scanInterval = 60 s + 30 s = 90 s`.
The first scan also runs at process start, so a cold boot doesn't wait an
interval — pending builds from a prior process are picked up immediately.

### Tuning constants

Defined in `cmd/api-server/applications/builds.go`:

```go
buildLeaseTTL      = 60 * time.Second   // lease validity
buildHeartbeat     = 20 * time.Second   // heartbeat cadence (< TTL)
buildPoll          = 5  * time.Second   // job status poll cadence
buildScanInterval  = 30 * time.Second   // adoption scan cadence
buildJobTTLAfter   = 3600               // k8s Job TTLSecondsAfterFinished
buildAdoptionBatch = 50                 // max orphans claimed per scan tick
```

## API reference

### Triggers + reads

```
POST /organizations/:orgID/a/:appID/e/:env/c/:componentID/builds
GET  /organizations/:orgID/a/:appID/e/:env/c/:componentID/builds
GET  /organizations/:orgID/a/:appID/e/:env/c/:componentID/builds/:buildID
GET  /organizations/:orgID/a/:appID/e/:env/c/:componentID/builds/:buildID/logs
```

`POST` body (`TriggerBuildRequest`):

```json
{
  "build_tool":     "dockerfile",
  "build_location": "remote",
  "platform":       "linux/amd64",
  "git_repository": "https://github.com/me/app",
  "git_branch":     "main",
  "image_ref":      "ghcr.io/me/app:sha-abc"
}
```

Validation rules:

- `image_ref` is always required.
- `build_location` ∈ {`local`, `remote`}; `build_tool` ∈ {`dockerfile`, `railpack`}.
- `build_location=remote` requires `git_repository` and `git_branch`.
- `build_tool=railpack` is rejected when `build_location=remote`
  (current guard — see [Railpack](#railpack)).

Local responses return `201 Created` with the succeeded `Build`. Remote
responses return `202 Accepted` with the `building` `Build`.

### Build resource

```json
{
  "id":             "65f...",
  "component_id":   "65f...",
  "application_id": "65f...",
  "environment_id": "abcd1234",
  "status":         "succeeded",
  "build_tool":     "dockerfile",
  "build_location": "local",
  "platform":       "linux/amd64",
  "image_ref":      "ghcr.io/me/app:sha-abc",
  "git_repository": "",
  "git_branch":     "",
  "job_name":       "",
  "job_namespace":  "",
  "error_message":  "",
  "created_at":     "2026-05-14T12:00:00Z",
  "finished_at":    "2026-05-14T12:00:03Z"
}
```

### System info

```
GET /system/info
```

```json
{ "platform": "linux/amd64", "kubernetes_version": "v1.32.0" }
```

Platform is the most common `kubernetes.io/arch`/`kubernetes.io/os` pair
across all nodes; ties are broken lexicographically so the result is
deterministic.
