#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

export GOMAXPROCS="${GOMAXPROCS:-2}"

git diff --check
scripts/release-tools-lockstep.sh "$ROOT" "${GLADE_TOOLS_DIR:-$ROOT/../glade-tools}"
npm run release:check --prefix site
scripts/ci-go-test.sh local-release
scripts/smoke.sh
