#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-testdata/local-tests/enterprise-composed}"
OUT="${2:-reports/enterprise-demo}"

mkdir -p "$OUT"

go run ./cmd/glade inspect graph --project "$ROOT" --json > "$OUT/graph.json"
go run ./cmd/glade test --project testdata/local-tests/basic --class PassingTest --trace "$OUT/trace.json" --json --no-progress > "$OUT/test.json"
go run ./cmd/glade report assess --project "$ROOT" --format html --out "$OUT/assessment.html"
go run ./cmd/glade report cruft --project "$ROOT" --format html --out "$OUT/cruft.html"
go run ./cmd/glade report refactor-proof --project "$ROOT" --since HEAD --trace "$OUT/trace.json" --format html --out "$OUT/refactor-proof.html"

printf 'enterprise demo reports: %s\n' "$OUT"
