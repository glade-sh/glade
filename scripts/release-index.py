#!/usr/bin/env python3
import argparse
import json
import re
import sys
import urllib.request


USER_AGENT = "glade-release-workflow/1.0"


def main():
    parser = argparse.ArgumentParser(description="Build the Glade download index.")
    parser.add_argument("--version", required=True)
    parser.add_argument("--download-base", default="https://downloads.glade.sh")
    parser.add_argument("--existing-index-url", default="https://downloads.glade.sh/index.json")
    parser.add_argument("--output", default="index.json")
    args = parser.parse_args()

    download_base = args.download_base.rstrip("/")
    current_version_row = {
        "version": args.version,
        "manifest": f"{download_base}/{args.version}/release-manifest.json",
    }
    versions_by_version = read_existing_versions(args.existing_index_url)
    versions_by_version[args.version] = current_version_row

    index = {
        "schemaVersion": 1,
        "channel": "stable",
        "latest": args.version,
        "versions": sorted(versions_by_version.values(), key=version_key, reverse=True),
    }
    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(index, f, indent=2, sort_keys=True)
        f.write("\n")


def read_existing_versions(index_url):
    versions_by_version = {}
    try:
        request = urllib.request.Request(index_url, headers={"User-Agent": USER_AGENT})
        with urllib.request.urlopen(request, timeout=10) as response:
            existing_index = json.load(response)
        for row in existing_index.get("versions", []):
            row_version = row.get("version")
            row_manifest = row.get("manifest")
            if row_version and row_manifest:
                versions_by_version[row_version] = {
                    "version": row_version,
                    "manifest": row_manifest,
                }
    except Exception as exc:
        print(f"warning: could not read existing download index: {exc}", file=sys.stderr)
    return versions_by_version


def version_key(row):
    parts = []
    for part in re.split(r"[.-]", row["version"].lstrip("v")):
        if part.isdigit():
            parts.append((1, int(part)))
        else:
            parts.append((0, part))
    return tuple(parts)


if __name__ == "__main__":
    main()
