#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"

cleanup() {
  rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"

CGO_ENABLED=1 go build -o "${TMP}/glade" ./cmd/glade
GLADE="${TMP}/glade"

scripts/smoke-runtime.sh "${GLADE}"

DIST_DIR="${TMP}/release-dist" VERSION=smoke scripts/release-build.sh >"${TMP}/release-build.out"
grep -q 'release artifact written' "${TMP}/release-build.out"
grep -q 'release manifest written' "${TMP}/release-build.out"
grep -q 'glade_smoke_' "${TMP}/release-build.out"
test -f "${TMP}/release-dist/release-manifest.json"
test -f "${TMP}/release-dist/index.json"
test -f "${TMP}/release-dist/latest/release-manifest.json"
grep -q '"schemaVersion": 2' "${TMP}/release-dist/release-manifest.json"
grep -q '"assets"' "${TMP}/release-dist/release-manifest.json"
grep -q '"sha256"' "${TMP}/release-dist/release-manifest.json"
grep -q '"parserSmoke": "passed"' "${TMP}/release-dist/release-manifest.json"

echo "smoke: ok"
