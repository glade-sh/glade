#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 3 ]]; then
	echo "usage: scripts/ci-race-shard-aggregate.sh <./internal/visualforce|./internal/lwc/compile> <output-json> <expected-head-sha> <shard-dirs...>" >&2
	exit 2
fi
case "$1" in
	./internal/visualforce) shard_count=4 ;;
	./internal/lwc/compile) shard_count=8 ;;
	*) echo "unsupported browser race aggregate package: $1" >&2; exit 2 ;;
esac
if [[ "$#" -ne "$((shard_count + 3))" ]]; then
	echo "browser race aggregate requires exactly ${shard_count} shard directories" >&2
	exit 2
fi
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
python3 - "${script_dir}/ci-go-test.sh" "$@" <<'PY'
import hashlib
import json
import math
import os
import re
import subprocess
import sys
import tempfile

summary_runner, relative_package, output_path, head_sha, *roots = sys.argv[1:]
package = "github.com/glade-sh/glade/" + relative_package.removeprefix("./")
slug = relative_package.removeprefix("./").replace("/", "-")
shard_count = 4 if relative_package == "./internal/visualforce" else 8
if not re.fullmatch(r"[0-9a-f]{40}", head_sha):
    raise SystemExit("race shard aggregate requires the expected checkout head SHA")

def reject_duplicates(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key {key!r}")
        result[key] = value
    return result

def load(path):
    with open(path, encoding="utf-8") as source:
        return json.load(source, object_pairs_hook=reject_duplicates)

expected_discovery = expected_plan = None
tested = []
deferred = []
counts = []
for index, root in enumerate(roots):
    runner = load(os.path.join(root, "runner-validation.json"))
    keys = {"schema_version", "package", "head_sha", "shard_index", "discovery_sha256", "plan_sha256"}
    if not isinstance(runner, dict) or set(runner) != keys or type(runner["schema_version"]) is not int or runner["schema_version"] != 1 or runner["package"] != package or runner["head_sha"] != head_sha or type(runner["shard_index"]) is not int or runner["shard_index"] != index:
        raise SystemExit(f"race shard {index} has invalid runner identity")
    data = []
    for name, filename in (("discovery", "discovery.txt"), ("plan", "plan.json")):
        with open(os.path.join(root, filename), "rb") as source:
            raw = source.read()
        if hashlib.sha256(raw).hexdigest() != runner[name + "_sha256"]:
            raise SystemExit(f"race shard {index} has mismatched {name}")
        data.append(raw)
    discovery, plan_bytes = data
    if index == 0:
        expected_discovery, expected_plan = discovery, plan_bytes
    if discovery != expected_discovery or plan_bytes != expected_plan:
        raise SystemExit("race shard runners used different discovery or plans")
    names = discovery.decode("utf-8").splitlines()
    if not names or names != sorted(set(names)) or any(not re.fullmatch(r"Test[A-Za-z0-9_]*", name) for name in names):
        raise SystemExit("race shard aggregate discovery is not canonical")
    plan = json.loads(plan_bytes, object_pairs_hook=reject_duplicates)
    if not isinstance(plan, dict) or set(plan) != {"version", "package", "historyUsed", "shards"} or type(plan["version"]) is not int or plan["version"] != 1 or plan["package"] != package or type(plan["historyUsed"]) is not bool or not isinstance(plan["shards"], list) or len(plan["shards"]) != shard_count:
        raise SystemExit("race shard aggregate plan has invalid schema")
    union = []
    for shard_index, shard in enumerate(plan["shards"]):
        if not isinstance(shard, dict) or set(shard) != {"index", "tests", "estimatedDurationMillis", "regex"} or type(shard["index"]) is not int or shard["index"] != shard_index:
            raise SystemExit("race shard aggregate has invalid selection")
        tests = shard["tests"]
        if not isinstance(tests, list) or not tests or any(not isinstance(name, str) or not re.fullmatch(r"Test[A-Za-z0-9_]*", name) for name in tests) or tests != sorted(set(tests)) or type(shard["estimatedDurationMillis"]) is not int or shard["estimatedDurationMillis"] < 0 or shard["regex"] != "^(?:" + "|".join(re.escape(name) for name in tests) + ")$":
            raise SystemExit("race shard aggregate selection is not canonical")
        union.extend(tests)
        if shard_index != index and any(os.path.exists(os.path.join(root, f"shard-{shard_index}", name)) for name in ("events.json", "resource.json")):
            raise SystemExit("race shard runner executed an unassigned shard")
    if sorted(union) != names or len(union) != len(set(union)):
        raise SystemExit("race shard plan does not exactly cover discovery")
    shard_dir = os.path.join(root, f"shard-{index}")
    selection = load(os.path.join(shard_dir, "selection.json"))
    if selection != plan["shards"][index]:
        raise SystemExit("race shard selection differs from the shared plan")
    resource = load(os.path.join(shard_dir, "resource.json"))
    resource_keys = {"schema_version", "lane", "elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb", "exit_status"}
    if not isinstance(resource, dict) or set(resource) != resource_keys or type(resource["schema_version"]) is not int or resource["schema_version"] != 1 or type(resource["exit_status"]) is not int or resource["exit_status"] != 0 or resource["lane"] != f"race-{slug}-shard-{index}":
        raise SystemExit("race shard resource result is invalid")
    if any(type(resource[key]) not in (int, float) or not math.isfinite(resource[key]) or resource[key] < 0 for key in ("elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb")):
        raise SystemExit("race shard resource metrics are invalid")
    events_path = os.path.join(shard_dir, "events.json")
    with tempfile.TemporaryDirectory(prefix="race-shard-summary-") as temporary:
        metadata = os.path.join(temporary, "packages.tsv")
        summary = os.path.join(temporary, "summary.json")
        with open(metadata, "w", encoding="utf-8") as target:
            target.write(package + "\thas-tests\n")
        subprocess.run(["bash", summary_runner, "package-summary", "race", f"{slug}-shard-{index}", events_path, summary, metadata, package], check=True)
        shard_deferred = load(summary)["deferred"]
        deferred.extend(shard_deferred)
    deferred_names = {row["test"] for row in shard_deferred}
    terminals = {}
    with open(events_path, encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            event = json.loads(line, object_pairs_hook=reject_duplicates)
            if not isinstance(event, dict) or event.get("Package") != package:
                raise SystemExit("race shard contains an invalid event or wrong package")
            name, action = event.get("Test"), event.get("Action")
            if isinstance(name, str) and "/" not in name and action in ("pass", "fail", "skip"):
                terminals.setdefault(name, []).append(action)
    if sorted(terminals) != selection["tests"] or any(actions != ["pass"] and not (name in deferred_names and actions == ["skip"]) for name, actions in terminals.items()):
        raise SystemExit("race shard results do not exactly match selection")
    tested.extend(terminals)
    counts.append(len(terminals))

if sorted(tested) != names or len(tested) != len(set(tested)):
    raise SystemExit("race shard tested union does not exactly cover discovery")
result = {"schema_version": 1, "package": package, "head_sha": head_sha, "discovered_count": len(names), "shard_counts": counts, "deferred": deferred, "valid": True}
output_dir = os.path.dirname(os.path.abspath(output_path))
os.makedirs(output_dir, exist_ok=True)
descriptor, temporary = tempfile.mkstemp(prefix=".race-shard-aggregate.", dir=output_dir, text=True)
with os.fdopen(descriptor, "w", encoding="utf-8") as target:
    json.dump(result, target, sort_keys=True)
    target.write("\n")
os.replace(temporary, output_path)
PY
