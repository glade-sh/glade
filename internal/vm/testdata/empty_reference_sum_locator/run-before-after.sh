#!/bin/sh
# Coordinator-only serial focused comparison; workers do not execute this file.
set -eu
if [ "$#" -ne 3 ]; then
  echo 'usage: run-before-after.sh BEFORE_REF AFTER_REF OUTPUT_DIR' >&2
  exit 2
fi
before_ref=$1
after_ref=$2
output_dir=$3
repo_dir=$(git rev-parse --show-toplevel)
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
comparison_dir=$(mktemp -d "${TMPDIR:-/tmp}/empty-locator-comparison.XXXXXX")
mkdir "$comparison_dir/before" "$comparison_dir/after"
git -C "$repo_dir" archive "$before_ref" | tar -x -C "$comparison_dir/before"
git -C "$repo_dir" archive "$after_ref" | tar -x -C "$comparison_dir/after"
regression=internal/vm/empty_reference_sum_locator_candidate_test.go
cp "$comparison_dir/after/$regression" "$comparison_dir/before/$regression"
export CGO_ENABLED=1
export GOMAXPROCS=1
go_bin=${GLADE_QUERY_CANDIDATE_GO:-go}
before_status=0
(
  cd "$comparison_dir/before"
  nice -n 10 "$go_bin" test -count=1 -run '^TestEmptyReferenceSumLocatorCandidateConstruction$' ./internal/vm
) >"$output_dir/before.log" 2>&1 || before_status=$?
if [ "$before_status" -eq 0 ]; then
  echo 'before unexpectedly passed the local construction candidate' >&2
  exit 1
fi
if ! rg -q 'local construction candidate failed:.*Aggregate query does not support queryMore' "$output_dir/before.log"; then
  echo 'before did not reach the expected aggregate rejection; inspect before.log' >&2
  exit 1
fi
(
  cd "$comparison_dir/after"
  nice -n 10 "$go_bin" test -count=1 -run '^TestEmptyReferenceSumLocator(CandidateConstruction|CandidateScope|KnownRejections)$' ./internal/vm
) >"$output_dir/after.log" 2>&1
echo "serial local comparison passed; archives retained at $comparison_dir"
echo "logs: $output_dir/before.log and $output_dir/after.log"
