#!/usr/bin/env bash
# Write the JSON list of Go packages a pull request must test: the changed
# packages and their direct importers. Run from the repository root on the
# merge commit. The PR test shards and the shared test-binary producers call
# this one script so they always agree on the selection.
set -euo pipefail
out=${1:?usage: scripts/ci-pr-select.sh OUTPUT_JSON}
tmp=$(mktemp -d "${RUNNER_TEMP:-/tmp}/ci-pr-select.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
go list -json ./... | jq -s . >"$tmp/graph.json"
jq -r '.[].ImportPath' "$tmp/graph.json" >"$tmp/packages.txt"
go run ./scripts/internal/cishard --package-manifest scripts/ci-package-lanes.json --packages "$tmp/packages.txt" >/dev/null
git diff --no-renames --name-only HEAD^1 HEAD | python3 scripts/ci-pr-select.py --root "$PWD" --graph "$tmp/graph.json" --manifest scripts/ci-package-lanes.json >"$out"
