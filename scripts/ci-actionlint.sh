#!/usr/bin/env bash
set -euo pipefail

if ! command -v actionlint >/dev/null 2>&1; then
  echo 'actionlint is required; see docs/WORKFLOW_VALIDATION.md for the pinned version' >&2
  exit 127
fi

if [[ $# -eq 0 ]]; then
  repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
  shopt -s nullglob
  workflows=("$repo_root"/.github/workflows/*.yml "$repo_root"/.github/workflows/*.yaml)
  if [[ ${#workflows[@]} -eq 0 ]]; then
    echo 'no workflow files found in .github/workflows' >&2
    exit 1
  fi
  set -- "${workflows[@]}"
fi

# Keep workflow validity independent of optional shell/Python lint installations.
exec actionlint -shellcheck= -pyflakes= "$@"
