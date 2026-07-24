#!/usr/bin/env bash
set -euo pipefail

pair=${1-}
if [[ ! "$pair" =~ ^[0-9]+$ ]] || (( 10#$pair > 999999 )); then
  echo "benchmark_cache_pair must be an integer from 0 through 999999" >&2
  exit 64
fi
