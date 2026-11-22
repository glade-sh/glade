#!/usr/bin/env bash
# Execute assigned PR packages and retain the existing package/skip authority.
set -euo pipefail
dir=${1:?usage: ci-pr-packages.sh DIR PACKAGE...}
shift
if (( $# == 0 )); then echo 'no assigned packages' >&2; exit 2; fi
mkdir -p "$dir"
go list -f '{{.ImportPath}}{{"\t"}}{{if or .TestGoFiles .XTestGoFiles}}has-tests{{else}}no-tests{{end}}' "$@" > "$dir/metadata.tsv"
python3 scripts/ci-pr-package-discovery.py collect "$dir" "$@"
rc=0
test_selection=()
if [[ -n "${CI_TEST_RUN_REGEX:-}" ]]; then test_selection=(-run "$CI_TEST_RUN_REGEX"); fi
bash scripts/ci-pr-browser-go-test.sh "$dir/events.json" "${test_selection[@]}" "$@" || rc=$?
validation=0
python3 scripts/ci-pr-package-discovery.py validate "$dir" "$@" || validation=$?
bash scripts/ci-go-test.sh package-summary remaining-go 'PR packages' "$dir/events.json" "$dir/summary.json" "$dir/metadata.tsv" "$@" || validation=$?
if (( rc != 0 )); then exit "$rc"; fi
exit "$validation"
