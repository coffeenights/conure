# Container Registries

Conure needs a container registry to store the images it builds and the
cluster pulls. This page helps you pick one and set it up.

> **Scope.** This document is about **workload images** — the artifacts
> `conure deploy` builds and pushes (e.g. `ghcr.io/me/app:sha-abc`). For
> **Timoni module artifacts** used by `ComponentDefinition`s, see
> [private-oci-registries.md](private-oci-registries.md). Two distinct
> concerns; both can target the same registry, but the auth is configured
> separately.

## Why you need one

When Conure deploys a Component, the workload pod's `image` field points
at a registry URL. The Kubernetes node scheduling that pod has to pull
the image from there. A locally-built image that never left your laptop
is unreachable by the cluster — the pod will stall in `ImagePullBackOff`.
So whichever registry you choose, it has to be **network-reachable from
the cluster's nodes**.

There are two distinct credentials involved:

| Credential | Used by | Where it lives |
|---|---|---|
| **Push** | The build (CLI locally, or BuildKit pod for remote builds) | Local `~/.docker/config.json`; or `registry-credentials` Secret in `conure-system` for remote builds. |
| **Pull** | The kubelet on the node running the workload pod | `imagePullSecrets` on the pod (wired via the Component / ServiceAccount). See [private-oci-registries.md](private-oci-registries.md) for the existing pattern. |

For a **public** repository on a public registry, pull creds aren't
needed. Push creds always are.

## Picking a registry

The decision boils down to four questions:

1. **Are you on a managed cloud (AWS / GCP / Azure)?**
   → Use the cloud's native registry. It's the path of least resistance,
   has IRSA/Workload Identity integration, and avoids egress costs.
   ECR, GAR, ACR.

2. **Do you already host code on GitHub / GitLab / a self-hosted Forgejo
   or Gitea instance?**
   → Use that platform's bundled container registry. Auth piggybacks on
   the same PAT you already use for git. GHCR, GitLab CR, Forgejo,
   Gitea.

3. **Do you need a registry inside your own network with no third-party
   account, and you have someone willing to operate it?**
   → Self-host Harbor (or `distribution/distribution` for something
   minimal).

4. **None of the above and you just want to deploy something today.**
   → Docker Hub or GHCR. Both have free public tiers; GHCR is preferred
   if you're already on GitHub.

### Quick comparison

| Registry | Free tier | Private repos | Cloud IAM | Notes |
|---|---|---|---|---|
| **GHCR** (`ghcr.io`) | Yes | Yes | — | Auth via GitHub PAT (classic, `write:packages`). Best fit if you're already on GitHub. |
| **Docker Hub** (`docker.io`) | Limited | 1 private repo free | — | Pull rate limits without a paid account; can bite shared CI. |
| **AWS ECR** | Pay-per-GB | Yes | IRSA / OIDC | Best for EKS. Pull creds via IRSA — no static tokens to manage. |
| **Google GAR** | Pay-per-GB | Yes | Workload Identity | Best for GKE. Same model as ECR. |
| **Azure ACR** | Pay-per-GB | Yes | Workload Identity | Best for AKS. |
| **GitLab Container Registry** | With GitLab account | Yes | — | Built into every GitLab project (cloud + self-hosted). |
| **Forgejo / Gitea** | Self-hosted | Yes | — | Container registry built into the forge. Same instance you use for git. |
| **Harbor** | Self-hosted | Yes | — | Full-featured: replication, vuln scanning, RBAC, project quotas. Operational overhead. |
| **`distribution/distribution`** | Self-hosted | Yes | — | Bare-bones reference implementation. No UI, no scanning. |

## Setup recipes

Each section gives you the two things you need:

- the **push secret** to drop in `conure-system` so remote builds can push;
- the **`--image-ref`** format to use with `conure deploy`.

For local builds, replace the push-secret step with `docker login <registry>`
on your laptop.

### GitHub Container Registry (GHCR)

1. Create a **classic** PAT (fine-grained tokens are not supported) at
   <https://github.com/settings/tokens> with `write:packages` and
   `read:packages` scopes.
2. Create the push secret:

   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=ghcr.io \
     --docker-username=<github-username> \
     --docker-password=ghp_xxx \
     --namespace conure-system
   ```

3. Image ref format:

   ```
   ghcr.io/<owner>/<repo>:<tag>
   ```

   Example: `conure deploy --image-ref ghcr.io/coffeenights/api:sha-abc`.

> The `--docker-username` must match the PAT owner; GHCR rejects logins
> where they don't.

### Docker Hub

1. Create an access token at <https://hub.docker.com/settings/security>
   (preferred over a password).
2. Create the push secret:

   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=https://index.docker.io/v1/ \
     --docker-username=<dockerhub-username> \
     --docker-password=<access-token> \
     --namespace conure-system
   ```

3. Image ref format:

   ```
   docker.io/<username>/<repo>:<tag>
   ```

   Or omit the registry for the implicit default: `<username>/<repo>:<tag>`.

> **Pull rate limits.** Anonymous and free-tier pulls are rate-limited.
> If you're pulling images on every node deploy across a fleet, configure
> a pull-side secret too or upgrade the account.

### AWS ECR

ECR has two paths: long-lived credentials (simple, periodic refresh
needed) or IAM Roles for Service Accounts (IRSA, recommended).

**Long-lived credentials** (works anywhere, but requires periodic refresh
because ECR auth tokens expire after 12 hours):

```bash
aws ecr get-login-password --region us-east-1 | \
  kubectl create secret docker-registry registry-credentials \
    --docker-server=<account>.dkr.ecr.us-east-1.amazonaws.com \
    --docker-username=AWS \
    --docker-password-stdin \
    --namespace conure-system
```

You'll need to refresh this every 12 hours via a CronJob or external
operator (e.g. `ecr-credentials-sync`).

**IRSA** (preferred on EKS):

1. Annotate the API server's ServiceAccount with an IAM role that has
   `ecr:GetAuthorizationToken` + `ecr:BatchCheckLayerAvailability` +
   `ecr:PutImage` etc.
2. Use the AWS ECR credential helper inside the BuildKit container (out
   of scope of the bundled remote-build Job today — supporting this is a
   future enhancement).

Image ref format:

```
<account>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>
```

The `<repo>` must already exist — ECR doesn't auto-create repos on push.
Use `aws ecr create-repository --repository-name <repo>` first.

### Google Artifact Registry (GAR)

1. Create a repository:
   ```bash
   gcloud artifacts repositories create conure \
     --repository-format=docker \
     --location=us-central1
   ```
2. **Workload Identity** (preferred on GKE):
   Bind the API server's ServiceAccount to a GCP SA with the
   `roles/artifactregistry.writer` role on the repository. No Secret
   needed.

3. **Service account JSON key** (works anywhere):
   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=us-central1-docker.pkg.dev \
     --docker-username=_json_key \
     --docker-password="$(cat sa-key.json)" \
     --namespace conure-system
   ```

Image ref format:

```
<region>-docker.pkg.dev/<project>/conure/<repo>:<tag>
```

### Azure Container Registry (ACR)

1. Create the registry:
   ```bash
   az acr create --resource-group <rg> --name <acr-name> --sku Basic
   ```
2. **Workload Identity / Managed Identity** (preferred on AKS): attach
   `AcrPush`/`AcrPull` roles to the API server's identity, skip the
   Secret.
3. **Admin credentials** (simple but coarse):
   ```bash
   az acr update --name <acr-name> --admin-enabled true
   ACR_PASSWORD=$(az acr credential show --name <acr-name> --query 'passwords[0].value' -o tsv)
   kubectl create secret docker-registry registry-credentials \
     --docker-server=<acr-name>.azurecr.io \
     --docker-username=<acr-name> \
     --docker-password="$ACR_PASSWORD" \
     --namespace conure-system
   ```

Image ref format:

```
<acr-name>.azurecr.io/<repo>:<tag>
```

### GitLab Container Registry

Works on GitLab.com or any self-hosted GitLab where the container
registry is enabled.

1. Create a personal/project/deploy token at
   `Settings > Repository > Project access tokens` with `write_registry`
   scope.
2. Create the push secret:
   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=registry.gitlab.com \
     --docker-username=<token-name> \
     --docker-password=<token> \
     --namespace conure-system
   ```

Image ref format:

```
registry.gitlab.com/<group>/<project>:<tag>
registry.gitlab.com/<group>/<project>/<path>:<tag>
```

The path mirrors the GitLab project structure — every project has its
own registry namespace.

### Forgejo / Gitea container registry

Forgejo (and its upstream Gitea) ship a built-in container registry.
Useful when you're self-hosting both your code and your registry on the
same forge.

1. Enable the package registry in `app.ini`:
   ```ini
   [packages]
   ENABLED = true
   ```
2. Create an access token in `Settings > Applications` with the
   `write:package` scope.
3. Create the push secret:
   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=forgejo.example.com \
     --docker-username=<your-forgejo-user> \
     --docker-password=<token> \
     --namespace conure-system
   ```

Image ref format:

```
forgejo.example.com/<owner>/<image>:<tag>
```

The `<owner>` is either the user or organization that owns the package.
TLS is required — `conure deploy` won't push to a plain-HTTP registry
unless your nodes' container runtime is explicitly configured to trust
it (not recommended).

Gitea uses the same format; substitute the hostname.

### Harbor

Operationally heavier but feature-rich: project-level RBAC, image
scanning (Trivy), replication between Harbor instances, retention
policies. Worth it when you have multiple teams or compliance requirements.

1. Install Harbor (Helm chart at <https://goharbor.io/docs/2.x/install-config/>).
2. Create a robot account at the project level with `push` permission.
3. Create the push secret:
   ```bash
   kubectl create secret docker-registry registry-credentials \
     --docker-server=harbor.example.com \
     --docker-username=robot$<project>+<robot-name> \
     --docker-password=<robot-token> \
     --namespace conure-system
   ```

Image ref format:

```
harbor.example.com/<project>/<repo>:<tag>
```

The `<project>` must exist in Harbor before the first push; Harbor
doesn't auto-create projects.

### `distribution/distribution` (the open-source reference registry)

Minimal — no UI, no auth out of the box, no scanning. Suitable for
single-tenant dev clusters or as the foundation behind a separately-
operated auth proxy.

Recipe is out of scope here because the operational details (storage
backend, TLS termination, auth) are too varied to cover briefly. If you
want to self-host without Harbor, start from
<https://distribution.github.io/distribution/about/deploying/>.

Image ref format (whatever hostname you expose):

```
registry.example.com/<repo>:<tag>
```

## Multiple registries in one cluster

The `registry-credentials` Secret can hold credentials for any number of
registries — `.dockerconfigjson` is a map keyed by hostname. Run
`kubectl create secret docker-registry` once per registry, then merge the
resulting `.auths` entries by hand, or use the existing Secret as a
starting point and `kubectl edit` the JSON:

```json
{
  "auths": {
    "ghcr.io":               { "auth": "<base64 user:token>" },
    "docker.io":             { "auth": "<base64 user:token>" },
    "harbor.example.com":    { "auth": "<base64 user:token>" }
  }
}
```

BuildKit selects the right entry by matching the hostname in
`--image-ref`. The Conure controller does the same when resolving pull
credentials for Timoni modules.

## Choosing where to store the image: a checklist

| Question | If yes → consider |
|---|---|
| Is your code on GitHub? | GHCR |
| Is your cluster on EKS? | ECR (+ IRSA) |
| Is your cluster on GKE? | GAR (+ Workload Identity) |
| Is your cluster on AKS? | ACR (+ Workload Identity) |
| Are you already running Forgejo or Gitea? | Use its built-in registry |
| Are you already running GitLab? | Use its built-in registry |
| Multi-team / compliance / scanning required? | Harbor (self-hosted) |
| Hobby cluster / first deploy / no preference? | Docker Hub or GHCR |
| All-air-gapped, no third-party services? | Harbor or `distribution` self-hosted |

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `conure deploy` fails: `unauthorized: authentication required` | Push secret missing or wrong; for local builds, run `docker login <registry>` first. |
| Pod stuck in `ImagePullBackOff` after a successful build | The cluster can reach the registry but the workload's `imagePullSecrets` doesn't include the registry's pull credentials. See [private-oci-registries.md](private-oci-registries.md). |
| `denied: requested access to the resource is denied` | The credentials work but the user/token doesn't have permission to push to that specific repo or project. Check the token's scopes and the registry's RBAC. |
| `manifest unknown` on pull | The image-ref's tag was overwritten or the registry's retention policy GC'd the manifest. Re-push or pin to a digest. |
| Pull works the first time, fails later with `429 Too Many Requests` | Docker Hub rate limits. Either authenticate pulls or move off Docker Hub. |
| `tls: x509: certificate signed by unknown authority` | The registry's TLS cert is signed by a CA the cluster nodes don't trust. Use a public CA (Let's Encrypt) or add the root to every node's trust store. |
