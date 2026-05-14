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
| `remote`  | A BuildKit Job in `conure-system` | `conure deploy --image-ref ...` | Dockerfile only      |

The CLI defaults to `--build-location auto`, which picks the right mode for
the current machine and the target cluster — see *Architecture detection*
below.

## CLI usage

```bash
# Backwards-compatible: promote the latest draft (no image build)
conure deploy

# Build locally and deploy. Auto-picks the right target architecture.
conure deploy --image-ref ghcr.io/me/app:sha-abc

# Force a remote build (server clones the repo and runs a BuildKit Job)
conure deploy --image-ref ghcr.io/me/app:sha-abc --build-location remote \
              --git-repository https://github.com/me/app --git-branch main
```

### Flags

| Flag                | Default | Notes                                                                                  |
|---------------------|---------|----------------------------------------------------------------------------------------|
| `--image-ref`       | —       | Target image (`registry/repo:tag`). Triggers a build when set.                         |
| `--build-location`  | `auto`  | `local`, `remote`, or `auto`. See decision table below.                                |
| `--build-tool`      | auto    | `dockerfile` when a `Dockerfile` exists in the context, `railpack` otherwise.          |
| `--platform`        | from API | Defaults to the cluster's dominant node platform (via `GET /system/info`).            |
| `--git-repository`  | —       | Required for remote builds.                                                            |
| `--git-branch`      | `main`  | Used for remote builds.                                                                |
| `--context`         | `.`     | Local build context directory.                                                         |

## Architecture detection

A developer on an Apple Silicon Mac (`darwin/arm64`) deploying to an
`amd64` cluster must not push an `arm64` image — the pod won't start.
Conure handles this for you:

1. The CLI calls `GET /system/info`, which returns the cluster's dominant
   node platform (e.g. `linux/amd64`) computed from `kubernetes.io/arch`
   labels.
2. `--platform` defaults to that value. `docker buildx build --platform
   linux/amd64 --push` or `railpack build --platform linux/amd64` produces
   the right image regardless of the host architecture.
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
- `railpack build <context> --push --tag <image-ref> --platform <platform>`
  when `--build-tool=railpack`. Requires the
  [Railpack CLI](https://railpack.com) on `PATH`.

After the build pushes successfully, the CLI calls
`POST .../builds` with `build_location: local` and the image ref. The API
records the build as `succeeded`, writes a new deployed revision with
`source.ociRepository` and `source.tag` overridden, and applies the
Component CRD synchronously.

> **Railpack is local-only.** Remote builds always use the BuildKit
> Dockerfile frontend. The API rejects `build_tool=railpack` for
> `build_location=remote`.

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
- `build_tool=railpack` is rejected when `build_location=remote`.

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
