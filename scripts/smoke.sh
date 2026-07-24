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
scripts/smoke-distribution.sh

echo "smoke: ok"
