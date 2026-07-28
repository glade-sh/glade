#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

export GOMAXPROCS="${GOMAXPROCS:-2}"

git diff --check
go test ./internal/repoguard
go test ./internal/gladecli ./internal/cliui
npm test --prefix site
npm run build --prefix site
go test -count=1 -p=1 ./...
scripts/smoke.sh
