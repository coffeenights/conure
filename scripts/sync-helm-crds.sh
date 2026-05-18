#!/usr/bin/env bash
#
# Sync the Helm-bundled CRDs from the controller-gen source of truth.
#
# The chart ships a copy of every CRD under
#   deploy/helm/conure/templates/crds/
# wrapped in a `{{- if .Values.crds.install }}` guard so chart users can opt
# out of CRD management. controller-gen writes the canonical CRDs to
#   config/crd/bases/
# These two trees drift whenever an API type changes and only `make
# manifests` is run. This script regenerates the chart copies from the bases
# so they can never silently fall behind.
#
# Modes:
#   (default)   rewrite the chart CRDs from config/crd/bases
#   --check     do not write; exit 1 if any chart CRD is out of sync
#               (used by CI and as a helm-push pre-flight)
#
set -euo pipefail

BASES_DIR="${BASES_DIR:-config/crd/bases}"
CHART_CRD_DIR="${CHART_CRD_DIR:-deploy/helm/conure/templates/crds}"

GUARD_OPEN='{{- if .Values.crds.install }}'
GUARD_CLOSE='{{- end }}'

CHECK_ONLY=0
[ "${1:-}" = "--check" ] && CHECK_ONLY=1

err() { printf 'sync-helm-crds: %s\n' "$1" >&2; exit 1; }

[ -d "$BASES_DIR" ]     || err "no source CRD dir at $BASES_DIR (run 'make manifests' first)"
[ -d "$CHART_CRD_DIR" ] || err "no chart CRD dir at $CHART_CRD_DIR"

shopt -s nullglob
bases=("$BASES_DIR"/core.conure.io_*.yaml)
[ ${#bases[@]} -gt 0 ] || err "no CRDs matched in $BASES_DIR"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

drift=0
for src in "${bases[@]}"; do
  base="$(basename "$src")"
  dst="$CHART_CRD_DIR/$base"

  # Build the expected wrapped chart copy.
  wrapped="$tmp/$base"
  {
    printf '%s\n' "$GUARD_OPEN"
    cat "$src"
    printf '%s\n' "$GUARD_CLOSE"
  } > "$wrapped"

  if [ -f "$dst" ] && cmp -s "$wrapped" "$dst"; then
    continue
  fi

  drift=1
  if [ "$CHECK_ONLY" = 1 ]; then
    printf 'sync-helm-crds: OUT OF SYNC  %s\n' "$dst" >&2
  else
    cp "$wrapped" "$dst"
    printf 'sync-helm-crds: synced       %s\n' "$dst"
  fi
done

if [ "$CHECK_ONLY" = 1 ]; then
  if [ "$drift" = 1 ]; then
    err "chart CRDs are stale — run 'make helm-crds' and commit the result"
  fi
  printf 'sync-helm-crds: chart CRDs in sync\n'
  exit 0
fi

if [ "$drift" = 0 ]; then
  printf 'sync-helm-crds: chart CRDs already in sync\n'
fi
