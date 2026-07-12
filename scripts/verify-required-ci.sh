#!/usr/bin/env bash
set -euo pipefail

repository="${1:-}"
sha="${2:-}"

if [[ ! "$repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "usage: $0 OWNER/REPOSITORY 40_HEX_SHA" >&2
  exit 2
fi
if [[ ! "$sha" =~ ^[0-9A-Fa-f]{40}$ ]]; then
  echo "usage: $0 OWNER/REPOSITORY 40_HEX_SHA" >&2
  exit 2
fi

runs_json="$(gh api --method GET \
  "/repos/$repository/actions/workflows/ci.yml/runs" \
  -f "head_sha=$sha" \
  -f status=completed \
  -f per_page=100)"

while IFS=$'\t' read -r run_id event run_url; do
  [[ -n "$run_id" ]] || continue
  jobs_json="$(gh api --method GET "/repos/$repository/actions/runs/$run_id/jobs" -f per_page=100)"
  authority="$(jq -c --arg sha "$sha" '
    .jobs
    | map(select(
        .name == "Required CI"
        and .head_sha == $sha
        and .status == "completed"
        and .conclusion == "success"
      ))
    | first // empty
  ' <<<"$jobs_json")"
  [[ -n "$authority" ]] || continue

  jq -n \
    --arg repository "$repository" \
    --arg sha "$sha" \
    --arg event "$event" \
    --arg run_url "$run_url" \
    --argjson run_id "$run_id" \
    --argjson authority "$authority" \
    '{
      schema_version: 1,
      repository: $repository,
      sha: $sha,
      conclusion: "success",
      event: $event,
      run_id: $run_id,
      run_url: $run_url,
      required_ci_job_id: $authority.id,
      required_ci_job_url: $authority.html_url
    }'
  exit 0
done < <(jq -r --arg sha "$sha" '
  .workflow_runs
  | map(select(
      .head_sha == $sha
      and .conclusion == "success"
      and .event == "push"
    ))
  | sort_by(.created_at)
  | reverse[]
  | [.id, .event, .html_url]
  | @tsv
' <<<"$runs_json")

echo "no successful Required CI authority for $repository@$sha" >&2
exit 1
