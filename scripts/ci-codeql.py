#!/usr/bin/env python3
"""Bound CodeQL analysis; only an expired wall-clock deadline becomes a warning."""

import argparse
import datetime
import json
import math
import os
from pathlib import Path
import re
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


def local_diff_ranges(deadline=None):
    """Complete PR ranges in the analyzed tree, without the compare API cap.

    Uncertain provenance is an error, never an empty diff. No network command or
    repository mutation is performed here; checkout must provide full history.
    """
    workspace = Path(os.environ["GITHUB_WORKSPACE"])
    deadline = min(deadline if deadline is not None else float("inf"), time.monotonic() + 60)

    def git(*args):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise subprocess.TimeoutExpired("complete local CodeQL diff", 60)
        return subprocess.check_output(
            ["git", "--literal-pathspecs", "-C", str(workspace), *args],
            stderr=subprocess.PIPE, timeout=remaining,
        )

    def commit(value):
        if not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", value):
            raise ValueError("invalid pinned PR commit")
        resolved = git("rev-parse", "--verify", value + "^{commit}").decode().strip()
        if resolved != value:
            raise ValueError("PR commit did not resolve exactly")
        return value

    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    pr = event["pull_request"]
    base, head = commit(pr["base"]["sha"]), commit(pr["head"]["sha"])
    analyzed = commit(os.environ["GITHUB_SHA"])
    if git("rev-parse", "HEAD").decode().strip() != analyzed:
        raise ValueError("CodeQL checkout does not match GITHUB_SHA")
    if git("rev-parse", "--is-shallow-repository").strip() != b"false":
        raise ValueError("local CodeQL diff requires complete checkout history")
    if analyzed != head:
        parents = git("show", "-s", "--format=%P", analyzed).decode().split()
        if parents != [base, head]:
            raise ValueError("CodeQL tree is neither the PR head nor its pinned merge")
    ancestors = git("merge-base", "--all", base, head).decode().split()
    if len(ancestors) != 1:
        raise ValueError("CodeQL PR diff requires exactly one merge base")
    ancestor = commit(ancestors[0])
    # Ranges must describe the program extracted from this checkout, not edits
    # made since checkout. Build output belongs outside the source tree.
    if git("diff", "--no-ext-diff", "--ignore-submodules=none", "--name-only", "-z", "HEAD", "--") or git("diff", "--cached", "--no-ext-diff", "--ignore-submodules=none", "--name-only", "-z", "HEAD", "--"):
        raise ValueError("tracked source changed since the analyzed commit")
    if any(p.endswith(b".go") for p in git("ls-files", "--others", "-z").split(b"\0")):
        raise ValueError("untracked Go source makes local diff provenance uncertain")

    # Raw NUL-separated paths avoid quoting, newline, rename and pathspec traps.
    # Disable renames: the destination becomes an addition, conservatively
    # covering every destination line; the old path has no nodes in this tree.
    raw = git("diff", "--raw", "--no-abbrev", "-z", "--no-renames", "--no-ext-diff",
              "--no-relative", "--ignore-submodules=none", ancestor, analyzed, "--")
    parts = raw.split(b"\0")
    if parts[-1] != b"" or (len(parts) - 1) % 2:
        raise ValueError("invalid complete Git change manifest")
    ranges = []
    for index in range(0, len(parts) - 1, 2):
        header, path = parts[index].decode("ascii"), parts[index + 1].decode("utf-8")
        fields = header.split()
        if len(fields) != 5 or fields[4] not in ("A", "M", "D", "T"):
            raise ValueError("unsupported Git change manifest entry")
        if not path or Path(path).is_absolute() or ".." in Path(path).parts:
            raise ValueError("invalid local PR diff path")
        if fields[4] == "D":
            continue
        if fields[1] not in ("100644", "100755"):
            raise ValueError("changed symlink or submodule requires full CodeQL analysis")
        patch = git("diff", "--no-ext-diff", "--no-textconv", "--no-renames",
                    "--no-color", "--no-relative", "--unified=0", ancestor, analyzed, "--", path)
        hunks = re.findall(rb"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@", patch, re.MULTILINE)
        # Binary/mode-only/empty changes and deletion-only hunks cannot supply
        # precise surviving lines. Whole-file matching is deliberately broader.
        if not hunks or any(count == b"0" for _, count in hunks):
            ranges.append({"path": path, "startLine": 0, "endLine": 0})
        else:
            for start, count in hunks:
                first, length = int(start), int(count) if count else 1
                if first < 1 or length < 1:
                    raise ValueError("invalid Git new-line hunk")
                ranges.append({"path": path, "startLine": first, "endLine": first + length - 1})
    print("CodeQL local diff: " + json.dumps({"merge_base": ancestor, "analyzed_sha": analyzed,
          "changed_files": (len(parts) - 1) // 2, "ranges": len(ranges)}), flush=True)
    return ranges


def diff_pack(profile, deadline=None):
    # Prefer pinned init's ranges. Large PRs exceed its compare API cap, so
    # compute a complete local fallback in the actual analyzed tree instead.
    temp = os.environ.get("CODEQL_ACTION_TEMP") or os.environ.get("RUNNER_TEMP")
    if profile != "fast" or not temp:
        return None
    ranges_file = Path(temp, "pr-diff-range.json")
    if ranges_file.exists():
        ranges = json.loads(ranges_file.read_text())
    elif os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        ranges = local_diff_ranges(deadline)
    else:
        return None
    if not isinstance(ranges, list):
        raise ValueError("invalid PR diff ranges")
    data = []
    for row in ranges:
        path, start, end = row["path"], row["startLine"], row["endLine"]
        if not isinstance(path, str) or Path(path).is_absolute() or ".." in Path(path).parts:
            raise ValueError("invalid PR diff path")
        if type(start) is not int or type(end) is not int or start < 0 or end < start:
            raise ValueError("invalid PR diff lines")
        # Diff filtering affects only alert ranges; never omit product paths.
        if path.endswith("_test.go") or "testdata" in Path(path).parts:
            continue
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
    finally:
        # The evaluator owns a separate session, so even interruption of the
        # invoking shell must terminate that session before this helper exits.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait()
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
    pack = diff_pack(args.profile, deadline)
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
    def interrupted(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupted)
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
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        action_env(CODEQL_ACTION_JOB_STATUS="JOB_STATUS_FAILURE")
        print(f"::error::CodeQL: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
