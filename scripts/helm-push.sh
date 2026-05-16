#!/usr/bin/env bash
#
# Package the conure Helm chart and push it to the GHCR OCI registry.
#
# Target ref:  oci://ghcr.io/coffeenights/charts/conure:<Chart.yaml version>
#
# Auth: by default this reuses whatever registry credentials are already on
# the machine (e.g. from a prior `docker login ghcr.io` / `helm registry
# login` — Helm reads ~/.docker/config.json). No env vars needed locally.
#
# For CI or a clean machine with no stored login, set BOTH:
#   GHCR_USERNAME   GitHub username that owns the PAT
#   GHCR_TOKEN      Classic PAT with write:packages (ghcr.io does NOT accept
#                   fine-grained PATs; username must match the PAT owner)
# and the script will perform an explicit login (and log out on exit).
#
# Optional env:
#   HELM_OCI_REPO   Override the OCI repo base (default below)
#   FORCE           Set to 1 to overwrite an already-published chart version
#
set -euo pipefail

CHART_DIR="${CHART_DIR:-deploy/helm/conure}"
HELM_OCI_REPO="${HELM_OCI_REPO:-oci://ghcr.io/coffeenights/charts}"

# Registry host parsed from the oci:// URL, used for `helm registry login`.
REGISTRY_HOST="$(printf '%s\n' "$HELM_OCI_REPO" | sed -E 's#^oci://##; s#/.*$##')"

err() { printf 'helm-push: %s\n' "$1" >&2; exit 1; }

command -v helm >/dev/null 2>&1 || err "helm not found on PATH"

[ -f "$CHART_DIR/Chart.yaml" ] || err "no Chart.yaml at $CHART_DIR"

CHART_NAME="$(awk '/^name:/ {print $2; exit}' "$CHART_DIR/Chart.yaml")"
CHART_VERSION="$(awk '/^version:/ {print $2; exit}' "$CHART_DIR/Chart.yaml")"
[ -n "$CHART_NAME" ]    || err "could not parse chart name from Chart.yaml"
[ -n "$CHART_VERSION" ] || err "could not parse chart version from Chart.yaml"

CHART_REF="${HELM_OCI_REPO}/${CHART_NAME}:${CHART_VERSION}"

WORKDIR="$(mktemp -d)"
DID_LOGIN=0
cleanup() {
  rm -rf "$WORKDIR"
  # Only drop the credential if *we* created it; don't clobber a login the
  # user set up themselves.
  [ "$DID_LOGIN" = 1 ] && helm registry logout "$REGISTRY_HOST" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Explicit login only when credentials are supplied (CI / clean machine).
# Otherwise reuse the machine's existing docker/helm credentials. Requiring
# both guards against a half-set pair silently falling back to no auth.
if [ -n "${GHCR_USERNAME:-}" ] || [ -n "${GHCR_TOKEN:-}" ]; then
  [ -n "${GHCR_USERNAME:-}" ] || err "GHCR_TOKEN set but GHCR_USERNAME is empty (ghcr.io needs a username matching the PAT owner)"
  [ -n "${GHCR_TOKEN:-}" ]    || err "GHCR_USERNAME set but GHCR_TOKEN is empty"
  printf 'helm-push: logging in to %s as %s\n' "$REGISTRY_HOST" "$GHCR_USERNAME"
  printf '%s' "$GHCR_TOKEN" | helm registry login "$REGISTRY_HOST" \
    --username "$GHCR_USERNAME" --password-stdin
  DID_LOGIN=1
else
  printf 'helm-push: using existing registry credentials for %s\n' "$REGISTRY_HOST"
fi

# Clobber guard: refuse to overwrite a version that already exists unless
# FORCE=1. Runs *after* login so the existence check is reliable even for a
# private chart (an unauthenticated `helm show` would 401 and look absent).
if [ "${FORCE:-0}" != "1" ]; then
  if helm show chart "${HELM_OCI_REPO}/${CHART_NAME}" --version "$CHART_VERSION" >/dev/null 2>&1; then
    err "chart ${CHART_NAME}:${CHART_VERSION} already exists at ${HELM_OCI_REPO} — bump Chart.yaml version, or re-run with FORCE=1 to overwrite"
  fi
fi

helm package "$CHART_DIR" --destination "$WORKDIR"
PKG="$WORKDIR/${CHART_NAME}-${CHART_VERSION}.tgz"
[ -f "$PKG" ] || err "expected package $PKG not produced by helm package"

printf 'helm-push: pushing %s -> %s\n' "$PKG" "$CHART_REF"
helm push "$PKG" "$HELM_OCI_REPO"

printf 'helm-push: done — %s\n' "$CHART_REF"
