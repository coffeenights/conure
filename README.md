# Conure

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/coffeenights/conure)](https://goreportcard.com/report/github.com/coffeenights/conure)
[![Go Version](https://img.shields.io/github/go-mod/go-version/coffeenights/conure)](go.mod)

Conure is a Kubernetes-native application platform that lets developers ship
applications to Kubernetes without writing or maintaining raw manifests. You
describe an application as a small set of typed Components — services, jobs,
data stores — and Conure renders, deploys, and reconciles the underlying
Kubernetes resources for you. It bundles an API server, a CLI, and a controller
so the same model can be driven from a terminal, an automation pipeline, or
directly through the API.

## Features

- **Component-based application model.** Applications are composed of typed
  components rendered through [Timoni](https://timoni.sh) modules, keeping the
  developer-facing surface small while still producing idiomatic Kubernetes
  resources.
- **Custom resources.** `Application` and `Component` CRDs are reconciled by a
  controller-runtime based controller, with drift detection and explicit
  `Rendered` / `Deployed` / `Ready` status conditions.
- **Environments and revisions.** Per-environment variable scopes, revision
  history with comments, and a restart workflow that propagates through pod
  templates.
- **First-class CLI.** Manage organizations, applications, components,
  environments, variables, deploys, logs, and revisions from `conure` on the
  command line.
- **REST API.** A Gin-based HTTP API with JWT authentication backs the CLI and
  can be used directly for integrations.
- **Pluggable secrets.** Secret material can be stored locally for development
  or backed by Kubernetes Secrets in cluster.

## Architecture

Conure ships as three cooperating processes:

- **API server** (`cmd/api-server`) — REST API for organizations, applications,
  components, environments, variables, and revisions. Persists state in MongoDB
  and issues JWTs for the CLI and other clients.
- **Controller** (`cmd/control`) — Kubernetes controller built with
  controller-runtime. Watches `Application` and `Component` custom resources,
  renders them through Timoni, applies the result, and reports status back
  through typed conditions.
- **CLI** (`cmd/cli`) — `conure` binary that talks to the API server. Supports
  multiple profiles, per-directory links to a component, and `text`, `json`, or
  `yaml` output for scripting.

The custom resource definitions live under `apis/core/v1alpha1` and are
installed into the cluster like any other CRD.

## Getting started

### Prerequisites

- Go 1.26 or newer
- Docker and Docker Compose
- A Kubernetes cluster (a local one such as Kind or k3d is fine)
- `kubectl` configured against that cluster

### Build

```shell
# Build the controller
make build

# Build the CLI
make build-cli
```

The resulting binaries land in `bin/manager` and `bin/conure`.

### Run the development stack

1. Copy the example environment file and adjust it for your setup:

   ```shell
   cp config.env .env
   ```

   At a minimum, set `DB_URL` to point at a reachable MongoDB instance. The
   `docker-compose.yml` file in the repo provides one out of the box.

2. Start MongoDB:

   ```shell
   docker-compose up -d mongo
   ```

3. Start the API server:

   ```shell
   go run ./cmd/api-server/main.go runserver
   ```

   The server listens on `localhost:8080` by default.

4. Create a superuser so you can sign in from the CLI:

   ```shell
   go run ./cmd/api-server/main.go createsuperuser -email=admin@example.com
   ```

5. Install the CRDs and run the controller against your cluster:

   ```shell
   make install
   make run
   ```

### Use the CLI

Log in, point the CLI at your local API server, and explore:

```shell
conure login --server http://localhost:8080
conure org list
conure app list
conure component status
```

`conure --help` lists every available command.

## Deploying to a cluster

To run Conure inside Kubernetes rather than locally:

```shell
# Build and push the controller image
make docker-build docker-push IMG=<your-registry>/conure-controller:tag

# Install the CRDs and deploy the controller
make install
make deploy IMG=<your-registry>/conure-controller:tag
```

The `deploy/` directory contains a Helm chart that can be used to install the
API server, controller, and supporting resources together.

## Testing

The test suite combines plain Go unit tests, fake-client handler tests, and
envtest-based controller integration tests:

```shell
make test
```

`make test` downloads the matching `kubebuilder` envtest assets the first time
it runs and writes coverage output to `cover.out`.

## Contributing

Contributions are welcome. If you plan to work on something larger than a small
fix, please open an issue first so we can discuss scope and direction. When
sending a pull request:

1. Fork the repository and create your branch from `main`.
2. Run `make fmt`, `make vet`, and `make test` locally.
3. Keep commits focused and write descriptive commit messages.
4. Open the pull request against `main` and link any related issues.

By contributing, you agree that your contributions will be licensed under the
project's AGPL-3.0 license.

## License

Conure is distributed under the terms of the GNU Affero General Public License,
version 3. See [LICENSE](LICENSE) for the full text.
