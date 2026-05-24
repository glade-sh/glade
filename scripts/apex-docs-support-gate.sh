#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${GLADE_APEX_DOCS_SOURCE:-}"

if [[ -z "${SOURCE}" ]]; then
  echo "apex-docs-support: skipped (set GLADE_APEX_DOCS_SOURCE to the scraped Apex docs directory)"
  exit 0
fi

if [[ ! -d "${SOURCE}" ]]; then
  echo "apex-docs-support: source directory not found: ${SOURCE}" >&2
  exit 1
fi

TMP="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP}"
}
trap cleanup EXIT

cd "${ROOT}"

GLADE="${GLADE_BIN:-}"
if [[ -z "${GLADE}" ]]; then
  go build -o "${TMP}/glade" ./cmd/glade
  GLADE="${TMP}/glade"
fi

INVENTORY="${TMP}/apex-docs-inventory.json"
CATALOG="${TMP}/apex-capability-catalog.json"
PRODUCT_NAMESPACES="${TMP}/apex-product-namespaces.json"
EVIDENCE="${TMP}/apex-evidence.txt"

"${GLADE}" compat docs-inventory --source "${SOURCE}" --output "${INVENTORY}"
"${GLADE}" compat docs-inventory --source "${SOURCE}" --check "${INVENTORY}"

"${GLADE}" compat catalog --inventory "${INVENTORY}" --output "${CATALOG}"
"${GLADE}" compat catalog --inventory "${INVENTORY}" --check "${CATALOG}"

"${GLADE}" compat product-namespaces --catalog "${CATALOG}" --output "${PRODUCT_NAMESPACES}"
"${GLADE}" compat product-namespaces --catalog "${CATALOG}" --check "${PRODUCT_NAMESPACES}"

"${GLADE}" compat evidence --catalog "${CATALOG}" docs/fixtures/*.json >"${EVIDENCE}"
grep -q 'unmatchedEvidence: 0' "${EVIDENCE}" || {
  echo "apex-docs-support: fixture evidence references symbols missing from the catalog" >&2
  cat "${EVIDENCE}" >&2
  exit 1
}

echo "apex-docs-support: ok"
