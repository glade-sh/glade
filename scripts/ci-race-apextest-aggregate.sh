#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 4 ]]; then
	echo "usage: scripts/ci-race-apextest-aggregate.sh <partition-dir> <runners-root> <output-json> <expected-head-sha>" >&2
	exit 2
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
python3 - "$@" "${script_dir}/ci-go-test.sh" "${script_dir}/ci-race-plan.py" <<'PY'
import hashlib
import json
import math
import os
import re
import subprocess
import sys
import tempfile

partition_dir, runners_root, output_path, expected_head_sha, summary_runner, plan_script = sys.argv[1:]
package = "github.com/glade-sh/glade/internal/apextest"
artifact_prefix = "race-internal-apextest-"
sha_pattern = re.compile(r"[0-9a-f]{40}\Z")
digest_pattern = re.compile(r"[0-9a-f]{64}\Z")
test_pattern = re.compile(r"Test[A-Za-z0-9_]*\Z")
runner_pattern = re.compile(r"[a-z]{1,8}\Z")
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

# The partition is the one plan shared by the workflow, runner and aggregate.
# Its digests, runner assignments and deadlines are re-derived from its own
# discovery and plan before any runner evidence is accepted.
verification = subprocess.run([sys.executable, plan_script, "verify", partition_dir], capture_output=True, text=True)
if verification.returncode != 0:
    raise SystemExit(f"race apextest aggregate rejected the partition: {verification.stdout}{verification.stderr}")

partition = load_json(os.path.join(partition_dir, "partition.json"), "partition")
require_exact_object(partition, {
    "schema_version", "package", "normal_duration_multiplier", "shard_count",
    "discovery_sha256", "plan_sha256", "binary_sha256", "binary_size_bytes",
    "shards", "runners"}, "partition")
if partition["package"] != package:
    raise SystemExit("race apextest aggregate partition has the wrong package")
shard_count = partition["shard_count"]
if type(shard_count) is not int or shard_count <= 0:
    raise SystemExit("race apextest aggregate partition has an invalid shard count")

discovery_bytes = load_bytes(os.path.join(partition_dir, "discovery.txt"), "discovery")
if hashlib.sha256(discovery_bytes).hexdigest() != partition["discovery_sha256"]:
    raise SystemExit("race apextest aggregate partition discovery identity mismatch")
discovery = discovery_bytes.decode("utf-8").splitlines()
if not discovery or discovery != sorted(set(discovery)) or any(not test_pattern.fullmatch(name) for name in discovery):
    raise SystemExit("race apextest aggregate partition discovery is not canonical")

plan_bytes = load_bytes(os.path.join(partition_dir, "plan.json"), "plan")
if hashlib.sha256(plan_bytes).hexdigest() != partition["plan_sha256"]:
    raise SystemExit("race apextest aggregate partition plan identity mismatch")
plan = json.loads(plan_bytes, object_pairs_hook=reject_duplicates)
require_exact_object(plan, {"version", "package", "historyUsed", "shards"}, "partition plan")
if type(plan["version"]) is not int or plan["version"] != 1 or plan["package"] != package or not isinstance(plan["historyUsed"], bool) or not isinstance(plan["shards"], list) or len(plan["shards"]) != shard_count:
    raise SystemExit("race apextest aggregate partition plan has an invalid schema")
shard_tests = {}
union = []
for index, shard in enumerate(plan["shards"]):
    require_exact_object(shard, {"index", "tests", "estimatedDurationMillis", "regex"}, f"partition plan shard {index}")
    tests = shard["tests"]
    if type(shard["index"]) is not int or shard["index"] != index or not isinstance(tests, list) or not tests or tests != sorted(set(tests)):
        raise SystemExit(f"race apextest aggregate partition plan shard {index} is invalid")
    if any(not isinstance(name, str) or not test_pattern.fullmatch(name) for name in tests) or type(shard["estimatedDurationMillis"]) is not int or shard["estimatedDurationMillis"] < 0:
        raise SystemExit(f"race apextest aggregate partition plan shard {index} is invalid")
    if shard["regex"] != "^(?:" + "|".join(re.escape(name) for name in tests) + ")$":
        raise SystemExit(f"race apextest aggregate partition plan shard {index} has a non-canonical regex")
    union.extend(tests)
    shard_tests[index] = tests
if len(union) != len(set(union)) or sorted(union) != discovery:
    raise SystemExit("race apextest aggregate partition plan union does not exactly match discovery")

assignments = {}
for runner in partition["runners"]:
    require_exact_object(runner, {"runner", "shard_indexes", "batch_timeout_seconds", "step_timeout_minutes", "job_timeout_minutes"}, "partition runner")
    if not isinstance(runner["runner"], str) or not runner_pattern.fullmatch(runner["runner"]) or runner["runner"] in assignments:
        raise SystemExit("race apextest aggregate partition has an invalid runner id")
    indexes = runner["shard_indexes"]
    if not isinstance(indexes, list) or not indexes or sorted(indexes) != indexes or len(indexes) != len(set(indexes)) or any(type(index) is not int or not 0 <= index < shard_count for index in indexes):
        raise SystemExit(f"race apextest aggregate partition runner {runner['runner']} has invalid shard indexes")
    assignments[runner["runner"]] = indexes
if sorted(index for indexes in assignments.values() for index in indexes) != list(range(shard_count)):
    raise SystemExit("race apextest aggregate partition runner assignments do not exactly cover the shards")

try:
    runner_dirs = sorted(os.listdir(runners_root))
except OSError as error:
    raise SystemExit(f"race apextest aggregate has no runner evidence root: {error}")
roots = {}
for name in runner_dirs:
    if not name.startswith(artifact_prefix):
        raise SystemExit(f"race apextest aggregate found unexpected evidence directory {name!r}")
    matches = [runner for runner in assignments if name.startswith(artifact_prefix + runner + "-")]
    if len(matches) != 1:
        raise SystemExit(f"race apextest aggregate found evidence for an unassigned runner: {name!r}")
    runner = matches[0]
    if runner in roots:
        raise SystemExit(f"race apextest aggregate found duplicate evidence for runner {runner}")
    roots[runner] = os.path.join(runners_root, name)
missing = sorted(set(assignments) - set(roots))
if missing:
    raise SystemExit(f"race apextest aggregate is missing runner evidence for {', '.join(missing)}")

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
    with tempfile.TemporaryDirectory(prefix="race-aggregate-summary-") as root:
        metadata = os.path.join(root, "packages.tsv")
        summary = os.path.join(root, "summary.json")
        with open(metadata, "w", encoding="utf-8") as target:
            target.write(package + "\thas-tests\n")
        subprocess.run(["bash", summary_runner, "package-summary", "race", f"apextest-shard-{index}", path, summary, metadata, package], check=True)
        deferred = load_json(summary, "package summary")["deferred"]
    deferred_names = {row["test"] for row in deferred}
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
        if terminals[name] != ["pass"] and not (name in deferred_names and terminals[name] == ["skip"]):
            raise SystemExit(f"race apextest aggregate shard {index} has non-passing or duplicate terminal for {name}")
    return deferred

runner_keys = {"schema_version", "runner", "package", "head_sha", "assigned_indexes", "discovered_count",
               "discovery_sha256", "plan_sha256", "partition_sha256", "binary_sha256", "binary_size_bytes", "binary_removed"}
validated = {}
for runner in sorted(assignments):
    root = roots[runner]
    evidence = load_json(os.path.join(root, "runner-validation.json"), "runner validation")
    require_exact_object(evidence, runner_keys, "runner validation")
    if type(evidence["schema_version"]) is not int or evidence["schema_version"] != 1 or evidence["runner"] != runner or evidence["package"] != package:
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if not isinstance(evidence["head_sha"], str) or not sha_pattern.fullmatch(evidence["head_sha"]):
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if evidence["assigned_indexes"] != assignments[runner]:
        raise SystemExit(f"race apextest aggregate runner {runner} executed a partition assignment it was not given")
    if type(evidence["discovered_count"]) is not int or evidence["discovered_count"] != len(discovery):
        raise SystemExit("race apextest aggregate has invalid runner validation")
    for field in ("discovery_sha256", "plan_sha256", "partition_sha256", "binary_sha256"):
        if not isinstance(evidence[field], str) or not digest_pattern.fullmatch(evidence[field]):
            raise SystemExit("race apextest aggregate has invalid runner validation")
    if evidence["discovery_sha256"] != partition["discovery_sha256"] or evidence["plan_sha256"] != partition["plan_sha256"]:
        raise SystemExit(f"race apextest aggregate runner {runner} identity does not match the partition")
    if type(evidence["binary_size_bytes"]) is not int or evidence["binary_size_bytes"] <= 0 or evidence["binary_removed"] is not True:
        raise SystemExit("race apextest aggregate has invalid runner validation")
    if evidence["head_sha"] != expected_head_sha:
        raise SystemExit("race apextest aggregate runner checkout head does not match expected head")
    if evidence["binary_sha256"] != partition["binary_sha256"] or evidence["binary_size_bytes"] != partition["binary_size_bytes"]:
        raise SystemExit(f"race apextest aggregate runner {runner} used a binary that is not the partition binary")
    with open(os.path.join(partition_dir, "partition.json"), "rb") as source:
        if hashlib.sha256(source.read()).hexdigest() != evidence["partition_sha256"]:
            raise SystemExit(f"race apextest aggregate runner {runner} recorded a different partition")

    binary = load_json(os.path.join(root, "binary.json"), "binary metadata")
    require_exact_object(binary, {"schema_version", "package", "sha256", "size_bytes", "removed"}, "binary metadata")
    if type(binary["schema_version"]) is not int or binary["schema_version"] != 1 or binary["package"] != package or binary["sha256"] != evidence["binary_sha256"] or binary["size_bytes"] != evidence["binary_size_bytes"] or binary["removed"] is not True:
        raise SystemExit("race apextest aggregate binary identity or cleanup mismatch")
    validate_resource(os.path.join(root, "build-resource.json"), "race-internal-apextest-build")

    deferred = []
    for index in range(shard_count):
        shard_dir = os.path.join(root, f"shard-{index}")
        events_path = os.path.join(shard_dir, "events.json")
        resource_path = os.path.join(shard_dir, "resource.json")
        if index not in assignments[runner]:
            if os.path.exists(events_path) or os.path.exists(resource_path):
                raise SystemExit(f"race apextest aggregate runner {runner} executed unassigned shard {index}")
            continue
        selection = load_json(os.path.join(shard_dir, "selection.json"), f"shard {index} selection")
        if selection != plan["shards"][index]:
            raise SystemExit(f"race apextest aggregate shard {index} selection does not match the partition plan")
        validate_resource(resource_path, f"race-internal-apextest-shard-{index}")
        deferred.extend(validate_events(events_path, shard_tests[index], index))
    validated[runner] = {"deferred": deferred}

if len(validated) != len(assignments):
    raise SystemExit("race apextest aggregate did not validate every planned runner")
result = {
    "schema_version": 1,
    "package": package,
    "head_sha": expected_head_sha,
    "discovered_count": len(discovery),
    "discovery_sha256": partition["discovery_sha256"],
    "plan_sha256": partition["plan_sha256"],
    "partition_sha256": hashlib.sha256(load_bytes(os.path.join(partition_dir, "partition.json"), "partition.json")).hexdigest(),
    "binary_sha256": partition["binary_sha256"],
    "shard_counts": [len(shard_tests[index]) for index in range(shard_count)],
    "deferred": [row for runner in sorted(validated) for row in validated[runner]["deferred"]],
    "valid": True,
}
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
