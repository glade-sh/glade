#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${1:-}" == "--tag-tools-sha" ]]; then
  [[ $# -eq 2 ]] || { echo "usage: $0 --tag-tools-sha <tag>" >&2; exit 2; }
  ref="refs/tags/$2"
  git check-ref-format "$ref" >/dev/null
  [[ "$(git for-each-ref --format='%(objecttype)' "$ref")" == "tag" ]] || {
    echo "$2 is not an annotated tag" >&2
    exit 1
  }
  mapfile -t trailers < <(git for-each-ref --format='%(trailers:only,unfold)' "$ref" | awk '/^Glade-Tools-SHA:/')
  [[ ${#trailers[@]} -eq 1 && "${trailers[0]}" =~ ^Glade-Tools-SHA:\ ([0-9a-f]{40})$ ]] || {
    echo "$2 must contain exactly one lowercase Glade-Tools-SHA trailer" >&2
    exit 1
  }
  printf '%s\n' "${BASH_REMATCH[1]}"
  exit 0
fi

[[ $# -eq 2 ]] || { echo "usage: $0 <glade-sha> <glade-tools-sha>" >&2; exit 2; }
glade_sha="$1"
tools_sha="$2"
repository="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
[[ "$glade_sha" =~ ^[0-9a-f]{40}$ ]] || { echo "glade SHA must be lowercase full 40-hex" >&2; exit 1; }
[[ "$tools_sha" =~ ^[0-9a-f]{40}$ ]] || { echo "glade-tools SHA must be lowercase full 40-hex" >&2; exit 1; }
[[ "$repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo "invalid GITHUB_REPOSITORY" >&2; exit 1; }

response="$(mktemp)"
trap 'rm -f "$response"' EXIT
gh api --method GET "repos/$repository/commits/$glade_sha/check-runs?per_page=100&filter=latest&check_name=Salesforce%20Correctness" > "$response"
python3 - "$root/.github/release-authorities.json" "$response" "$repository" "$glade_sha" "$tools_sha" <<'PY'
import json
import re
import sys


def reject_duplicates(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_json(stream):
    return json.load(stream, object_pairs_hook=reject_duplicates)


anchor_path, response_path, repository, glade_sha, tools_sha = sys.argv[1:]
with open(anchor_path, encoding="utf-8") as stream:
    anchor = load_json(stream)
if set(anchor) != {"schemaVersion", "githubAppID", "checkName"}:
    raise SystemExit("release authority anchor has unexpected schema")
if anchor["schemaVersion"] != 1 or type(anchor["githubAppID"]) is not int or anchor["githubAppID"] <= 0:
    raise SystemExit("release authority anchor has invalid identity")
if anchor["checkName"] != "Salesforce Correctness":
    raise SystemExit("release authority anchor has invalid check name")

try:
    with open(response_path, encoding="utf-8") as stream:
        payload = load_json(stream)
except (json.JSONDecodeError, ValueError) as exc:
    raise SystemExit(f"invalid check-runs JSON: {exc}")
if not isinstance(payload, dict) or type(payload.get("total_count")) is not int or not isinstance(payload.get("check_runs"), list):
    raise SystemExit("invalid check-runs response")
runs = payload["check_runs"]
if payload["total_count"] != len(runs) or len(runs) > 100:
    raise SystemExit("paginated check-runs response is not release authority")

matches = [run for run in runs if isinstance(run, dict) and run.get("name") == anchor["checkName"] and isinstance(run.get("app"), dict) and run["app"].get("id") == anchor["githubAppID"]]
if len(matches) != 1:
    raise SystemExit(f"expected exactly one trusted Salesforce authority, found {len(matches)}")
run = matches[0]
if type(run.get("id")) is not int or run["id"] <= 0:
    raise SystemExit("invalid check run ID")
if run.get("head_sha") != glade_sha or run.get("status") != "completed" or run.get("conclusion") != "success":
    raise SystemExit("Salesforce authority is not a successful exact-SHA check")
if not isinstance(run.get("html_url"), str) or not run["html_url"].startswith("https://github.com/"):
    raise SystemExit("invalid check run URL")
external = run.get("external_id")
if not isinstance(external, str):
    raise SystemExit("missing Salesforce authority external_id")
match = re.fullmatch(r"salesforce-release-authority/v1;tools_sha=([0-9a-f]{40});run_id=([1-9][0-9]*);run_attempt=([1-9][0-9]*);receipt_sha256=([0-9a-f]{64})", external)
if not match or match.group(1) != tools_sha:
    raise SystemExit("invalid Salesforce authority external_id")

evidence = {
    "schemaVersion": 1,
    "repository": repository,
    "gladeSHA": glade_sha,
    "toolsSHA": tools_sha,
    "githubAppID": anchor["githubAppID"],
    "checkName": anchor["checkName"],
    "checkRunID": run["id"],
    "checkRunURL": run["html_url"],
    "workflowRunID": int(match.group(2)),
    "workflowRunAttempt": int(match.group(3)),
    "receiptSHA256": match.group(4),
}
json.dump(evidence, sys.stdout, sort_keys=True, separators=(",", ":"))
sys.stdout.write("\n")
PY
