#!/usr/bin/env bash
set -euo pipefail
events=${1:?usage: ci-pr-browser-go-test.sh EVENTS PACKAGE...}
shift
go test -json -count=1 -timeout=0 "$@" | tee "$events"
