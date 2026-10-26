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


def plan(config):
    # Longest first. Estimates include a conservative package build allowance;
    # packages in a batch share their build cache and run one at a time.
    weights = config["estimatedSeconds"]
    batches = [{"index": i, "packages": [], "estimatedSeconds": 0} for i in range(8)]
    for package in sorted(config["packages"], key=lambda p: (-weights[p], p)):
        batch = min(batches, key=lambda b: (b["estimatedSeconds"], b["index"]))
        batch["packages"].append(package)
        batch["estimatedSeconds"] += weights[package]
    return {"include": batches}


def run(config, index):
    batches = plan(config)["include"]
    index = int(index)
    if not 0 <= index < len(batches):
        raise ValueError("invalid push race batch index")
    batch = batches[index]
    for package in batch["packages"]:
        environment = os.environ.copy()
        if package == "./internal/gladecli":
            environment["GOMAXPROCS"] = "1"
        directory = ROOT / "ci-artifacts/race-push" / package.removeprefix("./").replace("/", "-")
        directory.mkdir(parents=True, exist_ok=True)
        (directory / "validation-summary.json").unlink(missing_ok=True)
        discovery = subprocess.check_output([GO, "test", "-race", "-list", ".", package], cwd=ROOT, text=True, env=environment)
        (directory / "discovery-command.txt").write_text(discovery)
        discovered = sorted(line for line in discovery.splitlines() if re.fullmatch(r"Test\w+", line))
        selected = config["packages"][package]
        selected = discovered if selected is None else selected
        if len(discovered) != len(set(discovered)) or not set(selected) <= set(discovered):
            raise ValueError(f"focused race tests missing or duplicated in {package}")
        # Packages with no tests still compile under the race detector.
        if not selected and "[no test files]" not in discovery:
            raise ValueError(f"empty race discovery for {package}")
        regex = "^(?:" + "|".join(re.escape(n) for n in selected) + ")$" if selected else "^$"
        (directory / "selection.json").write_text(json.dumps({"package": package, "tests": selected, "regex": regex}, indent=2) + "\n")
        # Keep full Test selections short without also running fuzz seeds or
        # examples, whose terminal events are outside the discovered Test set.
        run_regex = "^Test" if selected and selected == discovered else regex
        command = [GO, "test", "-race", "-json", "-vet=off", "-count=1", "-timeout=12m", "-run", run_regex, package]
        terminal = {}
        expected_package = "github.com/glade-sh/glade/" + package.removeprefix("./")
        package_results = []
        print(f"[race] {package}: {len(selected)} selected tests", flush=True)
        with (directory / "events.json").open("w") as target:
            status = subprocess.run(command, cwd=ROOT, stdout=target, env=environment).returncode
        with (directory / "events.json").open() as source:
            for line in source:
                event = json.loads(line)
                if event.get("Package") != expected_package:
                    raise ValueError("race event has wrong package")
                name, action = event.get("Test"), event.get("Action")
                if name and "/" not in name and action in ("pass", "skip", "fail"):
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
        if status or terminal != expected or (selected and package_results != ["pass"]):
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
