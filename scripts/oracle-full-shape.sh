#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Build full oracle shape from ALL stub surfaces plus runtime-path evidence.

Usage:
  bash scripts/oracle-full-shape.sh --target-org <org-alias> [options]

Options:
  --target-org <alias>                Salesforce org alias for generated shard scripts.
  --run-id <id>                       Run ID. Default: full-all-apex-YYYYmmdd-HHMMSS
  --runs-dir <dir>                    Default: .oaer/oracle/runs
  --example-input <dir>               Default: example-projects
  --runtime-inventory <file>          Default: .oaer/runtime-path-inventory-with-stubs.json
  --runtime-markdown <file>           Default: .oaer/runtime-path-inventory-with-stubs.md
  --inventory-out <file>              Default: docs/generated/apex-oracle/INVENTORY.json
  --domains-out <file>                Default: docs/generated/apex-oracle/DOMAINS.json
  --manifest-out <file>               Default: docs/generated/apex-oracle/PROBE_MANIFEST.json
  --work-queue-out <file>             Default: docs/generated/apex-oracle/WORK_QUEUE.json
  --ranked-work-queue-out <file>      Default: docs/generated/apex-oracle/WORK_QUEUE.runtime-ranked.json
  --rank-report-out <file>            Default: docs/generated/apex-oracle/RUNTIME_QUEUE_REPORT.json
  --shard-count <n>                   Default: 256
  --skip-runtime-inventory            Reuse existing runtime inventory file.
USAGE
}

TARGET_ORG=""
RUN_ID="full-all-apex-$(date +%Y%m%d-%H%M%S)"
RUNS_DIR=".oaer/oracle/runs"
EXAMPLE_INPUT="example-projects"
RUNTIME_INVENTORY=".oaer/runtime-path-inventory-with-stubs.json"
RUNTIME_MD=".oaer/runtime-path-inventory-with-stubs.md"
INVENTORY_OUT="docs/generated/apex-oracle/INVENTORY.json"
DOMAINS_OUT="docs/generated/apex-oracle/DOMAINS.json"
MANIFEST_OUT="docs/generated/apex-oracle/PROBE_MANIFEST.json"
WORK_QUEUE_OUT="docs/generated/apex-oracle/WORK_QUEUE.json"
RANKED_QUEUE_OUT="docs/generated/apex-oracle/WORK_QUEUE.runtime-ranked.json"
RANK_REPORT_OUT="docs/generated/apex-oracle/RUNTIME_QUEUE_REPORT.json"
SHARD_COUNT=256
SKIP_RUNTIME_INVENTORY=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target-org) TARGET_ORG="$2"; shift 2 ;;
    --run-id) RUN_ID="$2"; shift 2 ;;
    --runs-dir) RUNS_DIR="$2"; shift 2 ;;
    --example-input) EXAMPLE_INPUT="$2"; shift 2 ;;
    --runtime-inventory) RUNTIME_INVENTORY="$2"; shift 2 ;;
    --runtime-markdown) RUNTIME_MD="$2"; shift 2 ;;
    --inventory-out) INVENTORY_OUT="$2"; shift 2 ;;
    --domains-out) DOMAINS_OUT="$2"; shift 2 ;;
    --manifest-out) MANIFEST_OUT="$2"; shift 2 ;;
    --work-queue-out) WORK_QUEUE_OUT="$2"; shift 2 ;;
    --ranked-work-queue-out) RANKED_QUEUE_OUT="$2"; shift 2 ;;
    --rank-report-out) RANK_REPORT_OUT="$2"; shift 2 ;;
    --shard-count) SHARD_COUNT="$2"; shift 2 ;;
    --skip-runtime-inventory) SKIP_RUNTIME_INVENTORY=1; shift 1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$TARGET_ORG" ]]; then
  echo "--target-org is required" >&2
  usage
  exit 1
fi

echo "[1/8] doctor"
go run ./cmd/oaer compat oracle doctor --json

echo "[2/8] oracle inventory from stubs"
go run ./cmd/oaer compat oracle inventory --stubs example-projects/stubs --output "$INVENTORY_OUT"

echo "[3/8] oracle domains"
go run ./cmd/oaer compat oracle domains --output "$DOMAINS_OUT"

if [[ "$SKIP_RUNTIME_INVENTORY" -eq 0 ]]; then
  echo "[4/8] runtime path inventory from example projects + stubs"
  go run ./scripts/runtime-path-inventory.go \
    --input "$EXAMPLE_INPUT" \
    --include-stubs \
    --output "$RUNTIME_INVENTORY" \
    --markdown "$RUNTIME_MD"
else
  echo "[4/8] runtime path inventory skipped (reuse): $RUNTIME_INVENTORY"
fi

echo "[5/8] plan full oracle queue (ALL stubs)"
go run ./cmd/oaer compat oracle plan \
  --inventory "$INVENTORY_OUT" \
  --domains "$DOMAINS_OUT" \
  --manifest "$MANIFEST_OUT" \
  --work-queue "$WORK_QUEUE_OUT" \
  --shard-count "$SHARD_COUNT"

echo "[6/8] rank queue by runtime-path evidence"
go run ./scripts/oracle-runtime-queue-prioritize.go \
  --runtime-inventory "$RUNTIME_INVENTORY" \
  --work-queue "$WORK_QUEUE_OUT" \
  --output "$RANKED_QUEUE_OUT" \
  --report "$RANK_REPORT_OUT"

echo "[7/8] generate Apex probes from ranked full queue"
go run ./cmd/oaer compat oracle generate \
  --run-id "$RUN_ID" \
  --runs-dir "$RUNS_DIR" \
  --work-queue "$RANKED_QUEUE_OUT"

echo "[8/8] generate shard scripts"
go run ./cmd/oaer compat oracle scripts \
  --run-id "$RUN_ID" \
  --runs-dir "$RUNS_DIR" \
  --work-queue "$RANKED_QUEUE_OUT" \
  --target-org "$TARGET_ORG" \
  --shard-count "$SHARD_COUNT"

RUN_DIR="$RUNS_DIR/$RUN_ID"
ALL_SCRIPT="$RUN_DIR/generated/scripts/07-run-all-shards.sh"

echo "ready"
echo "runId: $RUN_ID"
echo "runDir: $RUN_DIR"
echo "runtimeInventory: $RUNTIME_INVENTORY"
echo "rankedWorkQueue: $RANKED_QUEUE_OUT"
echo "rankReport: $RANK_REPORT_OUT"
echo "next: bash $ALL_SCRIPT"
