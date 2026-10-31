#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
file_list=$(mktemp)
trap 'rm -f "$file_list"' EXIT
git ls-files -z -- '*.go' > "$file_list"
files=()
while IFS= read -r -d '' file; do
  files+=("./$file")
done < "$file_list"
if ((${#files[@]} == 0)); then
  exit 0
fi
unformatted=$(gofmt -l "${files[@]}")
if [[ -n "$unformatted" ]]; then
  printf 'Go files need gofmt:\n%s\n' "$unformatted" >&2
  exit 1
fi
