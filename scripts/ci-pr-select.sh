#!/usr/bin/env bash
# Write the JSON list of Go packages a pull request must test: the changed
# packages and every package that transitively imports them, through
# production or test imports, in the root module or through the nested Apex
# parser module. A changed *_test.go file selects only its own package,
# because importers cannot see another package's test files. Run from the
# repository root on the merge commit. The PR test shards and the shared
# test-binary producers call this one script so they always agree on the
# selection.
set -euo pipefail
out=${1:?usage: scripts/ci-pr-select.sh OUTPUT_JSON}
tmp=$(mktemp -d "${RUNNER_TEMP:-/tmp}/ci-pr-select.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
go list -json ./... | jq -s . >"$tmp/graph.json"
# The nested Apex parser module is a separate Go module: its packages are
# never executed by the root module's lanes, but a parser change must reach
# the root packages that import the parser.
(cd third_party/glade-apex-parser && go list -json ./...) | jq -s . >"$tmp/parser-graph.json"
jq -r '.[].ImportPath' "$tmp/graph.json" >"$tmp/packages.txt"
go run ./scripts/internal/cishard --package-manifest scripts/ci-package-lanes.json --packages "$tmp/packages.txt" >/dev/null
git diff --no-renames --name-only HEAD^1 HEAD | python3 scripts/ci-pr-select.py --root "$PWD" --graph "$tmp/graph.json" --parser-graph "$tmp/parser-graph.json" --manifest scripts/ci-package-lanes.json >"$tmp/selected.json"
git diff --no-renames --name-only -z --diff-filter=ACDMRTUXB HEAD^1 HEAD^2 | python3 scripts/ci-pr-browser-select.py "$tmp/selected.json" >"$tmp/browser.json"
jq -c '.selected' "$tmp/browser.json" >"$out"
if [[ $# -ge 2 ]]; then jq -r '.browser' "$tmp/browser.json" >"$2"; fi
