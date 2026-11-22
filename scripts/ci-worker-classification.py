#!/usr/bin/env python3
"""Publish terminal Actions job conclusions without conflating cancellations and failures."""
import json
import os
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    pages = json.load(source)
if not isinstance(pages, list) or not pages:
    raise ValueError("missing Actions job pages")
classes = {key: [] for key in ("passed", "failed", "cancelled", "skipped", "other")}
for page in pages:
    if not isinstance(page, dict) or not isinstance(page.get("jobs"), list):
        raise ValueError("invalid Actions job page")
    for job in page["jobs"]:
        if not isinstance(job, dict) or not isinstance(job.get("name"), str):
            raise ValueError("invalid Actions job")
        if job.get("status") != "completed":
            continue
        conclusion = job.get("conclusion")
        category = {"success": "passed", "failure": "failed", "timed_out": "failed",
                    "startup_failure": "failed", "action_required": "failed",
                    "cancelled": "cancelled", "skipped": "skipped"}.get(conclusion, "other")
        classes[category].append(job["name"])
for names in classes.values():
    names.sort()
encoded = json.dumps(classes, ensure_ascii=True, separators=(",", ":"))
if os.environ.get("CI_CLASSIFICATION_PATH"):
    with open(os.environ["CI_CLASSIFICATION_PATH"], "w", encoding="utf-8") as artifact:
        artifact.write(encoded + "\n")
with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
    output.write("classification=" + encoded + "\n")
with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as summary:
    summary.write("## CI worker classification\n\n")
    for category, names in classes.items():
        summary.write(f"- {category}: {len(names)}\n")
        for name in names:
            summary.write("  - " + name.replace("\n", " ").replace("\r", " ") + "\n")
if classes["cancelled"]:
    message = ", ".join(classes["cancelled"])
    message = message.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")
    print("::warning::Cancelled CI workers: " + message)
