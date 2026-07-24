#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 4 ]]; then
	echo "usage: scripts/ci-race-apextest-aggregate.sh <runner-a-dir> <runner-b-dir> <output-json> <expected-head-sha>" >&2
	exit 2
fi

python3 - "$@" <<'PY'
import hashlib
import json
import math
import os
import re
import sys
import tempfile

runner_a_dir, runner_b_dir, output_path, expected_head_sha = sys.argv[1:]
package = "github.com/glade-sh/glade/internal/apextest"
runner_indexes = {"a": [0, 1, 2, 3, 4, 5, 6], "b": [7]}
test_pattern = re.compile(r"Test[A-Za-z0-9_]*\Z")
sha_pattern = re.compile(r"[0-9a-f]{40}\Z")
digest_pattern = re.compile(r"[0-9a-f]{64}\Z")
if not sha_pattern.fullmatch(expected_head_sha):
    raise SystemExit("race apextest aggregate expected head is not a 40-character lowercase SHA")

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

def load_json(path, label):
    try:
        with open(path, encoding="utf-8") as source:
            return json.load(source, object_pairs_hook=reject_duplicates)
    except (OSError, ValueError) as error:
        raise SystemExit(f"race apextest aggregate has invalid {label}: {error}")

def load_bytes(path, label):
    try:
        with open(path, "rb") as source:
            return source.read()
    except OSError as error:
        raise SystemExit(f"race apextest aggregate is missing {label}: {error}")

def require_exact_object(value, keys, label):
    if not isinstance(value, dict) or set(value) != keys:
        raise SystemExit(f"race apextest aggregate has invalid {label}")

def validate_resource(path, lane):
    value = load_json(path, f"resource evidence {lane}")
    require_exact_object(value, {"schema_version", "lane", "elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb", "exit_status"}, f"resource evidence {lane}")
    if type(value["schema_version"]) is not int or value["schema_version"] != 1 or value["lane"] != lane or type(value["exit_status"]) is not int or value["exit_status"] != 0:
        raise SystemExit(f"race apextest aggregate has invalid resource evidence {lane}")
    for field in ("elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb"):
        number = value[field]
        if isinstance(number, bool) or not isinstance(number, (int, float)) or not math.isfinite(number) or number < 0:
            raise SystemExit(f"race apextest aggregate has invalid resource evidence {lane}")

def validate_events(path, expected_tests, index):
    terminals = {}
    try:
        with open(path, encoding="utf-8") as source:
            for number, line in enumerate(source, 1):
                if not line.strip():
                    continue
                event = json.loads(line, object_pairs_hook=reject_duplicates)
                if not isinstance(event, dict):
                    raise ValueError("event is not an object")
                if event.get("Package") != package:
                    raise ValueError("wrong package")
                name, action = event.get("Test"), event.get("Action")
                if isinstance(name, str) and "/" not in name and action in ("pass", "fail", "skip"):
                    terminals.setdefault(name, []).append(action)
    except (OSError, ValueError) as error:
        raise SystemExit(f"race apextest aggregate has invalid shard {index} events: {error}")
    if set(terminals) != set(expected_tests):
        raise SystemExit(f"race apextest aggregate shard {index} terminal set does not match selection")
    for name in expected_tests:
        if terminals[name] != ["pass"]:
            raise SystemExit(f"race apextest aggregate shard {index} has non-passing or duplicate terminal for {name}")

def validate_runner(root, expected_runner):
    runner = load_json(os.path.join(root, "runner-validation.json"), "runner validation")
    runner_keys = {"schema_version", "runner", "package", "head_sha", "assigned_indexes", "discovered_count", "discovery_sha256", "plan_sha256", "binary_sha256", "binary_size_bytes", "binary_removed"}
    require_exact_object(runner, runner_keys, "runner validation")
    if type(runner["schema_version"]) is not int or runner["schema_version"] != 1 or runner["runner"] != expected_runner or runner["package"] != package or not isinstance(runner["head_sha"], str) or not sha_pattern.fullmatch(runner["head_sha"]):
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if runner["assigned_indexes"] != runner_indexes[expected_runner] or type(runner["discovered_count"]) is not int or runner["discovered_count"] <= 0:
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if any(not isinstance(runner[field], str) or not digest_pattern.fullmatch(runner[field]) for field in ("discovery_sha256", "plan_sha256", "binary_sha256")):
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if type(runner["binary_size_bytes"]) is not int or runner["binary_size_bytes"] <= 0 or runner["binary_removed"] is not True:
        raise SystemExit("race apextest aggregate has invalid runner validation")

    discovery_bytes = load_bytes(os.path.join(root, "discovery.txt"), "discovery")
    discovery = discovery_bytes.decode("utf-8").splitlines()
    if len(discovery) != runner["discovered_count"] or discovery != sorted(discovery) or len(discovery) != len(set(discovery)) or any(not test_pattern.fullmatch(name) for name in discovery):
        raise SystemExit("race apextest aggregate has invalid discovery")
    if hashlib.sha256(discovery_bytes).hexdigest() != runner["discovery_sha256"]:
        raise SystemExit("race apextest aggregate discovery identity mismatch")

    plan_bytes = load_bytes(os.path.join(root, "plan.json"), "plan")
    if hashlib.sha256(plan_bytes).hexdigest() != runner["plan_sha256"]:
        raise SystemExit("race apextest aggregate plan identity mismatch")
    plan = load_json(os.path.join(root, "plan.json"), "plan")
    require_exact_object(plan, {"version", "package", "historyUsed", "shards"}, "plan")
    if type(plan["version"]) is not int or plan["version"] != 1 or plan["package"] != package or not isinstance(plan["historyUsed"], bool) or not isinstance(plan["shards"], list) or len(plan["shards"]) != 8:
        raise SystemExit("race apextest aggregate has invalid plan")
    union = []
    shard_tests = {}
    for index, shard in enumerate(plan["shards"]):
        require_exact_object(shard, {"index", "tests", "estimatedDurationMillis", "regex"}, f"plan shard {index}")
        tests = shard["tests"]
        if type(shard["index"]) is not int or shard["index"] != index or not isinstance(tests, list) or not tests or tests != sorted(tests) or len(tests) != len(set(tests)):
            raise SystemExit(f"race apextest aggregate has invalid plan shard {index}")
        if any(not isinstance(name, str) or not test_pattern.fullmatch(name) for name in tests) or type(shard["estimatedDurationMillis"]) is not int or shard["estimatedDurationMillis"] < 0:
            raise SystemExit(f"race apextest aggregate has invalid plan shard {index}")
        if shard["regex"] != "^(?:" + "|".join(re.escape(name) for name in tests) + ")$":
            raise SystemExit(f"race apextest aggregate has non-canonical plan shard {index}")
        union.extend(tests)
        shard_tests[index] = tests
    if len(union) != len(set(union)) or sorted(union) != discovery:
        raise SystemExit("race apextest aggregate plan union does not exactly match discovery")

    binary = load_json(os.path.join(root, "binary.json"), "binary metadata")
    require_exact_object(binary, {"schema_version", "package", "sha256", "size_bytes", "removed"}, "binary metadata")
    if type(binary["schema_version"]) is not int or binary["schema_version"] != 1 or binary["package"] != package or binary["sha256"] != runner["binary_sha256"] or binary["size_bytes"] != runner["binary_size_bytes"] or binary["removed"] is not True:
        raise SystemExit("race apextest aggregate binary identity or cleanup mismatch")
    validate_resource(os.path.join(root, "build-resource.json"), "race-internal-apextest-build")

    for index in range(8):
        shard_dir = os.path.join(root, f"shard-{index}")
        events_path = os.path.join(shard_dir, "events.json")
        resource_path = os.path.join(shard_dir, "resource.json")
        if index not in runner_indexes[expected_runner]:
            if os.path.exists(events_path) or os.path.exists(resource_path):
                raise SystemExit(f"race apextest aggregate runner {expected_runner} executed unassigned shard {index}")
            continue
        selection = load_json(os.path.join(shard_dir, "selection.json"), f"shard {index} selection")
        if selection != plan["shards"][index]:
            raise SystemExit(f"race apextest aggregate shard {index} selection does not match plan")
        validate_resource(resource_path, f"race-internal-apextest-shard-{index}")
        validate_events(events_path, shard_tests[index], index)
    return runner, discovery, [len(shard_tests[index]) for index in range(8)]

runner_a, discovery_a, counts_a = validate_runner(runner_a_dir, "a")
runner_b, discovery_b, counts_b = validate_runner(runner_b_dir, "b")
for field in ("package", "head_sha", "discovered_count", "discovery_sha256", "plan_sha256", "binary_sha256", "binary_size_bytes"):
    if runner_a[field] != runner_b[field]:
        raise SystemExit(f"race apextest aggregate {field} identity mismatch")
if discovery_a != discovery_b:
    raise SystemExit("race apextest aggregate discovery mismatch")
if runner_a["head_sha"] != expected_head_sha or runner_b["head_sha"] != expected_head_sha:
    raise SystemExit("race apextest aggregate runner checkout head does not match expected head")
result = {"schema_version": 1, "package": package, "head_sha": runner_a["head_sha"], "discovered_count": len(discovery_a), "discovery_sha256": runner_a["discovery_sha256"], "plan_sha256": runner_a["plan_sha256"], "binary_sha256": runner_a["binary_sha256"], "shard_counts": counts_a, "valid": True}
output_dir = os.path.dirname(os.path.abspath(output_path))
os.makedirs(output_dir, exist_ok=True)
descriptor, temporary = tempfile.mkstemp(prefix=".apextest-aggregate.", dir=output_dir, text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8") as target:
        json.dump(result, target, sort_keys=True, separators=(",", ":"))
        target.write("\n")
    os.replace(temporary, output_path)
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
PY
