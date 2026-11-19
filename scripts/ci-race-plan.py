#!/usr/bin/env python3
"""One race plan shared by the Race workflow, runner and aggregate.

The scheduled Race run 44158211531 timed out all three Apex batches and the
VM route: the Apex runners wrapped several shards in one flat one-hour shell
deadline while each shard's Go timeout was also one hour, and the VM package
ran unsharded behind the six-wide generic matrix. This module is the single
authority that replaces those duplicated, hard-coded budgets.

Policy, applied to every planned lane:

  raced estimate   = normal duration * NORMAL_DURATION_MULTIPLIER
  shard budget     = ceil(raced estimate * SHARD_TEST_HEADROOM), with a floor
  batch deadline   = sum of the assigned shard budgets + BATCH_RESERVE_SECONDS
  step deadline    = ceil(batch / 60) + STEP_RESERVE_MINUTES
  job deadline     = step deadline + JOB_RESERVE_MINUTES, capped well under
                     GitHub's 6-hour job limit

The multiplier is a conservative interim: the one measured scheduled
observation is Apex shard 0 (hash-partitioned) still running at 3,416 s when
its one-hour batch deadline expired, against about 745 s of recorded normal
durations for its 141 table-known tests; charging its 68 unknown tests the
table median puts that shard near 1,000 s normal, an implied >= 3.4x. 4x is
the conservative rounding of that observation until measured race durations
replace it. The multiplier is a scheduling input only: budgets derived from
it surface an underestimated cost as a bounded, visible timeout with partial
evidence, never as silent under-coverage.

Subcommands:

  lanes                      emit the early lane matrix (VM shards) for the
                             Race plan job
  history DISCOVERY          emit the race-scaled Apex duration history for
                             the shared shard planner
  schedule DISCOVERY PLAN BINARY_SHA256 BINARY_SIZE
                             emit the partition manifest that binds the
                             frozen discovery, plan, binary identity, runner
                             assignments and every deadline
  matrix PARTITION_JSON      emit the workflow matrix include entries for the
                             partition runners
  verify PARTITION_DIR       re-derive the whole partition from its plan and
                             reject any deviation
"""

import hashlib
import json
import math
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
APEXTEST_PACKAGE = "github.com/glade-sh/glade/internal/apextest"
VM_PACKAGE = "github.com/glade-sh/glade/internal/vm"
TEST_NAME = re.compile(r"Test[A-Za-z0-9_]*\Z")
HEX64 = re.compile(r"[0-9a-f]{64}\Z")

NORMAL_DURATION_MULTIPLIER = 4
SHARD_TEST_HEADROOM = 1.25
SHARD_TEST_FLOOR_SECONDS = 900
SHARD_TEST_CAP_SECONDS = 7200
BATCH_RESERVE_SECONDS = 600
STEP_RESERVE_MINUTES = 5
JOB_RESERVE_MINUTES = 15
# GitHub-hosted jobs stop at 6 hours; stay under it with margin.
JOB_TIMEOUT_CAP_MINUTES = 355
APEXTEST_SHARDS = 16
RUNNER_IDS = "abcdefgh"
RUNNER_COUNT_MIN = 3
RUNNER_COUNT_MAX = 8


class PlanError(SystemExit):
    pass


def fail(message):
    raise PlanError("ci-race-plan: " + message)


def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value


def load_json(path):
    try:
        with open(path, encoding="utf-8") as source:
            return json.load(source, object_pairs_hook=reject_duplicates)
    except (OSError, ValueError) as error:
        fail(f"rejected {path}: {error}")


def require(condition, message):
    if not condition:
        fail(message)


def read_discovery(path):
    try:
        with open(path, "rb") as source:
            raw = source.read()
    except OSError as error:
        fail(f"could not read discovery {path}: {error}")
    return raw


def discovery_names(raw):
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        fail("discovery is not valid UTF-8")
    names = text.splitlines()
    require(names and all(names), "discovery is empty or has blank lines")
    require(all(TEST_NAME.fullmatch(name) for name in names), "discovery has an invalid test name")
    require(names == sorted(set(names)), "discovery is not canonical sorted unique text")
    return names


def load_config():
    config = load_json(ROOT / "scripts/ci-race-plan.json")
    require(isinstance(config, dict) and set(config) ==
            {"version", "apextestNormalHistory", "apextestNormalHistorySource", "vmLane"},
            "race plan config has an invalid schema")
    require(config["version"] == 1, "race plan config version must be 1")
    require(isinstance(config["apextestNormalHistory"], str) and config["apextestNormalHistory"],
            "race plan config needs the Apex normal history path")
    lane = config["vmLane"]
    require(isinstance(lane, dict) and set(lane) ==
            {"shards", "normalSeconds", "slowestTestNormalSeconds", "source"},
            "race plan VM lane has an invalid schema")
    for field in ("shards", "normalSeconds", "slowestTestNormalSeconds"):
        require(type(lane[field]) is int and lane[field] > 0, f"race plan VM lane has an invalid {field}")
    require(1 <= lane["shards"] <= 16, "race plan VM lane needs between 1 and 16 shards")
    return config


def shard_test_timeout_seconds(estimate_ms):
    raced_seconds = estimate_ms * NORMAL_DURATION_MULTIPLIER / 1000
    return max(math.ceil(raced_seconds * SHARD_TEST_HEADROOM), SHARD_TEST_FLOOR_SECONDS)


def runner_minutes(batch_seconds):
    step_minutes = math.ceil(batch_seconds / 60) + STEP_RESERVE_MINUTES
    return step_minutes, step_minutes + JOB_RESERVE_MINUTES


def assign_runners(shard_timeouts):
    """Bind shards to the fewest runners whose jobs fit the deadline cap."""
    ordered = sorted(range(len(shard_timeouts)), key=lambda index: (-shard_timeouts[index], index))
    for count in range(RUNNER_COUNT_MIN, RUNNER_COUNT_MAX + 1):
        loads = [0] * count
        assignment = [[] for _ in range(count)]
        for index in ordered:
            target = min(range(count), key=lambda runner: (loads[runner], runner))
            assignment[target].append(index)
            loads[target] += shard_timeouts[index]
        runners = []
        fits = True
        for runner in range(count):
            batch_seconds = loads[runner] + BATCH_RESERVE_SECONDS
            step_minutes, job_minutes = runner_minutes(batch_seconds)
            if job_minutes > JOB_TIMEOUT_CAP_MINUTES:
                fits = False
                break
            runners.append({
                "runner": RUNNER_IDS[runner],
                "shard_indexes": sorted(assignment[runner]),
                "batch_timeout_seconds": batch_seconds,
                "step_timeout_minutes": step_minutes,
                "job_timeout_minutes": job_minutes,
            })
        if fits:
            return runners
    fail("race plan cannot fit the Apex shards under the job deadline cap with "
         f"up to {RUNNER_COUNT_MAX} runners; the suite outgrew the current lanes")


def validate_plan(plan, names, package, shard_count):
    require(isinstance(plan, dict) and set(plan) == {"version", "package", "historyUsed", "shards"},
            "shard plan has an invalid schema")
    require(type(plan["version"]) is int and plan["version"] == 1, "shard plan version must be 1")
    require(plan["package"] == package, "shard plan has the wrong package")
    require(type(plan["historyUsed"]) is bool, "shard plan historyUsed must be boolean")
    shards = plan["shards"]
    require(isinstance(shards, list) and len(shards) == shard_count,
            f"shard plan must have exactly {shard_count} shards")
    union = []
    for index, shard in enumerate(shards):
        require(isinstance(shard, dict) and set(shard) ==
                {"index", "tests", "estimatedDurationMillis", "regex"},
                f"shard plan shard {index} has an invalid schema")
        require(type(shard["index"]) is int and shard["index"] == index,
                f"shard plan shard {index} has a misaligned index")
        tests = shard["tests"]
        require(isinstance(tests, list) and tests and tests == sorted(set(tests)) and
                all(isinstance(name, str) and TEST_NAME.fullmatch(name) for name in tests),
                f"shard plan shard {index} has invalid tests")
        estimate = shard["estimatedDurationMillis"]
        require(type(estimate) is int and estimate >= 0,
                f"shard plan shard {index} has an invalid estimate")
        canonical = "^(?:" + "|".join(re.escape(name) for name in tests) + ")$"
        require(shard["regex"] == canonical, f"shard plan shard {index} has a non-canonical regex")
        union.extend(tests)
    require(sorted(union) == names and len(union) == len(set(union)),
            "shard plan union does not exactly match discovery")
    return shards


def partition_document(discovery_path, plan_path, binary_sha256, binary_size_bytes, package, shard_count):
    discovery_raw = read_discovery(discovery_path)
    names = discovery_names(discovery_raw)
    try:
        with open(plan_path, "rb") as source:
            plan_raw = source.read()
    except OSError as error:
        fail(f"could not read shard plan {plan_path}: {error}")
    plan = load_json(plan_path)
    shards = validate_plan(plan, names, package, shard_count)
    require(HEX64.fullmatch(binary_sha256 or ""), "partition needs a 64-hex binary sha256")
    require(type(binary_size_bytes) is int and binary_size_bytes > 0, "partition needs a positive binary size")
    shard_timeouts = [shard_test_timeout_seconds(shard["estimatedDurationMillis"]) for shard in shards]
    oversized = [index for index, seconds in enumerate(shard_timeouts) if seconds > SHARD_TEST_CAP_SECONDS]
    if oversized:
        fail(f"shard {oversized[0]} needs {shard_timeouts[oversized[0]]}s of test budget, above the "
             f"{SHARD_TEST_CAP_SECONDS}s cap; repartition the package into more shards")
    runners = assign_runners(shard_timeouts)
    return {
        "schema_version": 1,
        "package": package,
        "normal_duration_multiplier": NORMAL_DURATION_MULTIPLIER,
        "shard_count": shard_count,
        "discovery_sha256": hashlib.sha256(discovery_raw).hexdigest(),
        "plan_sha256": hashlib.sha256(plan_raw).hexdigest(),
        "binary_sha256": binary_sha256,
        "binary_size_bytes": binary_size_bytes,
        "shards": [{"index": index, "test_timeout_seconds": shard_timeouts[index]}
                   for index in range(shard_count)],
        "runners": runners,
    }


def emit(document):
    json.dump(document, sys.stdout, sort_keys=True, separators=(",", ":"))
    sys.stdout.write("\n")


def cmd_lanes():
    config = load_config()
    lane = config["vmLane"]
    raced_total = lane["normalSeconds"] * NORMAL_DURATION_MULTIPLIER
    raced_slowest = lane["slowestTestNormalSeconds"] * NORMAL_DURATION_MULTIPLIER
    per_shard = max(math.ceil(raced_total / lane["shards"] * SHARD_TEST_HEADROOM),
                    math.ceil(raced_slowest * SHARD_TEST_HEADROOM),
                    SHARD_TEST_FLOOR_SECONDS)
    batch_seconds = per_shard + BATCH_RESERVE_SECONDS
    step_minutes, job_minutes = runner_minutes(batch_seconds)
    require(job_minutes <= JOB_TIMEOUT_CAP_MINUTES, "VM lane exceeds the job deadline cap")
    include = []
    for shard in range(lane["shards"]):
        include.append({
            "shard": shard,
            "testTimeoutSeconds": per_shard,
            "batchTimeoutSeconds": batch_seconds,
            "stepMinutes": step_minutes,
            "timeoutMinutes": job_minutes,
        })
    emit({"include": include})


def cmd_history(discovery_path):
    config = load_config()
    names = discovery_names(read_discovery(discovery_path))
    discovered = set(names)
    table = load_json(ROOT / config["apextestNormalHistory"])
    require(isinstance(table, dict) and type(table.get("version")) is int and table["version"] == 1,
            "Apex normal history has an invalid schema")
    require(table.get("package") == APEXTEST_PACKAGE, "Apex normal history has the wrong package")
    rows = table.get("tests")
    require(isinstance(rows, list), "Apex normal history has no test rows")
    scaled = []
    for row in rows:
        require(isinstance(row, dict) and set(row) == {"name", "durationMillis"},
                "Apex normal history has an invalid row")
        name, duration = row["name"], row["durationMillis"]
        require(isinstance(name, str) and TEST_NAME.fullmatch(name), "Apex normal history has an invalid name")
        require(type(duration) is int and duration >= 0, "Apex normal history has an invalid duration")
        if name in discovered:
            scaled.append({"name": name, "durationMillis": duration * NORMAL_DURATION_MULTIPLIER})
    if not scaled:
        fail("the Apex normal history has no entries for the discovered tests; "
             "refresh internal/apextest/testdata/shard_durations.json before planning race budgets")
    emit({"version": 1, "package": APEXTEST_PACKAGE, "complete": True, "tests": scaled})


def cmd_schedule(discovery_path, plan_path, binary_sha256, binary_size_text):
    try:
        binary_size = int(binary_size_text)
    except ValueError:
        fail("binary size must be an integer")
    emit(partition_document(discovery_path, plan_path, binary_sha256, binary_size,
                            APEXTEST_PACKAGE, APEXTEST_SHARDS))


def cmd_matrix(partition_path):
    partition = load_json(partition_path)
    require(isinstance(partition, dict) and set(partition) >= {"runners"}, "partition has no runners")
    include = []
    for runner in partition["runners"]:
        include.append({
            "runner": runner["runner"],
            "batchTimeoutSeconds": runner["batch_timeout_seconds"],
            "stepMinutes": runner["step_timeout_minutes"],
            "timeoutMinutes": runner["job_timeout_minutes"],
        })
    emit({"include": include})


def cmd_verify(partition_dir):
    partition_root = Path(partition_dir)
    partition_path = partition_root / "partition.json"
    partition = load_json(partition_path)
    require(isinstance(partition, dict) and set(partition) ==
            {"schema_version", "package", "normal_duration_multiplier", "shard_count",
             "discovery_sha256", "plan_sha256", "binary_sha256", "binary_size_bytes",
             "shards", "runners"},
            "partition has an invalid schema")
    require(type(partition["schema_version"]) is int and partition["schema_version"] == 1,
            "partition schema version must be 1")
    require(partition["package"] == APEXTEST_PACKAGE, "partition has the wrong package")
    require(type(partition["normal_duration_multiplier"]) is int and
            partition["normal_duration_multiplier"] == NORMAL_DURATION_MULTIPLIER,
            f"partition must record the documented multiplier {NORMAL_DURATION_MULTIPLIER}")
    shard_count = partition["shard_count"]
    require(type(shard_count) is int and shard_count == APEXTEST_SHARDS,
            f"partition must have exactly {APEXTEST_SHARDS} shards")
    require(HEX64.fullmatch(partition["discovery_sha256"] or "") and
            HEX64.fullmatch(partition["plan_sha256"] or "") and
            HEX64.fullmatch(partition["binary_sha256"] or ""),
            "partition digests must be 64-hex")
    require(type(partition["binary_size_bytes"]) is int and partition["binary_size_bytes"] > 0,
            "partition needs a positive binary size")
    recomputed = partition_document(partition_root / "discovery.txt", partition_root / "plan.json",
                                    partition["binary_sha256"], partition["binary_size_bytes"],
                                    APEXTEST_PACKAGE, APEXTEST_SHARDS)
    require(partition == recomputed, "partition does not match its own discovery, plan and policy")


def main():
    arguments = sys.argv[1:]
    if arguments == ["lanes"]:
        cmd_lanes()
    elif len(arguments) == 2 and arguments[0] == "history":
        cmd_history(arguments[1])
    elif len(arguments) == 5 and arguments[0] == "schedule":
        cmd_schedule(arguments[1], arguments[2], arguments[3], arguments[4])
    elif len(arguments) == 2 and arguments[0] == "matrix":
        cmd_matrix(arguments[1])
    elif len(arguments) == 2 and arguments[0] == "verify":
        cmd_verify(arguments[1])
    else:
        fail("usage: ci-race-plan.py lanes | history DISCOVERY | schedule DISCOVERY PLAN "
             "BINARY_SHA256 BINARY_SIZE | matrix PARTITION_JSON | verify PARTITION_DIR")


if __name__ == "__main__":
    main()
