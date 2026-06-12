#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 1 ]]; then
  echo "usage: $0 test-results/<run>.json [limit]" >&2
  exit 64
fi

file="$1"
limit="${2:-12}"

jq -r --argjson limit "$limit" '
  [.suites[].cases[]]
  | group_by(.className)
  | map({
      class: .[0].className,
      count: length,
      total: (map(.durationMs) | add),
      max: (map(.durationMs) | max),
      slowMethod: (max_by(.durationMs).methodName)
    })
  | sort_by(-.total)
  | .[:$limit][]
  | [.class, .count, .total, .max, .slowMethod]
  | @tsv
' "$file"
