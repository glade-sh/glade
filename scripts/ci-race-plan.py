#!/usr/bin/env python3
"""One race plan shared by the Race workflow, runner and aggregate.

The scheduled Race run 44158211531 timed out all three Apex batches and the
VM route: the Apex runners wrapped several shards in one flat one-hour shell
deadline while each shard's Go timeout was also one hour, and the VM package
ran unsharded behind the six-wide generic matrix. This module is the single
authority that replaces those duplicated, hard-coded budgets.

Policy, applied to every planned lane:

  raced estimate   = measured race duration from the checked-in race
                     history when the recorded run measured the test;
                     otherwise normal duration * NORMAL_DURATION_MULTIPLIER.
                     A test the run killed mid-flight is measured at its
                     elapsed time at the kill (a lower bound) and keeps the
                     multiplier's estimate wherever that is larger.
  shard budget     = ceil(raced estimate * SHARD_TEST_HEADROOM), with a floor
  isolated shard   = any single test whose own raced budget exceeds the
                     balanced per-shard budget runs in a shard of its own
                     with a budget derived from its own measurement, under
                     the shard cap
  shard count      = enough shards that the longest-processing-time bound
                      (package average + slowest test, both raced) fits the
                      budget under the cap, floored at APEXTEST_SHARD_FLOOR
  runner count     = the fewest count, from RUNNER_COUNT_MIN up to one
                      shard per runner (RUNNER_COUNT_MAX), whose balanced
                      longest-first assignment minimizes the slowest
                      runner's load: one more runner is kept only while
                      it strictly reduces that load, and the search stops
                      at the count where the slowest runner holds the
                      heaviest single shard alone
  batch deadline   = sum of the assigned shard budgets + BATCH_RESERVE_SECONDS,
                     or for one sequential lane the measured complete shard
                     walls scaled by the shard count with test headroom
  step deadline    = ceil(batch / 60) + STEP_RESERVE_MINUTES
  job deadline     = step deadline + JOB_RESERVE_MINUTES, capped well under
                      GitHub's 6-hour job limit

The multiplier is now a fallback, not the plan: it was derived from the one
measured scheduled observation (Apex shard 0, hash-partitioned, still
running at 3,416 s when its one-hour batch deadline expired, against about
745 s of recorded normal durations for its 141 table-known tests; charging
its 68 unknown tests the table median put that shard near 1,000 s normal, an
implied >= 3.4x, rounded to 4x), and it applies only to tests the recorded
race run never measured. The multiplier is a scheduling input only: budgets
derived from it surface an underestimated cost as a bounded, visible
timeout with partial evidence, never as silent under-coverage.

Subcommands:

  lanes                      emit the VM lane document (shard matrix,
                             isolated giant-test shards and budgets) for the
                              Race plan job
  early-lanes                emit the early lane matrix (sequential shards in
                              one job) with deadlines from measured walls
  independent-lanes          emit the independent lane matrix with a budget
                              from the measured warm normal wall
  generic-batches            emit batch deadlines for heavy generic lanes whose
                              measured budget exceeds the flat default
  history DISCOVERY          emit the raced Apex duration history (measured
                              first, multiplier fallback) for the planner
  vm-history DISCOVERY       emit the raced VM duration history so the VM
                              shards balance by measured cost, not names
  lane-history SLUG DISCOVERY
                              emit the raced history for a lane with measured
                              rows but no normal duration table
  browser-lanes PACKAGES_JSON
                              emit the browser lane matrices for the selected
                              dedicated browser packages, so a narrowed
                              dispatched run schedules only those legs
  shards DISCOVERY HISTORY   emit the Apex shard count that keeps every
                             shard budget under the cap
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
MODULE_PREFIX = "github.com/glade-sh/glade/"
APEXTEST_PACKAGE = MODULE_PREFIX + "internal/apextest"
VM_PACKAGE = MODULE_PREFIX + "internal/vm"
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
# The flat batch deadline heavy lanes get when no measured budget covers
# them; lanes whose measured budget stays under it keep the default.
DEFAULT_BATCH_SECONDS = 3600
# The Apex partition grows past its sixteen-shard floor when the suite's
# raced total no longer fits under the per-shard cap; beyond the ceiling the
# runner-count search fails loudly instead of planning unrunnable batches.
APEXTEST_SHARD_FLOOR = 16
APEXTEST_SHARD_CEILING = 96
# One id per scheduled runner; the count search below keeps at most one
# shard per runner (the RUNNER_COUNT_MAX ceiling), so the ids never run out.
RUNNER_IDS = "abcdefghijklmnop"
RUNNER_COUNT_MIN = 3
RUNNER_COUNT_MAX = 16
# Heavy generic lanes that run their shards sequentially in one job; the
# shard counts mirror the runner's planner_shards case values, which the
# scripts tests cross-check against this table.
GENERIC_LANE_PACKAGES = ("./internal/server", "./internal/testdaemon", "./internal/playground")
# Browser lanes have a fixed shard topology the shard aggregate already
# pins; the plan emits the runner and aggregate matrices from the selected
# dedicated packages so a narrowed dispatched run schedules only those legs.
# Together with the non-browser dedicated packages the topology covers the
# classifier's whole dedicated partition, and the scripts tests cross-check
# that coverage against the manifest, so an entry outside it fails the plan
# instead of silently skipping browser coverage.
BROWSER_LANE_SHARDS = {"./internal/visualforce": 4, "./internal/lwc/compile": 8}
NON_BROWSER_DEDICATED = ("./internal/apextest", "./internal/vm")
PACKAGE_PATH = re.compile(r"\./internal/[A-Za-z0-9._/-]+\Z")


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
            {"version", "apextestNormalHistory", "apextestNormalHistorySource",
             "vmNormalHistory", "vmNormalHistorySource", "raceHistory", "raceHistorySource",
             "vmLane", "earlyLane", "independentLane", "genericLanes"},
            "race plan config has an invalid schema")
    require(config["version"] == 1, "race plan config version must be 1")
    for field in ("apextestNormalHistory", "vmNormalHistory", "raceHistory"):
        require(isinstance(config[field], str) and config[field],
                f"race plan config needs the {field} path")
    lane = config["vmLane"]
    require(isinstance(lane, dict) and set(lane) == {"shards", "source"},
            "race plan VM lane has an invalid schema")
    require(type(lane["shards"]) is int and 1 <= lane["shards"] <= 16,
            "race plan VM lane needs between 1 and 16 shards")
    early = config["earlyLane"]
    require(isinstance(early, dict) and set(early) == {"package", "shards", "source"},
            "race plan early lane has an invalid schema")
    require(early["package"] == "./internal/gladecli",
            "race plan early lane must be the CLI package")
    require(type(early["shards"]) is int and 1 <= early["shards"] <= 16,
            "race plan early lane needs between 1 and 16 shards")
    independent = config["independentLane"]
    require(isinstance(independent, dict) and
            set(independent) == {"package", "normalSeconds", "normalSecondsSource"},
            "race plan independent lane has an invalid schema")
    require(independent["package"] == "./internal/repoguard",
            "race plan independent lane must be the repository guard package")
    require(type(independent["normalSeconds"]) is int and independent["normalSeconds"] > 0,
            "race plan independent lane needs a positive measured normal wall")
    generic = config["genericLanes"]
    require(isinstance(generic, dict) and set(generic) == set(GENERIC_LANE_PACKAGES),
            "race plan generic lanes must list exactly the heavy generic packages")
    for package, lane in generic.items():
        require(isinstance(lane, dict) and set(lane) == {"shards", "source"},
                f"race plan generic lane {package} has an invalid schema")
        require(type(lane["shards"]) is int and 1 <= lane["shards"] <= 16,
                f"race plan generic lane {package} needs between 1 and 16 shards")
    return config


def generic_lane_shard_counts():
    config = load_config()
    return {package: lane["shards"] for package, lane in config["genericLanes"].items()}


def load_race_history():
    """Read the checked-in measured race history.

    Returns {package: {"tests": {name: {"durationMillis", "killed"}},
    "completeWalls": [seconds, ...]}}. Rows are measurements from one named
    run; a killed row is the test's elapsed time at the kill, a lower bound.
    """
    config = load_config()
    document = load_json(ROOT / config["raceHistory"])
    require(isinstance(document, dict) and set(document) ==
            {"schema_version", "source_run_id", "source_run_url", "packages"},
            "race history has an invalid schema")
    require(type(document["schema_version"]) is int and document["schema_version"] == 1,
            "race history has an invalid schema version")
    require(type(document["source_run_id"]) is int and document["source_run_id"] > 0,
            "race history needs its source run id")
    require(isinstance(document["source_run_url"], str) and document["source_run_url"],
            "race history needs its source run URL")
    packages = {}
    for entry in document["packages"]:
        require(isinstance(entry, dict) and set(entry) == {"package", "tests", "shardWalls"},
                "race history package has an invalid schema")
        package = entry["package"]
        require(isinstance(package, str) and package.startswith("github.com/glade-sh/glade/"),
                "race history package has an invalid name")
        require(package not in packages, f"race history lists {package} twice")
        tests = {}
        for row in entry["tests"]:
            require(isinstance(row, dict) and set(row) == {"name", "durationMillis", "killed"},
                    f"race history row for {package} has an invalid schema")
            name, duration, killed = row["name"], row["durationMillis"], row["killed"]
            require(isinstance(name, str) and TEST_NAME.fullmatch(name),
                    f"race history row for {package} has an invalid name")
            require(type(duration) is int and duration >= 1,
                    f"race history row for {package} {name} has an invalid duration")
            require(type(killed) is bool, f"race history row for {package} {name} has an invalid killed flag")
            require(name not in tests, f"race history lists {package} {name} twice")
            tests[name] = {"durationMillis": duration, "killed": killed}
        walls = []
        for wall in entry["shardWalls"]:
            require(isinstance(wall, dict) and set(wall) == {"elapsedSeconds", "exitStatus", "complete"},
                    f"race history wall for {package} has an invalid schema")
            elapsed, status = wall["elapsedSeconds"], wall["exitStatus"]
            require(isinstance(elapsed, (int, float)) and not isinstance(elapsed, bool) and
                    math.isfinite(elapsed) and elapsed >= 0,
                    f"race history wall for {package} has an invalid elapsed time")
            require(type(status) is int, f"race history wall for {package} has an invalid exit status")
            if wall["complete"]:
                walls.append(elapsed)
        packages[package] = {"tests": tests, "completeWalls": walls}
    return packages


def load_normal_table(table_path, package):
    """Read one checked-in normal duration table as {name: millis}."""
    table = load_json(ROOT / table_path)
    require(isinstance(table, dict) and type(table.get("version")) is int and table["version"] == 1,
            f"{table_path} has an invalid schema")
    require(table.get("package") == package, f"{table_path} has the wrong package")
    rows = table.get("tests")
    require(isinstance(rows, list), f"{table_path} has no test rows")
    normal = {}
    for row in rows:
        require(isinstance(row, dict) and set(row) == {"name", "durationMillis"},
                f"{table_path} has an invalid row")
        name, duration = row["name"], row["durationMillis"]
        require(isinstance(name, str) and TEST_NAME.fullmatch(name), f"{table_path} has an invalid name")
        require(type(duration) is int and duration >= 0, f"{table_path} has an invalid duration")
        require(name not in normal, f"{table_path} lists {name} twice")
        normal[name] = duration
    return normal


def raced_estimate(name, race_rows, normal_rows):
    """One test's raced estimate: measured first, multiplier as fallback.

    A killed row is the test's elapsed time at the kill, a lower bound, so
    it also keeps the multiplier's estimate wherever that is larger.
    Returns (estimate_millis, known): known is false only when the
    recorded run never measured the test and the table holds no row for
    it, which the raced history leaves to the planner's own median
    estimate; a table row at zero stays a known row the planner can weigh.
    """
    measured = race_rows.get(name)
    if measured is not None and not measured["killed"]:
        return measured["durationMillis"], True
    scaled = normal_rows.get(name, 0) * NORMAL_DURATION_MULTIPLIER
    if measured is not None:
        return max(measured["durationMillis"], scaled), True
    if name in normal_rows:
        return scaled, True
    return 0, False


def raced_history_document(discovery_path, package, table_path):
    """Emit one package's raced durations for the shard planner.

    The measured race history wins for every test the recorded run
    measured; tests it never measured fall back to the normal table scaled
    by the multiplier; tests known to neither keep the planner's own
    median estimate.
    """
    names = discovery_names(read_discovery(discovery_path))
    discovered = set(names)
    history = load_race_history()
    race_rows = history.get(package, {}).get("tests", {})
    normal_rows = load_normal_table(table_path, package) if table_path else {}
    raced = []
    for name in names:
        estimate, known = raced_estimate(name, race_rows, normal_rows)
        if known:
            raced.append({"name": name, "durationMillis": estimate})
    if not raced:
        fail(f"the {package} raced history has no entries for the discovered tests; "
             f"refresh {table_path or 'the race history'} before planning race budgets")
    return {"version": 1, "package": package, "complete": True, "tests": raced}


def shard_test_timeout_seconds(estimate_ms):
    """Budget one shard from its race-scaled estimate: headroom once.

    The history subcommands already apply NORMAL_DURATION_MULTIPLIER, so the
    plan's estimates arrive raced; multiplying here again would charge the
    suite sixteen times its normal cost and no shard count could fit.
    """
    return max(math.ceil(estimate_ms / 1000 * SHARD_TEST_HEADROOM), SHARD_TEST_FLOOR_SECONDS)


def runner_minutes(batch_seconds):
    step_minutes = math.ceil(batch_seconds / 60) + STEP_RESERVE_MINUTES
    return step_minutes, step_minutes + JOB_RESERVE_MINUTES


def assign_runners(shard_timeouts):
    """Bind shards to the runner count that minimizes the slowest job.

    Hosted runners cost the nightly run nothing extra while every
    verification dispatch waits on its slowest job, so the count search
    balances the shards longest-first over each candidate count from the
    floor upward and keeps the fewest count whose slowest runner holds the
    least load: one more runner is kept only while it strictly reduces that
    load, and the search stops at the count where the slowest runner holds
    the heaviest single shard alone, which no further count below the
    one-shard-per-runner ceiling can reduce. Every deadline still derives
    from the assigned shard budgets alone.
    """
    ordered = sorted(range(len(shard_timeouts)), key=lambda index: (-shard_timeouts[index], index))
    heaviest = max(shard_timeouts)
    ceiling = min(RUNNER_COUNT_MAX, len(shard_timeouts))
    best_slowest = None
    best_assignment = None
    for count in range(min(RUNNER_COUNT_MIN, ceiling), ceiling + 1):
        loads = [0] * count
        assignment = [[] for _ in range(count)]
        for index in ordered:
            target = min(range(count), key=lambda runner: (loads[runner], runner))
            assignment[target].append(index)
            loads[target] += shard_timeouts[index]
        slowest = max(loads)
        if best_slowest is None or slowest < best_slowest:
            best_slowest, best_assignment = slowest, assignment
        if slowest <= heaviest:
            break
    runners = []
    for runner in range(len(best_assignment)):
        batch_seconds = sum(shard_timeouts[index] for index in best_assignment[runner]) + BATCH_RESERVE_SECONDS
        step_minutes, job_minutes = runner_minutes(batch_seconds)
        if job_minutes > JOB_TIMEOUT_CAP_MINUTES:
            fail("race plan cannot fit the Apex shards under the job deadline cap with "
                 f"up to {RUNNER_COUNT_MAX} runners; the suite outgrew the current lanes")
        runners.append({
            "runner": RUNNER_IDS[runner],
            "shard_indexes": sorted(best_assignment[runner]),
            "batch_timeout_seconds": batch_seconds,
            "step_timeout_minutes": step_minutes,
            "job_timeout_minutes": job_minutes,
        })
    return runners


def validate_plan(plan, names, package):
    require(isinstance(plan, dict) and set(plan) == {"version", "package", "historyUsed", "shards"},
            "shard plan has an invalid schema")
    require(type(plan["version"]) is int and plan["version"] == 1, "shard plan version must be 1")
    require(plan["package"] == package, "shard plan has the wrong package")
    require(type(plan["historyUsed"]) is bool, "shard plan historyUsed must be boolean")
    shards = plan["shards"]
    require(isinstance(shards, list) and shards,
            "shard plan has no shards")
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


def partition_document(discovery_path, plan_path, binary_sha256, binary_size_bytes):
    discovery_raw = read_discovery(discovery_path)
    names = discovery_names(discovery_raw)
    try:
        with open(plan_path, "rb") as source:
            plan_raw = source.read()
    except OSError as error:
        fail(f"could not read shard plan {plan_path}: {error}")
    plan = load_json(plan_path)
    shards = validate_plan(plan, names, APEXTEST_PACKAGE)
    shard_count = len(shards)
    require(APEXTEST_SHARD_FLOOR <= shard_count <= APEXTEST_SHARD_CEILING,
            f"shard plan must stay between {APEXTEST_SHARD_FLOOR} and {APEXTEST_SHARD_CEILING} shards, "
            f"got {shard_count}")
    require(shard_count <= len(names), "shard plan has more shards than discovered tests")
    require(HEX64.fullmatch(binary_sha256 or ""), "partition needs a 64-hex binary sha256")
    require(type(binary_size_bytes) is int and binary_size_bytes > 0, "partition needs a positive binary size")
    shard_timeouts = [shard_test_timeout_seconds(shard["estimatedDurationMillis"]) for shard in shards]
    oversized = [index for index, seconds in enumerate(shard_timeouts) if seconds > SHARD_TEST_CAP_SECONDS]
    if oversized:
        index = oversized[0]
        tests = shards[index]["tests"]
        if len(tests) == 1:
            fail(f"test {tests[0]} alone needs {shard_timeouts[index]}s of test budget, above the "
                 f"{SHARD_TEST_CAP_SECONDS}s cap; a single test cannot be sharded")
        fail(f"shard {index} needs {shard_timeouts[index]}s of test budget, above the "
             f"{SHARD_TEST_CAP_SECONDS}s cap; repartition the package into more shards")
    runners = assign_runners(shard_timeouts)
    return {
        "schema_version": 1,
        "package": APEXTEST_PACKAGE,
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


def shard_budget_seconds(estimate_ms):
    """One shard's test budget from its raced estimate: headroom once."""
    return max(math.ceil(estimate_ms / 1000 * SHARD_TEST_HEADROOM), SHARD_TEST_FLOOR_SECONDS)


def lane_matrix_entry(shard, test_budget_seconds):
    batch_seconds = test_budget_seconds + BATCH_RESERVE_SECONDS
    step_minutes, job_minutes = runner_minutes(batch_seconds)
    return {
        "shard": shard,
        "testTimeoutSeconds": test_budget_seconds,
        "batchTimeoutSeconds": batch_seconds,
        "stepMinutes": step_minutes,
        "timeoutMinutes": job_minutes,
    }


def cmd_lanes():
    """Emit the VM lane: balanced shards plus isolated giant-test shards.

    The plan-time universe is every test the recorded run measured plus
    every row of the normal table; discovery at run time can only add tests
    the planner estimates at its median. Any single test whose own raced
    budget exceeds the balanced per-shard budget gets a shard of its own
    with a budget derived from its own measurement, under the shard cap.
    """
    config = load_config()
    package = VM_PACKAGE
    history = load_race_history()
    race_rows = history.get(package, {}).get("tests", {})
    normal_rows = load_normal_table(config["vmNormalHistory"], package)
    universe = {}
    for name in set(race_rows) | set(normal_rows):
        universe[name] = raced_estimate(name, race_rows, normal_rows)[0]
    balanced_count = config["vmLane"]["shards"]
    total_ms = sum(universe.values())
    # A test is a giant when its own budget exceeds the balanced per-shard
    # budget computed without the giants; solving that by iteration keeps
    # the definition independent of evaluation order.
    giants = {name for name in universe if shard_budget_seconds(universe[name]) > SHARD_TEST_CAP_SECONDS}
    for _ in range(len(universe) + 1):
        balanced_budget = max(
            math.ceil((total_ms - sum(universe[name] for name in giants)) / balanced_count / 1000
                      * SHARD_TEST_HEADROOM),
            SHARD_TEST_FLOOR_SECONDS)
        detected = {name for name, estimate in universe.items()
                    if shard_budget_seconds(estimate) > balanced_budget}
        if detected == giants:
            break
        giants = detected
    else:
        fail("the VM lane giant-test derivation did not converge")
    if len(giants) >= balanced_count:
        fail(f"the VM lane has {len(giants)} isolated giant tests for {balanced_count} balanced shards; "
             f"raise the VM lane shard count in scripts/ci-race-plan.json")
    if balanced_budget > SHARD_TEST_CAP_SECONDS:
        fail(f"the balanced VM shard budget {balanced_budget}s exceeds the {SHARD_TEST_CAP_SECONDS}s "
             f"cap; raise the VM lane shard count in scripts/ci-race-plan.json")
    for name in sorted(giants):
        budget = shard_budget_seconds(universe[name])
        if budget > SHARD_TEST_CAP_SECONDS:
            fail(f"test {name} alone needs {budget}s of test budget, above the "
                 f"{SHARD_TEST_CAP_SECONDS}s cap; a single test cannot be sharded")
    include = [lane_matrix_entry(shard, balanced_budget) for shard in range(balanced_count)]
    for offset, name in enumerate(sorted(giants)):
        include.append(lane_matrix_entry(balanced_count + offset, shard_budget_seconds(universe[name])))
    for entry in include:
        require(entry["timeoutMinutes"] <= JOB_TIMEOUT_CAP_MINUTES,
                f"the VM lane shard budget {entry['testTimeoutSeconds']}s exceeds the job deadline cap")
    emit({
        "shardCount": len(include),
        "isolatedTests": sorted(giants),
        "include": include,
    })


def sequential_lane_batch(package, shard_count, history):
    """One sequential lane's batch deadline from its measured shard walls.

    Per-test sums overstate a sequential lane's wall several times over when
    subtests run in parallel, so the batch comes from the walls of complete
    shards: their average scaled by the shard count with test headroom.
    """
    walls = history.get(MODULE_PREFIX + package[2:], {}).get("completeWalls", [])
    if not walls:
        fail(f"the race history holds no complete shard wall for {package}; "
             f"refresh scripts/ci-race-history.json before planning its batch")
    average = sum(walls) / len(walls)
    return math.ceil(average * shard_count * SHARD_TEST_HEADROOM) + BATCH_RESERVE_SECONDS


def cmd_early_lanes():
    """Emit the early lane matrix: one job runs every shard sequentially."""
    config = load_config()
    early = config["earlyLane"]
    package = early["package"]
    batch_seconds = sequential_lane_batch(package, early["shards"], load_race_history())
    step_minutes, job_minutes = runner_minutes(batch_seconds)
    require(job_minutes <= JOB_TIMEOUT_CAP_MINUTES,
            f"the early lane batch {batch_seconds}s exceeds the job deadline cap; "
            f"the lane's measured walls outgrew one sequential job")
    emit({"include": [{
        "package": package,
        "batchTimeoutSeconds": batch_seconds,
        "stepMinutes": step_minutes,
        "timeoutMinutes": job_minutes,
    }]})


def cmd_independent_lanes():
    """Emit the independent lane matrix from the measured warm normal wall.

    The repository guard lane has no race measurement: every recorded race
    run died on its cold scan cache (the lane restored no Go build cache, so
    the per-blob repository scan rebuilt from nothing and exceeded the hour).
    The race workflow restores the cache family the CI lanes persist, so the
    scan arrives warm, and the raced budget is the multiplier on the lane's
    measured warm normal wall with the shard floor covering the day's
    changed blobs.
    """
    config = load_config()
    lane = config["independentLane"]
    package = lane["package"]
    raced_seconds = lane["normalSeconds"] * NORMAL_DURATION_MULTIPLIER
    test_budget = max(math.ceil(raced_seconds * SHARD_TEST_HEADROOM), SHARD_TEST_FLOOR_SECONDS)
    step_minutes, job_minutes = runner_minutes(test_budget)
    require(job_minutes <= JOB_TIMEOUT_CAP_MINUTES, "the independent lane exceeds the job deadline cap")
    emit({"include": [{
        "package": package,
        "testTimeoutSeconds": test_budget,
        "stepMinutes": step_minutes,
        "timeoutMinutes": job_minutes,
    }]})


def cmd_generic_batches():
    """Emit batch deadlines for heavy generic lanes past the flat default.

    A heavy generic lane runs its shards sequentially in one job behind the
    flat default batch; lanes whose measured budget exceeds that default get
    their own deadline, and lanes under it keep the default.
    """
    config = load_config()
    history = load_race_history()
    runner_shards = generic_lane_shard_counts()
    batches = {}
    for package in GENERIC_LANE_PACKAGES:
        batch_seconds = sequential_lane_batch(package, runner_shards[package], history)
        if batch_seconds > DEFAULT_BATCH_SECONDS:
            batches[package] = batch_seconds
    emit(batches)


def minimum_apextest_shards(discovery_names, scaled_rows):
    """Derive the Apex shard count that keeps every shard budget under the cap.

    The planner assigns the slowest tests first onto the least-loaded shard,
    so no shard estimate can exceed the package average plus the slowest test
    (the longest-processing-time bound). The smallest count whose bound fits
    the budget under the cap wins; a single test over the cap fails with its
    name because no count can schedule around it. Discovered tests missing
    from the history weigh the history median, mirroring the planner.
    """
    known = {}
    discovered = set(discovery_names)
    for row in scaled_rows:
        name, duration = row["name"], row["durationMillis"]
        require(TEST_NAME.fullmatch(name), "the scaled history has an invalid name")
        require(type(duration) is int and duration >= 0, "the scaled history has an invalid duration")
        require(name not in known, f"the scaled history has a duplicate entry for {name}")
        require(name in discovered, f"the scaled history has a stale entry for {name}")
        known[name] = duration
    adjusted = sorted(max(duration, 1) for duration in known.values())
    estimate = max(1, adjusted[(len(adjusted) - 1) // 2]) if adjusted else 1
    weights = {}
    for name in discovery_names:
        if name in known:
            weights[name] = known[name] if known[name] > 0 else 1
        else:
            weights[name] = estimate
    for name, weight in sorted(weights.items()):
        if shard_test_timeout_seconds(weight) > SHARD_TEST_CAP_SECONDS:
            fail(f"test {name} alone needs {shard_test_timeout_seconds(weight)}s of test budget, above the "
                 f"{SHARD_TEST_CAP_SECONDS}s cap; a single test cannot be sharded")
    total_ms = sum(weights.values())
    slowest_ms = max(weights.values())
    for count in range(APEXTEST_SHARD_FLOOR, APEXTEST_SHARD_CEILING + 1):
        if count > len(discovery_names):
            break
        # Longest-processing-time bound: no shard exceeds the package average
        # plus the slowest test, so its budget bounds every greedy shard.
        bound_ms = total_ms / count + slowest_ms
        if shard_test_timeout_seconds(bound_ms) <= SHARD_TEST_CAP_SECONDS:
            return count
    fail(f"the Apex race suite outgrew the current lanes: no shard count up to {APEXTEST_SHARD_CEILING} "
         f"keeps a shard budget under the {SHARD_TEST_CAP_SECONDS}s cap")


def cmd_history(discovery_path):
    config = load_config()
    emit(raced_history_document(discovery_path, APEXTEST_PACKAGE, config["apextestNormalHistory"]))


def cmd_vm_history(discovery_path):
    config = load_config()
    emit(raced_history_document(discovery_path, VM_PACKAGE, config["vmNormalHistory"]))


def cmd_lane_history(slug, discovery_path):
    """Emit the raced history for a lane with measured rows but no table.

    Packages without a checked-in normal duration table plan from the
    measured race rows alone; the planner estimates tests the run never
    measured at the measured median.
    """
    package = MODULE_PREFIX + slug.replace("-", "/")
    names = discovery_names(read_discovery(discovery_path))
    history = load_race_history()
    race_rows = history.get(package, {}).get("tests", {})
    raced = []
    for name in names:
        measured = race_rows.get(name)
        if measured is None:
            continue
        raced.append({"name": name, "durationMillis": measured["durationMillis"]})
    if not raced:
        fail(f"the race history holds no measured rows for {package}; "
             f"refresh scripts/ci-race-history.json before planning this lane")
    emit({"version": 1, "package": package, "complete": True, "tests": raced})


def cmd_browser_lanes(packages_path):
    """Emit the browser lane matrices for the selected dedicated packages.

    The input is the plan's dedicated package list, already validated
    against the manifest by the classifier: browser packages select their
    legs, non-browser dedicated packages select none, and an entry outside
    the dedicated partition fails the plan instead of silently dropping
    browser coverage. The shard topology mirrors the shard aggregate
    (scripts/ci-race-shard-aggregate.sh), which the scripts tests
    cross-check against this table.
    """
    packages = load_json(packages_path)
    require(isinstance(packages, list), "browser lane input must be a package list")
    selected = set()
    for package in packages:
        require(isinstance(package, str) and PACKAGE_PATH.fullmatch(package),
                f"browser lane input has an invalid package {package!r}")
        require(package not in selected, f"browser lane input lists {package} twice")
        require(package in BROWSER_LANE_SHARDS or package in NON_BROWSER_DEDICATED,
                f"browser lane input holds {package}, which is not a dedicated race package")
        selected.add(package)
    runners, aggregate = [], []
    for package, shard_count in BROWSER_LANE_SHARDS.items():
        if package not in selected:
            continue
        slug = package[2:].replace("/", "-")
        for shard in range(shard_count):
            runners.append({"package": package, "slug": slug, "shard": shard})
        aggregate.append({"package": package, "slug": slug, "shards": shard_count})
    emit({"runners": {"include": runners}, "aggregate": {"include": aggregate}})


def cmd_shards(discovery_path, history_path):
    names = discovery_names(read_discovery(discovery_path))
    history = load_json(history_path)
    require(isinstance(history, dict) and set(history) == {"version", "package", "complete", "tests"},
            "scaled history has an invalid schema")
    require(type(history["version"]) is int and history["version"] == 1 and history["complete"] is True,
            "scaled history has an invalid version or completeness")
    require(history["package"] == APEXTEST_PACKAGE, "scaled history has the wrong package")
    rows = history["tests"]
    require(isinstance(rows, list) and rows, "scaled history has no test rows")
    emit({"shard_count": minimum_apextest_shards(names, rows)})


def cmd_schedule(discovery_path, plan_path, binary_sha256, binary_size_text):
    try:
        binary_size = int(binary_size_text)
    except ValueError:
        fail("binary size must be an integer")
    emit(partition_document(discovery_path, plan_path, binary_sha256, binary_size))


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
    require(type(shard_count) is int and APEXTEST_SHARD_FLOOR <= shard_count <= APEXTEST_SHARD_CEILING,
            f"partition must stay between {APEXTEST_SHARD_FLOOR} and {APEXTEST_SHARD_CEILING} shards")
    require(HEX64.fullmatch(partition["discovery_sha256"] or "") and
            HEX64.fullmatch(partition["plan_sha256"] or "") and
            HEX64.fullmatch(partition["binary_sha256"] or ""),
            "partition digests must be 64-hex")
    require(type(partition["binary_size_bytes"]) is int and partition["binary_size_bytes"] > 0,
            "partition needs a positive binary size")
    recomputed = partition_document(partition_root / "discovery.txt", partition_root / "plan.json",
                                    partition["binary_sha256"], partition["binary_size_bytes"])
    require(partition == recomputed, "partition does not match its own discovery, plan and policy")


def main():
    arguments = sys.argv[1:]
    if arguments == ["lanes"]:
        cmd_lanes()
    elif arguments == ["early-lanes"]:
        cmd_early_lanes()
    elif arguments == ["independent-lanes"]:
        cmd_independent_lanes()
    elif arguments == ["generic-batches"]:
        cmd_generic_batches()
    elif len(arguments) == 2 and arguments[0] == "history":
        cmd_history(arguments[1])
    elif len(arguments) == 2 and arguments[0] == "vm-history":
        cmd_vm_history(arguments[1])
    elif len(arguments) == 3 and arguments[0] == "lane-history":
        cmd_lane_history(arguments[1], arguments[2])
    elif len(arguments) == 2 and arguments[0] == "browser-lanes":
        cmd_browser_lanes(arguments[1])
    elif len(arguments) == 3 and arguments[0] == "shards":
        cmd_shards(arguments[1], arguments[2])
    elif len(arguments) == 5 and arguments[0] == "schedule":
        cmd_schedule(arguments[1], arguments[2], arguments[3], arguments[4])
    elif len(arguments) == 2 and arguments[0] == "matrix":
        cmd_matrix(arguments[1])
    elif len(arguments) == 2 and arguments[0] == "verify":
        cmd_verify(arguments[1])
    else:
        fail("usage: ci-race-plan.py lanes | early-lanes | independent-lanes | generic-batches | "
             "history DISCOVERY | vm-history DISCOVERY | lane-history SLUG DISCOVERY | "
             "browser-lanes PACKAGES_JSON | "
             "shards DISCOVERY HISTORY | schedule DISCOVERY PLAN BINARY_SHA256 BINARY_SIZE | "
             "matrix PARTITION_JSON | verify PARTITION_DIR")


if __name__ == "__main__":
    main()
