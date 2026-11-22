#!/usr/bin/env bash
set -euo pipefail
dir=${1:?usage: ci-pr-doctor-test.sh DIR REGEX}
selection=${2:?missing doctor selection}
mkdir -p "$dir"
rc=0
go test -json -count=1 -timeout=0 -run "$selection" ./internal/gladecli | tee "$dir/events.json" || rc=$?
printf 'github.com/glade-sh/glade/internal/gladecli\thas-tests\n' > "$dir/metadata.tsv"
validation=0
bash scripts/ci-go-test.sh package-summary gladecli-ci 'PR doctor' "$dir/events.json" "$dir/summary.json" "$dir/metadata.tsv" ./internal/gladecli || validation=$?
if (( rc != 0 )); then exit "$rc"; fi
exit "$validation"
