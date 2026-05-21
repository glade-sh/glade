#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
perf_root="${OAER_PERF_ROOT:-/tmp/oaer-perf}"
if [[ -e "$perf_root" && ! -d "$perf_root" ]]; then
  perf_root="/tmp/oaer-perf-runs"
fi
out_dir="${OAER_PERF_DIR:-$perf_root/$timestamp}"
binary="${OAER_BIN:-$repo_root/bin/oaer-perf}"
project="${1:-example-projects/src-nmb-nutpl-develop}"
parallel="${OAER_PARALLEL:-$(sysctl -n hw.logicalcpu 2>/dev/null || nproc 2>/dev/null || printf '1')}"

mkdir -p "$out_dir" "$repo_root/bin"

if [[ ! -x "$binary" ]]; then
  go build -trimpath -o "$binary" "$repo_root/cmd/oaer"
fi

perf_json="$out_dir/local-tests.perf.json"
cpu_profile="$out_dir/local-tests.cpu.pprof"
mem_profile="$out_dir/local-tests.mem.pprof"
summary="$out_dir/local-tests.summary.json"

"$binary" compat local-tests \
  --project "$repo_root/$project" \
  --json \
  --timeout "${OAER_TIMEOUT_MS:-60000}" \
  --parallel "$parallel" \
  --parallel-methods \
  --perf-json "$perf_json" \
  --cpu-profile "$cpu_profile" \
  --mem-profile "$mem_profile" >"$summary"

printf 'perf dir: %s\n' "$out_dir"
python3 - "$perf_json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    perf = json.load(handle)

summary = perf.get("summary", {})
clone_stats = perf.get("cloneStats", {})
print(
    "casesRun={cases} durationMs={duration} pass={passed} fail={failed} unsupported={unsupported}".format(
        cases=perf.get("casesRun", 0),
        duration=perf.get("durationMs", 0),
        passed=summary.get("pass", 0),
        failed=summary.get("fail", 0),
        unsupported=summary.get("unsupported", 0),
    )
)
print(
    "cloneRuntimeOrg={cloneRuntimeOrgCalls} cloneRuntime={cloneRuntimeCalls} rollbackClones={cloneRollbackSnapshotCalls} journalRollbacks={journalRollbacks} cloneFallbacks={cloneFallbacks}".format(
        **{
            "cloneRuntimeOrgCalls": clone_stats.get("cloneRuntimeOrgCalls", 0),
            "cloneRuntimeCalls": clone_stats.get("cloneRuntimeCalls", 0),
            "cloneRollbackSnapshotCalls": clone_stats.get("cloneRollbackSnapshotCalls", 0),
            "journalRollbacks": clone_stats.get("journalRollbacks", 0),
            "cloneFallbacks": clone_stats.get("cloneFallbacks", 0),
        }
    )
)
for row in perf.get("topCloneClasses", [])[:5]:
    print(
        "cloneClass class={class_name} setup={setupClones} test={testClones} durationMs={durationMs}".format(
            **{
                "class_name": row.get("class", ""),
                "setupClones": row.get("setupClones", 0),
                "testClones": row.get("testClones", 0),
                "durationMs": row.get("durationMs", 0),
            }
        )
    )
PY
