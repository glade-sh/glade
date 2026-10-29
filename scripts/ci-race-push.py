#!/usr/bin/env python3
"""Run the checked concurrency scope; the scheduled workflow retains full race."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
GO = os.environ.get("CI_RACE_GO_COMMAND", "go")
VALIDATOR = ROOT / "scripts/ci-go-test.sh"


def scope():
    config = json.loads((ROOT / "scripts/ci-race-push.json").read_text())
    manifest = json.loads((ROOT / "scripts/ci-package-lanes.json").read_text())
    full = {"." + p.removeprefix("github.com/glade-sh/glade")
            for lane in manifest["lanes"].values() for p in lane}
    packages = config["packages"]
    if config["version"] != 1 or not packages or not set(packages) <= full:
        raise ValueError("push race scope must be a nonempty subset of full race")
    for package, names in packages.items():
        if names is not None and (not names or names != sorted(set(names)) or
                any(not re.fullmatch(r"Test\w+", n) for n in names)):
            raise ValueError(f"invalid focused tests for {package}")
    return config


RUN_SECONDS = 600
BUILD_SECONDS = 60
BATCH_SECONDS = 900


def positive_seconds(value):
    if type(value) is not int or value < 1:
        raise ValueError("race estimates must be positive integer seconds")
    return value


def plan(config):
    runs = []
    for package, names in sorted(config["packages"].items()):
        weights = config.get("testSeconds", {}).get(package)
        splits = config.get("splitTests", {}).get(package, {})
        if weights is None:
            seconds = positive_seconds(config["estimatedSeconds"][package])
            if seconds > BATCH_SECONDS:
                raise ValueError("oversized package needs measured test selections: " + package)
            runs.append({"package": package, "tests": names, "estimatedSeconds": seconds})
            continue
        if names is None or set(weights) != set(names) or not set(splits) <= set(names):
            raise ValueError("per-test estimates must cover the exact selected set")
        groups = {}
        for name in names:
            positive_seconds(weights[name])
            parts = splits.get(name)
            leaves = {name + "/" + path: value for path, value in parts.items()} if parts else {name: weights[name]}
            for path in leaves:
                segments = path.split("/")
                if any(not segment or re.search(r"[\s|\[\]()*+?^$\\]", segment) for segment in segments):
                    raise ValueError("invalid exact subtest path")
                if any(other.startswith(path + "/") for other in leaves):
                    raise ValueError("overlapping split selections")
                seconds = positive_seconds(leaves[path])
                if seconds > RUN_SECONDS:
                    raise ValueError("oversized test needs a finer measured split: " + path)
                groups.setdefault(tuple(segments[:-1]), []).append((path, seconds))
        # Only combine siblings. A Go -run filter is a slash-separated regex;
        # combining unrelated paths would create a cross-product of branches.
        for prefix, entries in sorted(groups.items()):
            bins = []
            for name, seconds in sorted(entries, key=lambda row: (-row[1], row[0])):
                target = next((b for b in bins if b["estimatedSeconds"] + seconds <= RUN_SECONDS + BUILD_SECONDS), None)
                if target is None:
                    target = {"package": package, "tests": [], "estimatedSeconds": BUILD_SECONDS}
                    bins.append(target)
                target["tests"].append(name)
                target["estimatedSeconds"] += seconds
            for item in bins:
                item["tests"].sort()
            runs.extend(bins)
    batches = []
    for item in sorted(runs, key=lambda r: (-r["estimatedSeconds"], r["package"], r["tests"] or [])):
        batch = next((b for b in batches if b["estimatedSeconds"] + item["estimatedSeconds"] <= BATCH_SECONDS), None)
        if batch is None:
            batch = {"index": len(batches), "packages": [], "runs": [], "estimatedSeconds": 0}
            batches.append(batch)
        batch["runs"].append(item)
        batch["packages"] = sorted(set(batch["packages"] + [item["package"]]))
        batch["estimatedSeconds"] += item["estimatedSeconds"]
    if len(batches) > 256:
        raise ValueError("race plan exceeds the CI matrix limit")
    return {"include": batches}


def run_pattern(selected):
    if not selected:
        return "^$"
    paths = [name.split("/") for name in selected]
    if any(path[:-1] != paths[0][:-1] for path in paths):
        raise ValueError("run selection must contain exact siblings")
    return "/".join(["^" + re.escape(part) + "$" for part in paths[0][:-1]] +
                    ["^(?:" + "|".join(re.escape(path[-1]) for path in paths) + ")$"])


def run(config, index):
    batches = plan(config)["include"]
    index = int(index)
    if not 0 <= index < len(batches):
        raise ValueError("invalid push race batch index")
    batch = batches[index]
    for ordinal, selection in enumerate(batch["runs"]):
        package = selection["package"]
        environment = os.environ.copy()
        if package == "./internal/apextest" and environment.get("GLADE_APEXTEST_SHARD"):
            raise ValueError("push race selections cannot be intersected with a second Apex partition")
        if package == "./internal/gladecli":
            environment["GOMAXPROCS"] = "1"
        directory = ROOT / "ci-artifacts/race-push" / package.removeprefix("./").replace("/", "-")
        if sum(r["package"] == package for r in batch["runs"]) > 1:
            directory /= "run-" + str(ordinal)
        directory.mkdir(parents=True, exist_ok=True)
        (directory / "validation-summary.json").unlink(missing_ok=True)
        discovery = subprocess.check_output([GO, "test", "-race", "-list", ".", package], cwd=ROOT, text=True, env=environment)
        (directory / "discovery-command.txt").write_text(discovery)
        discovered = sorted(line for line in discovery.splitlines() if re.fullmatch(r"Test\w+", line))
        selected = selection["tests"]
        selected = discovered if selected is None else selected
        if len(discovered) != len(set(discovered)) or not {name.split("/")[0] for name in selected} <= set(discovered):
            raise ValueError(f"focused race tests missing or duplicated in {package}")
        # Packages with no tests still compile under the race detector.
        if not selected and "[no test files]" not in discovery:
            raise ValueError(f"empty race discovery for {package}")
        regex = run_pattern(selected)
        (directory / "selection.json").write_text(json.dumps({"package": package, "tests": selected, "regex": regex}, indent=2) + "\n")
        # Keep full Test selections short without also running fuzz seeds or
        # examples, whose terminal events are outside the discovered Test set.
        run_regex = "^Test" if selected and selected == discovered else regex
        command = [GO, "test", "-race", "-json", "-vet=off", "-count=1", "-timeout=" + str(max(720, selection["estimatedSeconds"] + 60)) + "s", "-run", run_regex, package]
        terminal = {}
        started = set()
        finished = set()
        expected_package = "github.com/glade-sh/glade/" + package.removeprefix("./")
        package_results = []
        package_started = False
        print(f"[race] {package}: {len(selected)} selected tests", flush=True)
        with (directory / "events.json").open("w") as target:
            status = subprocess.run(command, cwd=ROOT, stdout=target, env=environment).returncode
        with (directory / "events.json").open() as source:
            for line in source:
                event = json.loads(line)
                if event.get("Package") != expected_package:
                    raise ValueError("race event has wrong package")
                name, action = event.get("Test"), event.get("Action")
                if package_results:
                    raise ValueError("race event after package completion")
                if action == "start" and not name:
                    if package_started:
                        raise ValueError("repeated race package start")
                    package_started = True
                elif not package_started:
                    raise ValueError("race event before package start")
                if name and action == "run":
                    if name in started:
                        raise ValueError("duplicate race start: " + name)
                    started.add(name)
                if name and action in ("pass", "skip", "fail"):
                    if name in finished:
                        raise ValueError("duplicate race terminal: " + name)
                    finished.add(name)
                if name in selected and action in ("pass", "skip", "fail"):
                    if name in terminal:
                        raise ValueError(f"duplicate terminal result: {name}")
                    terminal[name] = action
                if not name and action in ("pass", "fail", "skip"):
                    package_results.append(action)
                if action == "output":
                    print(event.get("Output", ""), end="", flush=True)
        metadata = directory / "package-metadata.tsv"
        metadata.write_text(expected_package + ("\thas-tests\n" if selected else "\tno-tests\n"))
        package_summary = directory / "package-summary.json"
        validation = subprocess.run(["bash", str(VALIDATOR), "package-summary", "race", package,
            str(directory / "events.json"), str(package_summary), str(metadata), expected_package],
            text=True, capture_output=True, env=environment)
        if validation.returncode:
            raise ValueError(f"race package validation failed: {validation.stdout}{validation.stderr}")
        deferred = {row["test"] for row in json.loads(package_summary.read_text())["deferred"]}
        expected = {name: "skip" if name in deferred else "pass" for name in selected}
        ancestors = {"/".join(name.split("/")[:i]) for name in selected for i in range(1, len(name.split("/")))}
        extra = {name for name in finished if name not in ancestors and not any(name == root or name.startswith(root + "/") for root in selected)}
        if status or terminal != expected or extra or started != finished or (selected and package_results != ["pass"]):
            raise ValueError(f"race results do not exactly pass selected tests: {package}; exit={status}")
        (directory / "validation-summary.json").write_text(json.dumps({"valid": True, "package": package,
            "discovered": discovered, "selected": selected,
            "passed": sorted(n for n, action in terminal.items() if action == "pass"),
            "deferred": sorted(deferred)}, indent=2) + "\n")


def main():
    config = scope()
    if sys.argv[1:] == ["plan"]:
        print(json.dumps(plan(config), separators=(",", ":")))
    elif len(sys.argv) == 3 and sys.argv[1] == "run":
        run(config, sys.argv[2])
    else:
        raise ValueError("usage: ci-race-push.py plan|run INDEX")


if __name__ == "__main__":
    main()
