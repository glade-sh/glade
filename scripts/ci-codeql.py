#!/usr/bin/env python3
"""Bound CodeQL analysis; only an expired wall-clock deadline becomes a warning."""

import argparse
import datetime
import json
import math
import os
from pathlib import Path
import shlex
import signal
import subprocess
import sys
import time

# Total evaluator RAM in MB for run-queries. Without it the CLI's default Java
# heap (about 1 GiB) ran out on hosted runners. GitHub's ubuntu-latest runner
# has 16 GB; this leaves about half for the OS, the runner and other processes.
RUN_QUERIES_RAM_MB = 8192


def action_output(completed):
    if path := os.environ.get("GITHUB_OUTPUT"):
        with open(path, "a") as output:
            output.write(f"completed={str(completed).lower()}\n")


def action_env(**values):
    if path := os.environ.get("GITHUB_ENV"):
        with open(path, "a") as output:
            for key, value in values.items():
                output.write(f"{key}={value}\n")


def diff_pack(profile):
    # Pinned init persists these ranges before the analysis step. Keep the
    # same data-extension protocol as its analyze action, without API calls.
    temp = os.environ.get("CODEQL_ACTION_TEMP") or os.environ.get("RUNNER_TEMP")
    if profile != "fast" or not temp:
        return None
    ranges_file = Path(temp, "pr-diff-range.json")
    if not ranges_file.exists():
        return None
    ranges = json.loads(ranges_file.read_text())
    if not isinstance(ranges, list):
        raise ValueError("invalid PR diff ranges")
    data = []
    for row in ranges:
        path, start, end = row["path"], row["startLine"], row["endLine"]
        if not isinstance(path, str) or Path(path).is_absolute() or ".." in Path(path).parts:
            raise ValueError("invalid PR diff path")
        if type(start) is not int or type(end) is not int or start < 0 or end < start:
            raise ValueError("invalid PR diff lines")
        data.append([(Path(os.environ["GITHUB_WORKSPACE"]) / path).as_posix(), start, end])
    pack = Path(temp, "pr-diff-range")
    pack.mkdir(parents=True, exist_ok=True)
    # JSON is also valid YAML; it safely quotes paths containing punctuation.
    manifest = {"name": "codeql-action/pr-diff-range", "version": "0.0.0", "library": True,
                "extensionTargets": {"codeql/util": "*"}, "dataExtensions": ["diff-ranges.yml"]}
    extension = {"extensions": [{"addsTo": {"pack": "codeql/util", "extensible": "restrictAlertsTo", "checkPresence": False},
                                 "data": data or [["", 0, 0]]}]}
    (pack / "qlpack.yml").write_text(json.dumps(manifest))
    (pack / "diff-ranges.yml").write_text(json.dumps(extension))
    return pack


def run_phase(command, seconds):
    stamp = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
    print(f"{stamp} [command]{shlex.join(command)}", flush=True)
    if seconds <= 0:
        raise subprocess.TimeoutExpired(command, seconds)
    # Kill the evaluator's descendants too, before allowing another phase/job.
    process = subprocess.Popen(command, start_new_session=True)
    try:
        status = process.wait(timeout=seconds)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait()
        raise
    if status:
        raise subprocess.CalledProcessError(status, command)


def analyze(args):
    limits = json.loads(Path(args.budget).read_text())
    total, queries = limits["analyze_seconds"], limits["query_seconds"]
    if not all(isinstance(n, (int, float)) and math.isfinite(n) and n > 0 for n in (total, queries)):
        raise ValueError("analysis budgets must be finite and positive")
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.unlink(missing_ok=True)
    action_output(False)
    deadline = time.monotonic() + total
    pack = diff_pack(args.profile)
    query_options = [f"--additional-packs={pack}", "--extension-packs=codeql-action/pr-diff-range"] if pack else []
    sarif_options = ["--sarif-run-property=incrementalMode=diff-informed"] if pack else []
    phases = [
        ("finalize", ["--threads=0"]),
        ("run-queries", ["--threads=0", f"--ram={RUN_QUERIES_RAM_MB}", "--verbose", *query_options]),
        ("interpret-results", ["--threads=0", "--format=sarifv2.1.0", f"--output={output}", "--sarif-category=codeql-go", *sarif_options]),
    ]
    try:
        for phase, options in phases:
            remaining = deadline - time.monotonic()
            seconds = min(remaining, queries) if phase == "run-queries" else remaining
            # No explicit suite: use the configuration recorded by the pinned
            # init action in the database, including its query filters.
            run_phase([args.codeql, "database", phase, *options, args.database], seconds)
    except subprocess.TimeoutExpired:
        output.unlink(missing_ok=True)
        message = f"CodeQL {phase} exceeded its wall-clock budget; analysis is incomplete. Review logs and rerun."
        print(f"::warning::{message}", flush=True)
        action_env(CODEQL_ACTION_SKIP_SARIF_UPLOAD="true", CODEQL_ACTION_JOB_STATUS="JOB_STATUS_UNKNOWN")
        if path := os.environ.get("GITHUB_STEP_SUMMARY"):
            with open(path, "a") as summary:
                summary.write(f"**Warning:** {message}\n")
        return
    # Validate before permitting upload; findings are checked after upload.
    read_results(output)
    action_output(True)


def read_results(path):
    sarif = json.loads(Path(path).read_text())
    if sarif.get("version") != "2.1.0" or not isinstance(sarif.get("runs"), list) or not sarif["runs"]:
        raise ValueError("missing SARIF runs")
    results = []
    for run in sarif["runs"]:
        if not isinstance(run.get("results"), list) or not run.get("tool", {}).get("driver", {}).get("name"):
            raise ValueError("incomplete SARIF run")
        for result in run["results"]:
            if not isinstance(result, dict) or "message" not in result:
                raise ValueError("invalid SARIF result")
            results.append(result)
    return results


def check(path):
    alerts = [r for r in read_results(path) if r.get("kind", "fail") == "fail"
              and not any(s.get("status", "accepted") == "accepted" for s in r.get("suppressions", []))]
    if alerts:
        raise ValueError(f"CodeQL reported {len(alerts)} unsuppressed alert(s); inspect uploaded code scanning results")
    print("CodeQL completed with no unsuppressed alerts")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    scan = commands.add_parser("analyze")
    for name in ("codeql", "database", "output"):
        scan.add_argument(f"--{name}", required=True)
    scan.add_argument("--budget", default=".github/codeql-budget.json")
    scan.add_argument("--profile", choices=("fast", "full"), default="full")
    commands.add_parser("check").add_argument("sarif")
    args = parser.parse_args()
    try:
        analyze(args) if args.command == "analyze" else check(args.sarif)
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as error:
        action_env(CODEQL_ACTION_JOB_STATUS="JOB_STATUS_FAILURE")
        print(f"::error::CodeQL: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
