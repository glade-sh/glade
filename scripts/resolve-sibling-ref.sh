#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 ]]; then
	echo "usage: scripts/resolve-sibling-ref.sh <remote> <requested-ref> [fallback-ref]" >&2
	exit 2
fi

remote="$1"
requested_ref="$2"
fallback_ref="${3:-main}"
resolved_ref="${fallback_ref}"

if [[ -n "${requested_ref}" ]]; then
	if git ls-remote --exit-code --tags "${remote}" "refs/tags/${requested_ref}" >/dev/null 2>&1; then
		resolved_ref="${requested_ref}"
	elif git ls-remote --exit-code --heads "${remote}" "refs/heads/${requested_ref}" >/dev/null 2>&1; then
		resolved_ref="${requested_ref}"
	fi
fi

printf '%s\n' "${resolved_ref}"
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
	printf 'ref=%s\n' "${resolved_ref}" >>"${GITHUB_OUTPUT}"
fi
