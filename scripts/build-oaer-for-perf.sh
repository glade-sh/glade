#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="${OAER_PERF_BIN:-$repo_root/bin/oaer-perf}"

mkdir -p "$(dirname "$out")"

args=(-trimpath -o "$out")
if [[ -n "${PGO_PROFILE:-}" ]]; then
  args=(-trimpath -pgo="$PGO_PROFILE" -o "$out")
fi

go build "${args[@]}" "$repo_root/cmd/oaer"
printf '%s\n' "$out"
