#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

cd "${repo_root}"
evidence_path="${CI_CACHE_EVIDENCE_PATH:-ci-artifacts/cache-evidence.json}"
args=(--check)
if [[ -f "${evidence_path}" ]]; then
	args+=(--evidence-mode strict --cache-evidence "${evidence_path}")
else
	printf '[ci-cache] structural mode: no cache evidence supplied; cache consolidation remains unproven\n' >&2
fi
exec go run ./scripts/internal/cicache "${args[@]}"
