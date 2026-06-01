#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="${GLADE_BIN:-/tmp/glade-local-test-regression-probes}"

if [[ ! -x "$binary" ]]; then
  go build -trimpath -o "$binary" "$repo_root/cmd/glade"
fi

run_probe() {
  local name="$1"
  shift
  local out
  out="$(mktemp)"
  local start
  local end
  start="$(python3 -c 'import time; print(int(time.time() * 1000))')"
  "$binary" compat local-tests "$@" --json >"$out"
  end="$(python3 -c 'import time; print(int(time.time() * 1000))')"
  python3 - "$name" "$((end - start))" "$out" <<'PY'
import json
import sys

name = sys.argv[1]
elapsed_ms = int(sys.argv[2])
path = sys.argv[3]
with open(path, "r", encoding="utf-8") as handle:
    data = json.load(handle)

summary = data.get("summary", {})
print(
    "{name}: elapsedMs={elapsed} durationMs={duration} total={total} pass={passed} fail={failed} unsupported={unsupported} loadError={load_error} compileError={compile_error} internalError={internal_error} runtimeGap={runtime_gap}".format(
        name=name,
        elapsed=elapsed_ms,
        duration=data.get("durationMs", 0),
        total=summary.get("total", data.get("total", 0)),
        passed=summary.get("pass", 0),
        failed=summary.get("fail", 0),
        unsupported=summary.get("unsupported", 0),
        load_error=summary.get("load_error", summary.get("loadError", 0)),
        compile_error=summary.get("compile_error", summary.get("compileError", 0)),
        internal_error=summary.get("internal_error", summary.get("internalError", 0)),
        runtime_gap=summary.get("runtime_gap", summary.get("runtimeGap", 0)),
    )
)
PY
  rm -f "$out"
}

run_probe "recipes-account-trigger" \
  --project "$repo_root/example-projects/apex-recipes-main" \
  --class AccountTriggerHandler_Tests

run_probe "npsp-address-update" \
  --project "$repo_root/example-projects/NPSP-rel-3.237" \
  --class ADDR_Addresses_TEST \
  --method updateAccAddrNew
