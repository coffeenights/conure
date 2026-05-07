# Private OCI Registry Authentication

Conure renders `Component` resources by pulling Timoni module artifacts from an
OCI registry. Public registries work without configuration. Private registries
require a `kubernetes.io/dockerconfigjson` Secret referenced from the
`ComponentDefinition`.

## How it works

1. The platform administrator creates a `dockerconfigjson` Secret in the
   controller's namespace (e.g. `conure-system`).
2. The `ComponentDefinition` references that Secret via
   `spec.registrySecretRef`.
3. On reconcile, the controller reads the Secret, picks the `.auths` entry
   matching the registry host of `spec.ociRepository`, and passes
   `username:password` to the OCI fetcher.

The Secret lives in the controller's namespace because `ComponentDefinition` is
cluster-scoped and platform-curated. Tenants don't manage pull credentials.

## Setup

### 1. Create the Secret

```bash
kubectl create secret docker-registry ghcr-creds \
  --docker-server=ghcr.io \
  --docker-username=<username> \
  --docker-password=<token> \
  --namespace conure-system
```

A single Secret can hold credentials for multiple registries — repeat
`--docker-server`/`--docker-username`/`--docker-password` per registry, or edit
the resulting Secret's `.dockerconfigjson` payload directly. The controller
selects the entry whose host matches the `ociRepository`.

> **GHCR note:** `ghcr.io` requires a **classic** Personal Access Token with
> the `read:packages` scope. Fine-grained PATs are not accepted.

### 2. Reference the Secret from the ComponentDefinition

```yaml
apiVersion: core.conure.io/v1alpha1
kind: ComponentDefinition
metadata:
  name: fastapi-demo
spec:
  type: fastapi-demo
  description: FastAPI demo module
  ociRepository: ghcr.io/coffeenights/modules/fastapi-demo
  ociTag: 0.3.0
  ociDigest: sha256:...
  registrySecretRef:
    name: ghcr-creds
```

Or patch an existing definition:

```bash
kubectl patch componentdefinition fastapi-demo --type=merge \
  -p '{"spec":{"registrySecretRef":{"name":"ghcr-creds"}}}'
```

Public modules continue to work — omit `registrySecretRef` and the controller
makes an unauthenticated pull.

## Configuration

The controller resolves the Secret namespace from, in order:

1. `--registry-secret-namespace` flag.
2. `POD_NAMESPACE` environment variable (set automatically via the downward
   API in the shipped manifests).

For local development (`make run`), pass the flag explicitly:

```bash
go run ./cmd/control --registry-secret-namespace=conure-system
```

## Module cache

Module artifacts are cached at `CONURE_TIMONI_CACHE_DIR` (default
`/var/cache/timoni` in-cluster, `os.TempDir()/conure-timoni-cache` locally).
The cache is backed by an `emptyDir` volume — ephemeral, recreated on every
pod reschedule.

## Troubleshooting

| Symptom in `Ready` condition message | Cause |
|---|---|
| `failed to get registry secret <ns>/<name>: ... not found` | Secret doesn't exist in the controller namespace, or `registrySecretRef.name` is wrong. |
| `must be of type kubernetes.io/dockerconfigjson` | Secret was created as `Opaque` — recreate with `kubectl create secret docker-registry ...`. |
| `has no credentials for registry "<host>"` | The `.auths` map in the Secret has no entry whose hostname matches the `ociRepository`. Add an entry for that registry. |
| `auth field for ... is not in user:password form` | Manually edited `.dockerconfigjson` with a malformed `auth` value. Recreate the Secret. |
| `registry secret namespace not configured` | Controller running without `POD_NAMESPACE` env or `--registry-secret-namespace` flag. |

For pull failures after credential resolution succeeds (e.g. 401/403 from the
registry), check the controller logs — the underlying error from the OCI
client is propagated through the `failed to get apply sets` log line.