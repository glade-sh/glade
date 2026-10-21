#!/usr/bin/env python3
"""Summarize read-only GitHub Actions jobs responses without logs or source IDs.

Input directory: ci.json/race.json/browser.json/security.json from
  gh run list -R OWNER/REPO -w NAME --limit 20 --json databaseId,...
and ID.json from gh api repos/OWNER/REPO/actions/runs/ID/jobs?per_page=100.
Step times include compilation inside test commands; the API cannot split that.
"""
import csv
from datetime import datetime
import json
from pathlib import Path
import statistics
import sys


def seconds(item):
    start, end = item.get("started_at"), item.get("completed_at")
    return max(0, (datetime.fromisoformat(end.replace("Z", "+00:00")) -
                   datetime.fromisoformat(start.replace("Z", "+00:00"))).total_seconds()) if start and end else 0


def bucket(name):
    name = name.lower()
    if "checkout" in name:
        return "checkout"
    if "setup-go" in name or "set up go" in name:
        return "go"
    if "setup-node" in name or "set up node" in name:
        return "node"
    if "cache" in name and ("restore" in name or "download" in name):
        return "cache_restore"
    if "install" in name or "npm ci" in name:
        return "dependencies"
    if "build" in name and ("parser" in name or "binar" in name):
        return "build"
    if any(word in name for word in ("test", "race detector", "race shard", "oracle", "gosec", "analyze codeql", "govulncheck", "audit", "run ", "check ", "verify ", "smoke")):
        return "validation"
    return "other"


def main():
    root, destination = map(Path, sys.argv[1:])
    groups = {}
    columns = ["wall", "checkout", "go", "node", "cache_restore", "dependencies", "build", "validation", "other"]
    for workflow in ("ci", "race", "browser", "security"):
        for run in json.loads((root / f"{workflow}.json").read_text()):
            response = json.loads((root / f"{run['databaseId']}.json").read_text())
            if response["total_count"] > len(response["jobs"]):
                raise ValueError("incomplete job pagination")
            for job in response["jobs"]:
                if job["conclusion"] in ("skipped", "cancelled") or not job["completed_at"]:
                    continue
                values = dict.fromkeys(columns, 0)
                values["wall"] = seconds(job)
                for step in job["steps"]:
                    values[bucket(step["name"])] += seconds(step)
                values["failed"] = int(job["conclusion"] != "success")
                values["timeout"] = int(job["conclusion"] == "timed_out")
                groups.setdefault((workflow, job["name"]), []).append(values)
    with destination.open("w") as target:
        writer = csv.writer(target, delimiter="\t", lineterminator="\n")
        writer.writerow(["workflow", "job", "samples", *[c + "_p50_s" for c in columns], "failures", "timeouts"])
        for (workflow, job), samples in sorted(groups.items()):
            writer.writerow([workflow, job, len(samples), *[round(statistics.median(s[c] for s in samples), 1) for c in columns], sum(s["failed"] for s in samples), sum(s["timeout"] for s in samples)])


if __name__ == "__main__":
    main()
