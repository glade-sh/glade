#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

git diff --check
go test ./internal/repoguard
go test ./internal/gladecli ./internal/cliui
npm test --prefix site
npm run build --prefix site
go test ./...
scripts/smoke.sh
