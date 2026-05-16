# Component definitions

ComponentDefinitions are no longer applied to the cluster as standalone
manifests, and they are no longer cluster-wide.

They are now **org-scoped with MongoDB as the source of truth**:

- The platform's shipped defaults live in the Helm chart at
  [`deploy/helm/conure/files/component-definitions/`](../../deploy/helm/conure/files/component-definitions/).
  A post-install/post-upgrade Job upserts them into MongoDB under the
  default-owner sentinel; every organization inherits them until it
  overrides or hides one via the API
  (`/:organizationID/component-definitions`).
- At deploy time the API resolves the org's effective definition and
  materializes it into the target cluster as a cluster-scoped
  `ComponentDefinition` CRD labelled with the org id. The controller
  resolves it by `(type, engine)` scoped to that label and stays
  org-unaware.

To add or change a default, edit the chart `files/component-definitions/`
directory (or set `apiServer.componentDefinitions.extra` in values) and run
`helm upgrade` — the seed is idempotent on `(type, engine)`.

To seed manually (e.g. local dev without Helm):

```bash
go run ./cmd/api-server/main.go seeddefaultcomponentdefinitions \
  -dir=deploy/helm/conure/files/component-definitions
```
